package pipeline

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
)

const intakeChanSize = 10_000

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

// venueOrder couples an order with its dispatch context, carrying the order
// from the router stage to the venue dispatch stage.
type venueOrder struct {
	order *domain.Order
}

// Pipeline orchestrates the order lifecycle:
// Submit -> intake chan -> Router -> Venue Dispatch -> Fill Collector -> Notifier
type Pipeline struct {
	store    Store
	venue    adapter.LiquidityProvider
	notifier Notifier
	logger   *slog.Logger

	intake   chan *domain.Order
	dispatch chan venueOrder

	// orderMap tracks in-flight orders by ID so the fill collector
	// can look them up without a store round-trip.
	orderMu  sync.RWMutex
	orderMap map[domain.OrderID]*domain.Order

	wg sync.WaitGroup
}

// NewPipeline creates a new order processing pipeline.
func NewPipeline(store Store, venue adapter.LiquidityProvider, notifier Notifier) *Pipeline {
	return &Pipeline{
		store:    store,
		venue:    venue,
		notifier: notifier,
		logger:   logging.NewDefault("gateway", "pipeline"),
		intake:   make(chan *domain.Order, intakeChanSize),
		dispatch: make(chan venueOrder, intakeChanSize),
		orderMap: make(map[domain.OrderID]*domain.Order),
	}
}

// Start launches the pipeline goroutines. All goroutines respect ctx for
// cancellation and will drain within 5 seconds.
func (p *Pipeline) Start(ctx context.Context) {
	p.wg.Add(3)
	go p.router(ctx)
	go p.venueDispatch(ctx)
	go p.fillCollector(ctx)
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

// router reads orders from the intake channel, sets the venue ID,
// persists the order as New, and forwards to venue dispatch.
func (p *Pipeline) router(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case order, ok := <-p.intake:
			if !ok {
				return
			}
			order.VenueID = p.venue.VenueID()

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

			select {
			case p.dispatch <- venueOrder{order: order}:
			case <-ctx.Done():
				return
			}
		}
	}
}

// venueDispatch calls venue.SubmitOrder, transitions the order to Acknowledged,
// and persists the update.
func (p *Pipeline) venueDispatch(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case vo, ok := <-p.dispatch:
			if !ok {
				return
			}
			order := vo.order

			_, err := p.venue.SubmitOrder(ctx, order)
			if err != nil {
				p.logger.Error("venue rejected order",
					slog.String("order_id", string(order.ID)),
					slog.String("error", err.Error()),
				)
				_ = order.ApplyTransition(domain.OrderStatusRejected)
				_ = p.store.UpdateOrder(ctx, order)
				p.notifier.NotifyOrderUpdate(order)
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
			)
			p.notifier.NotifyOrderUpdate(order)
		}
	}
}

// fillCollector reads fills from the venue's fill feed, applies them to orders,
// updates positions, persists everything, and notifies.
func (p *Pipeline) fillCollector(ctx context.Context) {
	defer p.wg.Done()
	fillCh := p.venue.FillFeed()
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
		// Position doesn't exist yet — create a new one
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

	p.notifier.NotifyOrderUpdate(order)
	p.notifier.NotifyPositionUpdate(pos)

	// Clean up fully filled orders from the in-memory map
	if order.Status == domain.OrderStatusFilled {
		p.orderMu.Lock()
		delete(p.orderMap, order.ID)
		p.orderMu.Unlock()
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
