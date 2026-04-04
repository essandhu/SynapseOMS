package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/crossing"
	"github.com/synapse-oms/gateway/internal/domain"
	riskgrpc "github.com/synapse-oms/gateway/internal/grpc"
	kafkapkg "github.com/synapse-oms/gateway/internal/kafka"
	"github.com/synapse-oms/gateway/internal/logging"
	"github.com/synapse-oms/gateway/internal/metrics"
	"github.com/synapse-oms/gateway/internal/router"
)

const intakeChanSize = 10_000

// riskPoolSize is the number of concurrent goroutines for risk checks.
const riskPoolSize = 32

// Store is the interface the pipeline uses for persistence.
// It is satisfied by store.PostgresStore but can be mocked for tests.
type Store interface {
	CreateOrder(ctx context.Context, order *domain.Order) error
	UpdateOrder(ctx context.Context, order *domain.Order) error
	GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, error)
	CreateFill(ctx context.Context, fill *domain.Fill) error
	UpsertPosition(ctx context.Context, pos *domain.Position) error
	GetPosition(ctx context.Context, instrumentID, venueID string) (*domain.Position, error)
}

// KafkaPublisher is the interface for publishing order lifecycle events to Kafka.
// It is satisfied by kafka.Producer but can be mocked for tests.
type KafkaPublisher interface {
	PublishOrderLifecycle(ctx context.Context, instrumentID string, payload []byte) error
}

// venueOrder couples an order with its dispatch context, carrying the order
// from the router stage to the venue dispatch stage.
type venueOrder struct {
	order *domain.Order
}

// Pipeline orchestrates the order lifecycle:
// Submit -> intake chan -> Risk Check -> Router -> Venue Dispatch (per-adapter) -> Fill Collector -> Notifier (WS + Kafka)
type Pipeline struct {
	store      Store
	venues     []adapter.LiquidityProvider
	venueMap   map[string]adapter.LiquidityProvider // venueID -> provider
	notifier   Notifier
	riskClient riskgrpc.RiskClient
	kafka      KafkaPublisher // nil if Kafka is not configured
	logger     *slog.Logger

	// Smart routing: optional router and crossing engine (Phase 3).
	// When nil, the pipeline falls back to legacy asset-class-based routing.
	orderRouter    *router.Router
	crossingEngine *crossing.CrossingEngine

	// failOpenRisk controls whether risk engine unavailability should fail-open.
	// When true (default for paper trading), orders pass if the risk engine is unreachable.
	failOpenRisk bool

	intake     chan *domain.Order       // from Submit to risk check
	riskOut    chan *domain.Order       // from risk check to router
	dispatchCh map[string]chan venueOrder // per-venue dispatch channels

	// orderMap tracks in-flight orders by ID so the fill collector
	// can look them up without a store round-trip.
	orderMu  sync.RWMutex
	orderMap map[domain.OrderID]*domain.Order

	wg sync.WaitGroup
}

// Option configures a Pipeline.
type Option func(*Pipeline)

// WithKafkaPublisher sets the Kafka publisher for order lifecycle events.
func WithKafkaPublisher(kp KafkaPublisher) Option {
	return func(p *Pipeline) {
		p.kafka = kp
	}
}

// WithFailOpenRisk sets whether the risk check should fail-open (approve)
// when the risk engine is unavailable. Default is true.
func WithFailOpenRisk(failOpen bool) Option {
	return func(p *Pipeline) {
		p.failOpenRisk = failOpen
	}
}

// WithRouter injects a smart order router into the pipeline.
// When set, the router() goroutine delegates to this router instead of
// the legacy asset-class-based routeOrder() method.
func WithRouter(r *router.Router) Option {
	return func(p *Pipeline) {
		p.orderRouter = r
	}
}

// WithCrossingEngine injects an internal crossing engine into the pipeline.
// When set, the router() goroutine attempts internal crossing before
// dispatching to external venues.
func WithCrossingEngine(e *crossing.CrossingEngine) Option {
	return func(p *Pipeline) {
		p.crossingEngine = e
	}
}

