package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	riskgrpc "github.com/synapse-oms/gateway/internal/grpc"
	"github.com/synapse-oms/gateway/internal/logging"
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

// Submit validates the order, assigns a UUID, and pushes it to the intake channel.
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

// router reads orders from the riskOut channel, determines the target venue,
// persists the order as New, and forwards to the per-venue dispatch channel.
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

			venueID := p.routeOrder(order)
			if venueID == "" {
				p.logger.Error("no venue available for order",
					slog.String("order_id", string(order.ID)),
				)
				_ = order.ApplyTransition(domain.OrderStatusRejected)
				_ = p.store.UpdateOrder(ctx, order)
				p.notifyOrderUpdate(ctx, order)
				continue
			}

			order.VenueID = venueID

			if err := p.store.CreateOrder(ctx, order); err != nil {
				p.logger.Error("failed to persist new order",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				continue
			}

			p.logger.Info("order routed",
				slog.String("order_id", string(order.ID)),
				slog.String("venue", order.VenueID),
			)

			ch, ok := p.dispatchCh[venueID]
			if !ok {
				p.logger.Error("no dispatch channel for venue",
					slog.String("order_id", string(order.ID)),
					slog.String("venue", venueID),
				)
				continue
			}

			select {
			case ch <- venueOrder{order: order}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// venueDispatch calls venue.SubmitOrder, transitions the order to Acknowledged,
// and persists the update. Each venue has its own goroutine and channel.
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

			p.logger.Info("order acknowledged",
				slog.String("order_id", string(order.ID)),
				slog.String("venue", venue.VenueID()),
			)
			p.notifyOrderUpdate(ctx, order)
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
		// Position doesn't exist yet -- create a new one
		pos = &domain.Position{
			InstrumentID: order.InstrumentID,
			VenueID:      order.VenueID,
			AssetClass:   order.AssetClass,
		}
	}

	if err := pos.ApplyFill(fill, order.Side); err != nil {
		p.logger.Error("failed to apply fill to position",
			slog.String("fill_id", fill.ID),
			slog.String("error", err.Error()),
		)
		return
	}

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

	p.notifyOrderUpdate(ctx, order)
	p.notifier.NotifyPositionUpdate(pos)

	// Clean up fully filled orders from the in-memory map
	if order.Status == domain.OrderStatusFilled {
		p.orderMu.Lock()
		delete(p.orderMap, order.ID)
		p.orderMu.Unlock()
	}
}

// notifyOrderUpdate sends an order update to both the WebSocket notifier and
// (optionally) the Kafka producer.
func (p *Pipeline) notifyOrderUpdate(ctx context.Context, order *domain.Order) {
	p.notifier.NotifyOrderUpdate(order)

	if p.kafka != nil {
		payload, err := json.Marshal(order)
		if err != nil {
			p.logger.Error("failed to marshal order for Kafka",
				slog.String("order_id", string(order.ID)),
				slog.String("error", err.Error()),
			)
			return
		}
		if err := p.kafka.PublishOrderLifecycle(ctx, order.InstrumentID, payload); err != nil {
			p.logger.Error("failed to publish order lifecycle event",
				slog.String("order_id", string(order.ID)),
				slog.String("error", err.Error()),
			)
		}
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