// NewPipeline creates a new order processing pipeline.
// venues is the list of liquidity providers to dispatch orders to.
// riskClient may be nil (in which case a fail-open client is used).
func NewPipeline(store Store, venues []adapter.LiquidityProvider, notifier Notifier, riskClient riskgrpc.RiskClient, opts ...Option) *Pipeline {
	venueMap := make(map[string]adapter.LiquidityProvider, len(venues))
	dispatchCh := make(map[string]chan venueOrder, len(venues))
	for _, v := range venues {
		venueMap[v.VenueID()] = v
		dispatchCh[v.VenueID()] = make(chan venueOrder, intakeChanSize)
	}

	if riskClient == nil {
		riskClient = riskgrpc.NewFailOpenRiskClient()
	}

	p := &Pipeline{
		store:        store,
		venues:       venues,
		venueMap:     venueMap,
		notifier:     notifier,
		riskClient:   riskClient,
		logger:       logging.NewDefault("gateway", "pipeline"),
		failOpenRisk: true, // default: fail-open
		intake:       make(chan *domain.Order, intakeChanSize),
		riskOut:      make(chan *domain.Order, intakeChanSize),
		dispatchCh:   dispatchCh,
		orderMap:     make(map[domain.OrderID]*domain.Order),
	}

	for _, opt := range opts {
		opt(p)
	}

	return p
}

// Start launches the pipeline goroutines. All goroutines respect ctx for
// cancellation and will drain within 5 seconds.
func (p *Pipeline) Start(ctx context.Context) {
	// Risk check pool: riskPoolSize goroutines reading from intake, writing to riskOut
	p.wg.Add(riskPoolSize)
	for i := 0; i < riskPoolSize; i++ {
		go p.riskCheckWorker(ctx)
	}

	// Router: reads from riskOut, dispatches to per-venue channels
	p.wg.Add(1)
	go p.router(ctx)

	// One venue dispatch goroutine per registered adapter
	for _, v := range p.venues {
		p.wg.Add(1)
		go p.venueDispatch(ctx, v)
	}

	// Fill collector: one goroutine per venue, reading from each venue's fill feed
	for _, v := range p.venues {
		p.wg.Add(1)
		go p.fillCollector(ctx, v)
	}
}

// Wait blocks until all pipeline goroutines have exited.
func (p *Pipeline) Wait() {
	p.wg.Wait()
}

// Submit validates the order, assigns a UUID, persists it to the database,
// and pushes it to the intake channel. The order is persisted synchronously
// so that it is available via GET /orders immediately after the REST response.
func (p *Pipeline) Submit(ctx context.Context, order *domain.Order) error {
	if order.Quantity.IsZero() || order.Quantity.IsNegative() {
		return fmt.Errorf("order quantity must be positive")
	}
	if order.InstrumentID == "" {
		return fmt.Errorf("instrument ID is required")
	}

	order.ID = domain.OrderID(newUUID())
	order.Status = domain.OrderStatusNew
	now := time.Now()
	order.CreatedAt = now
	order.UpdatedAt = now

	// Persist the order to the database so it survives page refreshes.
	if err := p.store.CreateOrder(ctx, order); err != nil {
		return fmt.Errorf("persisting new order: %w", err)
	}

	// Track in memory for fill collector lookups
	p.orderMu.Lock()
	p.orderMap[order.ID] = order
	p.orderMu.Unlock()

	select {
	case p.intake <- order:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// riskCheckWorker is one of riskPoolSize goroutines that perform concurrent
// pre-trade risk checks. Approved orders are forwarded to the router via riskOut.
func (p *Pipeline) riskCheckWorker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-p.intake:
			if !ok {
				return
			}

			result, err := p.riskClient.CheckPreTradeRisk(ctx, order)
			if err != nil {
				if p.failOpenRisk {
					p.logger.Warn("risk engine error, failing open",
						slog.String("order_id", string(order.ID)),
						slog.String("error", err.Error()),
					)
					// Fall through: treat as approved
				} else {
					p.logger.Error("risk engine error, rejecting order",
						slog.String("order_id", string(order.ID)),
						slog.String("error", err.Error()),
					)
					_ = order.ApplyTransition(domain.OrderStatusRejected)
					_ = p.store.UpdateOrder(ctx, order)
					p.notifyOrderUpdate(ctx, order)
					continue
				}
			} else if !result.Approved {
				p.logger.Warn("order rejected by risk engine",
					slog.String("order_id", string(order.ID)),
					slog.String("reason", result.RejectReason),
				)
				_ = order.ApplyTransition(domain.OrderStatusRejected)
				_ = p.store.UpdateOrder(ctx, order)
				p.notifyOrderUpdate(ctx, order)
				continue
			}

			select {
			case p.riskOut <- order:
			case <-ctx.Done():
				return
			}
		}
	}
}

// routeOrder determines the target venue ID for an order based on asset class.
// Phase 2 routing: equity -> alpaca, crypto -> binance_testnet, default -> simulated.
func (p *Pipeline) routeOrder(order *domain.Order) string {
	switch order.AssetClass {
	case domain.AssetClassEquity:
		if _, ok := p.venueMap["alpaca"]; ok {
			return "alpaca"
		}
	case domain.AssetClassCrypto:
		if _, ok := p.venueMap["binance_testnet"]; ok {
			return "binance_testnet"
		}
	}

	// Default: use simulated if available, otherwise first venue
	if _, ok := p.venueMap["sim-exchange"]; ok {
		return "sim-exchange"
	}

	// Fallback to any available venue
	for vid := range p.venueMap {
		return vid
	}
	return ""
}

// router reads orders from the riskOut channel, determines the target venue(s),
// persists the order as New, and forwards to the per-venue dispatch channel(s).
//
// Phase 3 flow (when orderRouter and/or crossingEngine are injected):
//  1. Try internal crossing first (crossingEngine.TryCross)
//  2. If fully crossed: process fills directly, skip venue dispatch
//  3. If partially crossed or not crossed: pass residual to router.Route()
//  4. For each allocation: dispatch to the appropriate per-venue channel
//
// Falls back to legacy routeOrder() when no smart router is configured.
func (p *Pipeline) router(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-p.riskOut:
			if !ok {
				return
			}

			// If no smart router is configured, use legacy routing.
			if p.orderRouter == nil {
				p.routeLegacy(ctx, order)
				continue
			}

			p.routeSmart(ctx, order)
		}
	}
}

// routeLegacy dispatches an order using the Phase 2 asset-class-based routing.
func (p *Pipeline) routeLegacy(ctx context.Context, order *domain.Order) {
	venueID := p.routeOrder(order)
	if venueID == "" {
		p.logger.Error("no venue available for order",
			slog.String("order_id", string(order.ID)),
		)
		_ = order.ApplyTransition(domain.OrderStatusRejected)
		_ = p.store.UpdateOrder(ctx, order)
		p.notifyOrderUpdate(ctx, order)
		return
	}

	order.VenueID = venueID

	// Order was already persisted in Submit(); update with the assigned venue.
	if err := p.store.UpdateOrder(ctx, order); err != nil {
		p.logger.Error("failed to update order with venue",
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		return
	}

	p.logger.Info("order routed (legacy)",
		slog.String("order_id", string(order.ID)),
		slog.String("venue", order.VenueID),
	)

	p.dispatchToVenue(ctx, order, venueID)
}

// routeSmart implements the Phase 3 smart routing flow:
// crossing -> router.Route() -> multi-venue dispatch.
func (p *Pipeline) routeSmart(ctx context.Context, order *domain.Order) {
	orderToRoute := order

	// Step 1: Try internal crossing if engine is available.
	if p.crossingEngine != nil {
		result, err := p.crossingEngine.TryCross(order)
		if err != nil {
			p.logger.Warn("crossing engine error, proceeding to external routing",
				slog.String("order_id", string(order.ID)),
				slog.String("error", err.Error()),
			)
		} else if result.Crossed {
			// TryCross mutates order.FilledQuantity in-place. We need to
			// reset it so that ApplyFill (called from processFill) can
			// correctly compute remaining quantity and update VWAP.
			order.FilledQuantity = decimal.Zero

			// Process crossing fills.
			// Order was already persisted in Submit(); update with venue assignment.
			order.VenueID = "INTERNAL"
			if err := p.store.UpdateOrder(ctx, order); err != nil {
				p.logger.Error("failed to update order for crossing",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				return
			}

			// Transition to Acknowledged so fills can be applied.
			if err := order.ApplyTransition(domain.OrderStatusAcknowledged); err != nil {
				p.logger.Error("failed to transition order to acknowledged for crossing",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				return
			}
			_ = p.store.UpdateOrder(ctx, order)

			for _, fill := range result.Fills {
				if fill.OrderID == order.ID {
					p.processFill(ctx, fill)
				}
			}

			p.logger.Info("order crossed internally",
				slog.String("order_id", string(order.ID)),
				slog.Int("fill_count", len(result.Fills)),
			)

			// If fully crossed (no residual), we are done.
			if result.ResidualOrder == nil {
				return
			}

			// Partially crossed: route the residual externally.
			orderToRoute = result.ResidualOrder
		}
	}

	// Step 2: Build venue candidates from registered venues.
	candidates := p.buildVenueCandidates(orderToRoute)
	if len(candidates) == 0 {
		p.logger.Error("no venue candidates for order",
			slog.String("order_id", string(orderToRoute.ID)),
		)
		_ = order.ApplyTransition(domain.OrderStatusRejected)
		_ = p.store.UpdateOrder(ctx, order)
		p.notifyOrderUpdate(ctx, order)
		return
	}

	// Step 3: Determine strategy name.
	strategyName := ""
	if order.VenueID != "" && order.VenueID != "smart" && order.VenueID != "INTERNAL" {
		strategyName = "venue-preference"
	}

	// Step 4: Route via the smart router.
	decision, err := p.orderRouter.Route(ctx, orderToRoute, candidates, strategyName)
	if err != nil {
		p.logger.Error("smart router failed, falling back to legacy",
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		p.routeLegacy(ctx, order)
		return
	}

	// Step 5: Update the parent order with venue assignment (already persisted in Submit).
	if orderToRoute == order {
		if len(decision.Allocations) == 1 {
			order.VenueID = decision.Allocations[0].VenueID
		}
		if err := p.store.UpdateOrder(ctx, order); err != nil {
			p.logger.Error("failed to update order with venue",
				slog.String("order_id", string(order.ID)),
				slog.String("error", err.Error()),
			)
			return
		}
	}

	// Step 6: Dispatch to venue(s).
	if len(decision.Allocations) == 1 {
		// Single venue: dispatch the order directly.
		alloc := decision.Allocations[0]
		order.VenueID = alloc.VenueID
		_ = p.store.UpdateOrder(ctx, order)

		p.logger.Info("order routed (smart)",
			slog.String("order_id", string(order.ID)),
			slog.String("venue", alloc.VenueID),
			slog.String("strategy", decision.Strategy),
		)

		p.dispatchToVenue(ctx, order, alloc.VenueID)
	} else {
		// Multiple venues: create child orders for each allocation.
		p.logger.Info("order split across venues",
			slog.String("order_id", string(order.ID)),
			slog.Int("venue_count", len(decision.Allocations)),
			slog.String("strategy", decision.Strategy),
		)

		for _, alloc := range decision.Allocations {
			child := p.createChildOrder(order, alloc)

			// Track child in memory.
			p.orderMu.Lock()
			p.orderMap[child.ID] = child
			p.orderMu.Unlock()

			if err := p.store.CreateOrder(ctx, child); err != nil {
				p.logger.Error("failed to persist child order",
					slog.String("order_id", string(child.ID)),
					slog.String("parent_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				continue
			}

			p.logger.Info("child order dispatched",
				slog.String("child_id", string(child.ID)),
				slog.String("parent_id", string(order.ID)),
				slog.String("venue", alloc.VenueID),
				slog.String("quantity", alloc.Quantity.String()),
			)

			p.dispatchToVenue(ctx, child, alloc.VenueID)
		}
	}
}

// dispatchToVenue sends an order to the per-venue dispatch channel.
// It first checks whether the target venue is connected. If the venue is
// disconnected, the order is rejected immediately without affecting other venues.
func (p *Pipeline) dispatchToVenue(ctx context.Context, order *domain.Order, venueID string) {
	// Venue disconnect isolation: check adapter status before dispatching.
	if err := adapter.CheckVenueReady(venueID); err != nil {
		p.logger.Warn("venue disconnected, rejecting order",
			slog.String("order_id", string(order.ID)),
			slog.String("venue", venueID),
			slog.String("error", err.Error()),
		)
		_ = order.ApplyTransition(domain.OrderStatusRejected)
		_ = p.store.UpdateOrder(ctx, order)
		p.notifyOrderUpdate(ctx, order)
		return
	}

	ch, ok := p.dispatchCh[venueID]
	if !ok {
		p.logger.Error("no dispatch channel for venue",
			slog.String("order_id", string(order.ID)),
			slog.String("venue", venueID),
		)
		return
	}

	select {
	case ch <- venueOrder{order: order}:
	case <-ctx.Done():
	}
}

// buildVenueCandidates creates stub VenueCandidates from registered venue adapters.
// In the full P3-26 wiring, these will be populated with real market data from
// the price comparator. For now, we use basic metadata with generous defaults.
func (p *Pipeline) buildVenueCandidates(order *domain.Order) []router.VenueCandidate {
	candidates := make([]router.VenueCandidate, 0, len(p.venues))
	for _, v := range p.venues {
		// Use the order price as a placeholder for bid/ask (no real market data yet).
		price := order.Price
		if price.IsZero() {
			price = decimal.NewFromInt(1) // avoid zero-price issues for market orders
		}
		candidates = append(candidates, router.VenueCandidate{
			VenueID:      v.VenueID(),
			BidPrice:     price,
			AskPrice:     price,
			DepthAtPrice: order.Quantity, // assume full depth available
			LatencyP50:   50 * time.Millisecond,
			FillRate30d:  0.95,
			FeeRate:      decimal.NewFromFloat(0.001),
		})
	}
	return candidates
}

// createChildOrder creates a child order from a parent order and a venue allocation.
func (p *Pipeline) createChildOrder(parent *domain.Order, alloc router.VenueAllocation) *domain.Order {
	child := &domain.Order{
		ID:            domain.OrderID(newUUID()),
		ClientOrderID: fmt.Sprintf("%s-child-%s", parent.ClientOrderID, alloc.VenueID),
		InstrumentID:  parent.InstrumentID,
		Side:          parent.Side,
		Type:          parent.Type,
		Quantity:      alloc.Quantity,
		Price:         parent.Price,
		Status:        domain.OrderStatusNew,
		VenueID:       alloc.VenueID,
		AssetClass:    parent.AssetClass,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	return child
}

// venueDispatch transitions the order to Acknowledged, then calls
// venue.SubmitOrder. The transition MUST happen before SubmitOrder because
// venues (especially the simulated exchange) may generate fills synchronously
// during SubmitOrder, and the fillCollector will reject fills for orders that
// are not yet in Acknowledged status.
func (p *Pipeline) venueDispatch(ctx context.Context, venue adapter.LiquidityProvider) {
	defer p.wg.Done()
	ch := p.dispatchCh[venue.VenueID()]
	for {
		select {
		case <-ctx.Done():
			return
		case vo, ok := <-ch:
			if !ok {
				return
			}
			order := vo.order

			// Transition to Acknowledged BEFORE submitting to venue so that
			// any fills produced synchronously by the venue can be applied.
			if err := order.ApplyTransition(domain.OrderStatusAcknowledged); err != nil {
				p.logger.Error("failed to transition order to acknowledged",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				continue
			}

			if err := p.store.UpdateOrder(ctx, order); err != nil {
				p.logger.Error("failed to persist acknowledged order",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				continue
			}

			p.notifyOrderUpdate(ctx, order)

			submitStart := time.Now()
			_, err := venue.SubmitOrder(ctx, order)
			if err != nil {
				p.logger.Error("venue rejected order",
					slog.String("order_id", string(order.ID)),
					slog.String("venue", venue.VenueID()),
					slog.String("error", err.Error()),
				)
				_ = order.ApplyTransition(domain.OrderStatusRejected)
				_ = p.store.UpdateOrder(ctx, order)
				p.notifyOrderUpdate(ctx, order)
				continue
			}

			venueLatency := time.Since(submitStart)
			metrics.OrdersSubmittedTotal.WithLabelValues(assetClassLabel(order.AssetClass), venue.VenueID()).Inc()
			metrics.VenueLatencySeconds.WithLabelValues(venue.VenueID()).Observe(venueLatency.Seconds())

			p.logger.Info("order acknowledged",
				slog.String("order_id", string(order.ID)),
				slog.String("venue", venue.VenueID()),
			)
		}
	}
}

// fillCollector reads fills from a venue's fill feed, applies them to orders,
// updates positions, persists everything, and notifies.
func (p *Pipeline) fillCollector(ctx context.Context, venue adapter.LiquidityProvider) {
	defer p.wg.Done()
	fillCh := venue.FillFeed()
	for {
		select {
		case <-ctx.Done():
			return
		case fill, ok := <-fillCh:
			if !ok {
				return
			}
			p.processFill(ctx, fill)
		}
	}
}

func (p *Pipeline) processFill(ctx context.Context, fill domain.Fill) {
	// Look up the order in our in-memory map
	p.orderMu.RLock()
	order, exists := p.orderMap[fill.OrderID]
	p.orderMu.RUnlock()

	if !exists {
		p.logger.Error("fill for unknown order",
			slog.String("fill_id", fill.ID),
			slog.String("order_id", string(fill.OrderID)),
		)
		return
	}

	// Apply fill to order (updates filled quantity, average price, status)
	if err := order.ApplyFill(fill); err != nil {
		p.logger.Error("failed to apply fill to order",
			slog.String("fill_id", fill.ID),
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		return
	}

	// Persist the fill
	if err := p.store.CreateFill(ctx, &fill); err != nil {
		p.logger.Error("failed to persist fill",
			slog.String("fill_id", fill.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	metrics.FillsReceivedTotal.WithLabelValues(fill.VenueID, "venue").Inc()

	// Update the order in the store
	if err := p.store.UpdateOrder(ctx, order); err != nil {
		p.logger.Error("failed to persist order after fill",
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		return
	}

	// Update position
	pos, err := p.store.GetPosition(ctx, order.InstrumentID, order.VenueID)
	if err != nil {
		// Position doesn't exist yet -- create a new one.
		// Set initial market price from the fill so the position has a
		// non-zero price before the market data feed provides updates.
		pos = &domain.Position{
			InstrumentID: order.InstrumentID,
			VenueID:      order.VenueID,
			AssetClass:   order.AssetClass,
			MarketPrice:  fill.Price,
		}
	}

	if err := pos.ApplyFill(fill, order.Side); err != nil {
		p.logger.Error("failed to apply fill to position",
			slog.String("fill_id", fill.ID),
			slog.String("error", err.Error()),
		)
		return
	}

	// Recompute unrealized P&L using current market price.
	// For new positions the market price was seeded from the fill price above;
	// for existing positions it reflects the latest market data.
	pos.UpdateMarketPrice(pos.MarketPrice)

	if err := p.store.UpsertPosition(ctx, pos); err != nil {
		p.logger.Error("failed to persist position",
			slog.String("instrument", order.InstrumentID),
			slog.String("error", err.Error()),
		)
		return
	}

	p.logger.Info("fill processed",
		slog.String("order_id", string(order.ID)),
		slog.String("fill_id", fill.ID),
		slog.Int("order_status", int(order.Status)),
		slog.String("position_qty", pos.Quantity.String()),
	)

	p.notifyFillEvent(ctx, order, fill)
	p.notifier.NotifyPositionUpdate(pos)

	// Clean up fully filled orders from the in-memory map
	if order.Status == domain.OrderStatusFilled {
		p.orderMu.Lock()
		delete(p.orderMap, order.ID)
		p.orderMu.Unlock()
	}
}

// notifyOrderUpdate sends an order update to both the WebSocket notifier and
// (optionally) the Kafka producer. For status-only events (not fills), it
// publishes a structured OrderLifecycleEvent envelope to Kafka.
func (p *Pipeline) notifyOrderUpdate(ctx context.Context, order *domain.Order) {
	p.notifier.NotifyOrderUpdate(order)

	if p.kafka != nil {
		eventType := orderStatusToEventType(order.Status)
		if eventType == "" {
			return
		}
		p.publishOrderEvent(ctx, eventType, order, nil)
	}
}

// notifyFillEvent sends an order update and publishes a fill_received event
// to Kafka with the fill details needed by the risk engine.
func (p *Pipeline) notifyFillEvent(ctx context.Context, order *domain.Order, fill domain.Fill) {
	p.notifier.NotifyOrderUpdate(order)

	if p.kafka != nil {
		p.publishOrderEvent(ctx, kafkapkg.EventFillReceived, order, &fill)

		// If the order is fully filled, also publish a terminal event
		if order.Status == domain.OrderStatusFilled {
			p.publishOrderEvent(ctx, kafkapkg.EventOrderFilled, order, nil)
		}
	}
}

// publishOrderEvent marshals and publishes a structured OrderLifecycleEvent.
func (p *Pipeline) publishOrderEvent(ctx context.Context, eventType string, order *domain.Order, fill *domain.Fill) {
	event := kafkapkg.OrderLifecycleEvent{
		Type:    eventType,
		OrderID: string(order.ID),
	}

	if fill != nil {
		event.Fill = &kafkapkg.FillPayload{
			InstrumentID:    order.InstrumentID,
			VenueID:         fill.VenueID,
			Side:            order.Side.String(),
			Quantity:        fill.Quantity.String(),
			Price:           fill.Price.String(),
			Fee:             fill.Fee.String(),
			AssetClass:      order.AssetClass.String(),
			SettlementCycle: kafkapkg.SettlementCycleForAssetClass(order.AssetClass.String()),
			Timestamp:       kafkapkg.FormatTimestamp(fill.Timestamp),
		}
	}

	payload, err := json.Marshal(event)
	if err != nil {
		p.logger.Error("failed to marshal order lifecycle event",
			slog.String("order_id", string(order.ID)),
			slog.String("error", err.Error()),
		)
		return
	}

	if err := p.kafka.PublishOrderLifecycle(ctx, order.InstrumentID, payload); err != nil {
		p.logger.Error("failed to publish order lifecycle event",
			slog.String("order_id", string(order.ID)),
			slog.String("type", eventType),
			slog.String("error", err.Error()),
		)
	}
}

// orderStatusToEventType maps an order status to the corresponding Kafka event type.
func orderStatusToEventType(status domain.OrderStatus) string {
	switch status {
	case domain.OrderStatusNew:
		return kafkapkg.EventOrderCreated
	case domain.OrderStatusAcknowledged:
		return kafkapkg.EventOrderAcknowledged
	case domain.OrderStatusFilled:
		return kafkapkg.EventOrderFilled
	case domain.OrderStatusCanceled:
		return kafkapkg.EventOrderCanceled
	case domain.OrderStatusRejected:
		return kafkapkg.EventOrderRejected
	default:
		return ""
	}
}

// assetClassLabel returns a Prometheus-friendly label for an asset class.
func assetClassLabel(ac domain.AssetClass) string {
	switch ac {
	case domain.AssetClassEquity:
		return "equity"
	case domain.AssetClassCrypto:
		return "crypto"
	default:
		return "other"
	}
}

// newUUID generates a UUID v4 using crypto/rand.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
