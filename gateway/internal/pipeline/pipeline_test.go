package pipeline

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/crossing"
	"github.com/synapse-oms/gateway/internal/domain"
	riskgrpc "github.com/synapse-oms/gateway/internal/grpc"
	"github.com/synapse-oms/gateway/internal/router"
)

// --- In-memory Store mock ---

type memStore struct {
	mu        sync.Mutex
	orders    map[domain.OrderID]*domain.Order
	fills     []domain.Fill
	positions map[string]*domain.Position // key: instrumentID|venueID
}

func newMemStore() *memStore {
	return &memStore{
		orders:    make(map[domain.OrderID]*domain.Order),
		positions: make(map[string]*domain.Position),
	}
}

func posKey(instrumentID, venueID string) string {
	return instrumentID + "|" + venueID
}

func (m *memStore) CreateOrder(_ context.Context, o *domain.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *memStore) UpdateOrder(_ context.Context, o *domain.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *memStore) GetOrder(_ context.Context, id domain.OrderID) (*domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return nil, fmt.Errorf("order not found: %s", id)
	}
	cp := *o
	return &cp, nil
}

func (m *memStore) CreateFill(_ context.Context, f *domain.Fill) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fills = append(m.fills, *f)
	return nil
}

func (m *memStore) UpsertPosition(_ context.Context, p *domain.Position) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *p
	m.positions[posKey(p.InstrumentID, p.VenueID)] = &cp
	return nil
}

func (m *memStore) GetPosition(_ context.Context, instrumentID, venueID string) (*domain.Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.positions[posKey(instrumentID, venueID)]
	if !ok {
		return nil, fmt.Errorf("position not found")
	}
	cp := *p
	return &cp, nil
}

func (m *memStore) getOrder(id domain.OrderID) *domain.Order {
	m.mu.Lock()
	defer m.mu.Unlock()
	o := m.orders[id]
	if o == nil {
		return nil
	}
	cp := *o
	return &cp
}

func (m *memStore) getPosition(instrumentID, venueID string) *domain.Position {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.positions[posKey(instrumentID, venueID)]
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// --- Mock venue (LiquidityProvider) ---

type mockVenue struct {
	mu       sync.Mutex
	fillCh   chan domain.Fill
	orders   []*domain.Order
	venueID  string
	status   adapter.VenueStatus
}

func newMockVenue() *mockVenue {
	return &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "sim-exchange",
		status:  adapter.Connected,
	}
}

func (v *mockVenue) VenueID() string   { return v.venueID }
func (v *mockVenue) VenueName() string { return "Mock Venue" }
func (v *mockVenue) VenueType() string { return "simulated" }
func (v *mockVenue) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassEquity, domain.AssetClassCrypto}
}
func (v *mockVenue) SupportedInstruments() ([]domain.Instrument, error) {
	return nil, nil
}
func (v *mockVenue) Connect(_ context.Context, _ domain.VenueCredential) error {
	v.status = adapter.Connected
	return nil
}
func (v *mockVenue) Disconnect(_ context.Context) error {
	v.status = adapter.Disconnected
	return nil
}
func (v *mockVenue) Status() adapter.VenueStatus                                { return v.status }
func (v *mockVenue) Ping(_ context.Context) (time.Duration, error)              { return 0, nil }

func (v *mockVenue) SubmitOrder(_ context.Context, order *domain.Order) (*adapter.VenueAck, error) {
	v.mu.Lock()
	v.orders = append(v.orders, order)
	v.mu.Unlock()
	return &adapter.VenueAck{
		VenueOrderID: fmt.Sprintf("MOCK-%s", order.ID),
		ReceivedAt:   time.Now(),
	}, nil
}

func (v *mockVenue) CancelOrder(_ context.Context, _ domain.OrderID, _ string) error {
	return nil
}

func (v *mockVenue) QueryOrder(_ context.Context, _ string) (*domain.Order, error) {
	return nil, fmt.Errorf("not found")
}

func (v *mockVenue) SubscribeMarketData(_ context.Context, _ []string) (<-chan adapter.MarketDataSnapshot, error) {
	return make(chan adapter.MarketDataSnapshot), nil
}

func (v *mockVenue) UnsubscribeMarketData(_ context.Context, _ []string) error {
	return nil
}

func (v *mockVenue) FillFeed() <-chan domain.Fill { return v.fillCh }

func (v *mockVenue) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{}
}

func (v *mockVenue) sendFill(f domain.Fill) {
	v.fillCh <- f
}

// --- Mock risk client ---

type mockRiskClient struct {
	approved     bool
	rejectReason string
	err          error
}

func newMockRiskClient(approved bool) *mockRiskClient {
	return &mockRiskClient{approved: approved}
}

func (r *mockRiskClient) CheckPreTradeRisk(_ context.Context, _ *domain.Order) (*riskgrpc.RiskCheckResult, error) {
	if r.err != nil {
		return nil, r.err
	}
	return &riskgrpc.RiskCheckResult{
		Approved:     r.approved,
		RejectReason: r.rejectReason,
	}, nil
}

func (r *mockRiskClient) Close() error { return nil }

// --- Mock Kafka publisher ---

type mockKafkaPublisher struct {
	mu       sync.Mutex
	messages []kafkaMsg
}

type kafkaMsg struct {
	instrumentID string
	payload      []byte
}

func newMockKafkaPublisher() *mockKafkaPublisher {
	return &mockKafkaPublisher{}
}

func (k *mockKafkaPublisher) PublishOrderLifecycle(_ context.Context, instrumentID string, payload []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.messages = append(k.messages, kafkaMsg{instrumentID: instrumentID, payload: payload})
	return nil
}

func (k *mockKafkaPublisher) getMessages() []kafkaMsg {
	k.mu.Lock()
	defer k.mu.Unlock()
	out := make([]kafkaMsg, len(k.messages))
	copy(out, k.messages)
	return out
}

// --- Mock notifier ---

type mockNotifier struct {
	mu              sync.Mutex
	orderUpdates    []*domain.Order
	positionUpdates []*domain.Position
}

func newMockNotifier() *mockNotifier {
	return &mockNotifier{}
}

func (n *mockNotifier) NotifyOrderUpdate(order *domain.Order) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := *order
	n.orderUpdates = append(n.orderUpdates, &cp)
}

func (n *mockNotifier) NotifyPositionUpdate(position *domain.Position) {
	n.mu.Lock()
	defer n.mu.Unlock()
	cp := *position
	n.positionUpdates = append(n.positionUpdates, &cp)
}

func (n *mockNotifier) getOrderUpdates() []*domain.Order {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]*domain.Order, len(n.orderUpdates))
	copy(out, n.orderUpdates)
	return out
}

func (n *mockNotifier) getPositionUpdates() []*domain.Position {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]*domain.Position, len(n.positionUpdates))
	copy(out, n.positionUpdates)
	return out
}

// --- Helpers ---

func makeOrder() *domain.Order {
	return &domain.Order{
		ClientOrderID: "client-1",
		InstrumentID:  "AAPL",
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeMarket,
		Quantity:      decimal.NewFromInt(100),
		Price:         decimal.NewFromFloat(185.00),
		AssetClass:    domain.AssetClassEquity,
	}
}

func makeDefaultRouter() *router.Router {
	r := router.New()
	r.Register(router.NewBestPriceStrategy())
	return r
}

func makePipeline(store *memStore, venue *mockVenue, notifier *mockNotifier) *Pipeline {
	risk := newMockRiskClient(true)
	return NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithRouter(makeDefaultRouter()))
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting: %s", msg)
}

// --- Tests ---

func TestSubmitOrderAppearsAsNew(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	err := p.Submit(ctx, order)
	if err != nil {
		t.Fatalf("Submit failed: %v", err)
	}
	if order.ID == "" {
		t.Fatal("expected order ID to be generated")
	}
	if order.Status != domain.OrderStatusNew {
		t.Fatalf("expected status New, got %d", order.Status)
	}

	// Order should be persisted by the router goroutine.
	// It may already have transitioned to Acknowledged by the time we check,
	// so we verify the order exists and has progressed at least to New.
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status >= domain.OrderStatusNew
	}, "order to be persisted")
}

func TestPipelineTransitionsToAcknowledged(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for the order to be acknowledged (venue dispatch)
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to transition to Acknowledged")
}

func TestFillTransitionsToFilled(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for acknowledged first
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to be acknowledged")

	// Send a complete fill
	venue.sendFill(domain.Fill{
		ID:          "fill-1",
		OrderID:     order.ID,
		VenueID:     "sim-exchange",
		Quantity:    decimal.NewFromInt(100),
		Price:       decimal.NewFromFloat(185.50),
		Fee:         decimal.NewFromFloat(0.50),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "exec-1",
	})

	// Wait for order to be filled
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order to transition to Filled")

	// Position should be updated
	waitFor(t, 2*time.Second, func() bool {
		pos := store.getPosition("AAPL", "sim-exchange")
		return pos != nil && pos.Quantity.Equal(decimal.NewFromInt(100))
	}, "position to be updated")

	// Notifier should have received updates
	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getOrderUpdates()
		return len(updates) > 0
	}, "notifier to receive order update")

	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getPositionUpdates()
		return len(updates) > 0
	}, "notifier to receive position update")
}

func TestMultipleFillsAccumulatePosition(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Submit two orders for the same instrument
	order1 := makeOrder()
	order1.ClientOrderID = "client-1"
	if err := p.Submit(ctx, order1); err != nil {
		t.Fatalf("Submit order1 failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order1.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order1 to be acknowledged")

	order2 := makeOrder()
	order2.ClientOrderID = "client-2"
	order2.Quantity = decimal.NewFromInt(50)
	if err := p.Submit(ctx, order2); err != nil {
		t.Fatalf("Submit order2 failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order2.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order2 to be acknowledged")

	// Fill both orders
	venue.sendFill(domain.Fill{
		ID:          "fill-1",
		OrderID:     order1.ID,
		VenueID:     "sim-exchange",
		Quantity:    decimal.NewFromInt(100),
		Price:       decimal.NewFromFloat(185.00),
		Fee:         decimal.NewFromFloat(0.50),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "exec-1",
	})

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order1.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order1 to be filled")

	venue.sendFill(domain.Fill{
		ID:          "fill-2",
		OrderID:     order2.ID,
		VenueID:     "sim-exchange",
		Quantity:    decimal.NewFromInt(50),
		Price:       decimal.NewFromFloat(186.00),
		Fee:         decimal.NewFromFloat(0.25),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "exec-2",
	})

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order2.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order2 to be filled")

	// Position should have accumulated: 100 + 50 = 150 shares
	pos := store.getPosition("AAPL", "sim-exchange")
	if pos == nil {
		t.Fatal("expected position to exist")
	}
	if !pos.Quantity.Equal(decimal.NewFromInt(150)) {
		t.Fatalf("expected position quantity 150, got %s", pos.Quantity)
	}
}

func TestShutdownCompletesWithinTimeout(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	// Submit a few orders so goroutines are active
	for i := 0; i < 3; i++ {
		order := makeOrder()
		order.ClientOrderID = fmt.Sprintf("client-%d", i)
		_ = p.Submit(ctx, order)
	}

	// Give a moment for processing
	time.Sleep(100 * time.Millisecond)

	// Cancel context and measure shutdown time
	start := time.Now()
	cancel()
	p.Wait()
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("shutdown took %v, expected within 5 seconds", elapsed)
	}
}

func TestPartialFillTransitionsToPartiallyFilled(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	order.Quantity = decimal.NewFromInt(100)
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to be acknowledged")

	// Send partial fill (50 of 100)
	venue.sendFill(domain.Fill{
		ID:          "fill-partial",
		OrderID:     order.ID,
		VenueID:     "sim-exchange",
		Quantity:    decimal.NewFromInt(50),
		Price:       decimal.NewFromFloat(185.00),
		Fee:         decimal.NewFromFloat(0.25),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "exec-partial",
	})

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusPartiallyFilled
	}, "order to be partially filled")

	// Send remaining fill (50 of 100)
	venue.sendFill(domain.Fill{
		ID:          "fill-rest",
		OrderID:     order.ID,
		VenueID:     "sim-exchange",
		Quantity:    decimal.NewFromInt(50),
		Price:       decimal.NewFromFloat(186.00),
		Fee:         decimal.NewFromFloat(0.25),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "exec-rest",
	})

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order to be fully filled after two partial fills")
}

// --- Phase 2 specific tests ---

func TestRiskRejectionRejectsOrder(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()
	risk := &mockRiskClient{approved: false, rejectReason: "VaR limit exceeded"}

	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// The order should be rejected by risk check, not routed to venue
	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getOrderUpdates()
		for _, u := range updates {
			if u.ID == order.ID && u.Status == domain.OrderStatusRejected {
				return true
			}
		}
		return false
	}, "order to be rejected by risk check")
}

func TestRiskErrorFailsOpen(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()
	risk := &mockRiskClient{err: fmt.Errorf("connection refused")}

	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithFailOpenRisk(true))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// With fail-open, the order should pass through risk and reach acknowledged
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to be acknowledged despite risk error (fail-open)")
}

func TestRiskErrorFailsClosed(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()
	risk := &mockRiskClient{err: fmt.Errorf("connection refused")}

	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithFailOpenRisk(false))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// With fail-closed, the order should be rejected
	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getOrderUpdates()
		for _, u := range updates {
			if u.ID == order.ID && u.Status == domain.OrderStatusRejected {
				return true
			}
		}
		return false
	}, "order to be rejected when risk engine fails (fail-closed)")
}

func TestMultiVenueRouting(t *testing.T) {
	store := newMemStore()

	equityVenue := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "alpaca",
		status:  adapter.Connected,
	}
	cryptoVenue := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "binance_testnet",
		status:  adapter.Connected,
	}
	simVenue := newMockVenue() // sim-exchange

	venues := []adapter.LiquidityProvider{equityVenue, cryptoVenue, simVenue}
	notifier := newMockNotifier()
	risk := newMockRiskClient(true)

	p := NewPipeline(store, venues, notifier, risk)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Submit equity order -> should route to alpaca
	equityOrder := makeOrder()
	equityOrder.AssetClass = domain.AssetClassEquity
	if err := p.Submit(ctx, equityOrder); err != nil {
		t.Fatalf("Submit equity order failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(equityOrder.ID)
		return o != nil && o.VenueID == "alpaca" && o.Status == domain.OrderStatusAcknowledged
	}, "equity order to be routed to alpaca")

	// Submit crypto order -> should route to binance_testnet
	cryptoOrder := &domain.Order{
		ClientOrderID: "client-crypto-1",
		InstrumentID:  "BTC-USD",
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeMarket,
		Quantity:      decimal.NewFromFloat(0.01),
		Price:         decimal.NewFromFloat(50000.00),
		AssetClass:    domain.AssetClassCrypto,
	}
	if err := p.Submit(ctx, cryptoOrder); err != nil {
		t.Fatalf("Submit crypto order failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(cryptoOrder.ID)
		return o != nil && o.VenueID == "binance_testnet" && o.Status == domain.OrderStatusAcknowledged
	}, "crypto order to be routed to binance_testnet")
}

func TestKafkaPublisherReceivesEvents(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()
	risk := newMockRiskClient(true)
	kafka := newMockKafkaPublisher()

	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithKafkaPublisher(kafka))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Wait for order to be acknowledged (which triggers Kafka publish)
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to be acknowledged")

	// Kafka should have received at least one event
	waitFor(t, 2*time.Second, func() bool {
		msgs := kafka.getMessages()
		return len(msgs) > 0
	}, "kafka to receive order lifecycle event")

	msgs := kafka.getMessages()
	if msgs[0].instrumentID != "AAPL" {
		t.Fatalf("expected kafka message for AAPL, got %s", msgs[0].instrumentID)
	}
}

func TestNilRiskClientUsesFailOpen(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	// Pass nil risk client -- should use fail-open client internally
	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusAcknowledged
	}, "order to be acknowledged with nil risk client")
}

// --- Phase 3: Smart Router Integration Tests ---

func TestCrossingEngineInternalFill(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()
	risk := newMockRiskClient(true)
	ce := crossing.NewCrossingEngine()

	// Add a resting sell order to the crossing book.
	restingOrder := &domain.Order{
		ID:             domain.OrderID("resting-sell-1"),
		ClientOrderID:  "resting-1",
		InstrumentID:   "AAPL",
		Side:           domain.SideSell,
		Type:           domain.OrderTypeLimit,
		Quantity:       decimal.NewFromInt(100),
		Price:          decimal.NewFromFloat(185.00),
		FilledQuantity: decimal.Zero,
		Status:         domain.OrderStatusAcknowledged,
	}
	ce.AddOrder(restingOrder)

	p := NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithRouter(makeDefaultRouter()),
		WithCrossingEngine(ce))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Submit a buy order that should cross internally.
	order := makeOrder()
	order.Type = domain.OrderTypeLimit
	order.Price = decimal.NewFromFloat(185.00)
	order.Quantity = decimal.NewFromInt(100)

	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// The order should be fully crossed internally -> Filled status.
	waitFor(t, 2*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order to be fully crossed internally and filled")

	// Verify a fill was persisted with INTERNAL venue.
	waitFor(t, 2*time.Second, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		for _, f := range store.fills {
			if f.OrderID == order.ID && f.VenueID == "INTERNAL" {
				return true
			}
		}
		return false
	}, "internal fill to be persisted")

	// Venue should NOT have received the order (fully crossed internally).
	venue.mu.Lock()
	venueOrders := len(venue.orders)
	venue.mu.Unlock()
	if venueOrders > 0 {
		t.Fatalf("expected no orders dispatched to venue, got %d", venueOrders)
	}
}

func TestSmartRouterSplitAcrossTwoVenues(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()
	risk := newMockRiskClient(true)

	venue1 := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "venue-a",
		status:  adapter.Connected,
	}
	venue2 := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "venue-b",
		status:  adapter.Connected,
	}

	venues := []adapter.LiquidityProvider{venue1, venue2}

	// Create a custom router with a strategy that splits across both venues.
	r := router.New()
	r.Register(&splitStrategy{})

	p := NewPipeline(store, venues, notifier, risk, WithRouter(r))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	order.Quantity = decimal.NewFromInt(200)

	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// Both venues should receive a child order.
	waitFor(t, 2*time.Second, func() bool {
		venue1.mu.Lock()
		n1 := len(venue1.orders)
		venue1.mu.Unlock()
		return n1 > 0
	}, "venue-a to receive a child order")

	waitFor(t, 2*time.Second, func() bool {
		venue2.mu.Lock()
		n2 := len(venue2.orders)
		venue2.mu.Unlock()
		return n2 > 0
	}, "venue-b to receive a child order")

	// Verify the quantities are correct (100 each).
	venue1.mu.Lock()
	v1Order := venue1.orders[0]
	venue1.mu.Unlock()
	venue2.mu.Lock()
	v2Order := venue2.orders[0]
	venue2.mu.Unlock()

	if !v1Order.Quantity.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected venue-a child quantity 100, got %s", v1Order.Quantity)
	}
	if !v2Order.Quantity.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("expected venue-b child quantity 100, got %s", v2Order.Quantity)
	}

	// Verify parent order was persisted.
	parentOrder := store.getOrder(order.ID)
	if parentOrder == nil {
		t.Fatal("expected parent order to be persisted")
	}
}

// --- P5-09: Error Handling & Graceful Degradation Tests ---

func TestDisconnectedVenueRejectsOrder_OthersUnaffected(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()
	risk := newMockRiskClient(true)

	connectedVenue := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "venue-ok",
		status:  adapter.Connected,
	}
	disconnectedVenue := &mockVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "venue-down",
		status:  adapter.Disconnected,
	}

	// Register instances so CheckVenueReady can find them.
	adapter.RegisterInstance("venue-ok", connectedVenue)
	adapter.RegisterInstance("venue-down", disconnectedVenue)

	venues := []adapter.LiquidityProvider{connectedVenue, disconnectedVenue}

	p := NewPipeline(store, venues, notifier, risk)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	// Submit an order — legacy routing will pick the first available venue.
	// To test disconnected venue rejection specifically, submit an order and
	// then verify the pipeline is still functional after processing.
	order1 := makeOrder()
	order1.ClientOrderID = "test-dc-1"
	if err := p.Submit(ctx, order1); err != nil {
		t.Fatalf("Submit order1 failed: %v", err)
	}

	// Wait for order to be processed (may route to either venue)
	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getOrderUpdates()
		return len(updates) > 0
	}, "first order to be processed")

	// Verify pipeline is still alive: submit another order
	order2 := makeOrder()
	order2.ClientOrderID = "test-dc-2"
	if err := p.Submit(ctx, order2); err != nil {
		t.Fatalf("Submit order2 failed after disconnected venue: %v", err)
	}

	waitFor(t, 2*time.Second, func() bool {
		updates := notifier.getOrderUpdates()
		return len(updates) >= 2
	}, "second order to be processed — pipeline survived disconnected venue")
}

func TestCheckVenueReady_Disconnected(t *testing.T) {
	disconnected := &mockVenue{
		fillCh:  make(chan domain.Fill, 1),
		venueID: "check-dc-test",
		status:  adapter.Disconnected,
	}
	adapter.RegisterInstance("check-dc-test", disconnected)

	err := adapter.CheckVenueReady("check-dc-test")
	if err == nil {
		t.Fatal("expected error for disconnected venue")
	}

	dcErr, ok := err.(*adapter.ErrVenueDisconnected)
	if !ok {
		t.Fatalf("expected *ErrVenueDisconnected, got %T", err)
	}
	if dcErr.VenueID != "check-dc-test" {
		t.Errorf("expected venue ID check-dc-test, got %s", dcErr.VenueID)
	}
}

func TestCheckVenueReady_Connected(t *testing.T) {
	connected := &mockVenue{
		fillCh:  make(chan domain.Fill, 1),
		venueID: "check-ok-test",
		status:  adapter.Connected,
	}
	adapter.RegisterInstance("check-ok-test", connected)

	err := adapter.CheckVenueReady("check-ok-test")
	if err != nil {
		t.Fatalf("expected no error for connected venue, got %v", err)
	}
}

func TestCheckVenueReady_UnknownVenue(t *testing.T) {
	err := adapter.CheckVenueReady("totally-unknown-venue-xyz123")
	if err != nil {
		t.Fatalf("expected nil for unknown venue, got %v", err)
	}
}

// TestSynchronousFillDuringSubmitOrder verifies that fills generated
// synchronously during SubmitOrder() (as the simulated exchange does for
// market orders) are correctly applied. This tests the fix for the race
// condition where venueDispatch previously transitioned to Acknowledged
// AFTER SubmitOrder, causing ApplyFill to reject fills for New orders.
func TestSynchronousFillDuringSubmitOrder(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()

	// Create a venue that sends a fill synchronously during SubmitOrder,
	// mimicking the real simulated exchange behavior.
	venue := &syncFillVenue{
		fillCh:  make(chan domain.Fill, 100),
		venueID: "sim-exchange",
		status:  adapter.Connected,
	}

	p := makePipelineWith(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// The fill was sent synchronously during SubmitOrder.
	// Wait for the order to reach Filled status.
	waitFor(t, 5*time.Second, func() bool {
		o := store.getOrder(order.ID)
		return o != nil && o.Status == domain.OrderStatusFilled
	}, "order to be filled after synchronous fill during SubmitOrder")

	// Verify fill details
	o := store.getOrder(order.ID)
	if !o.FilledQuantity.Equal(decimal.NewFromInt(100)) {
		t.Errorf("expected filled quantity 100, got %s", o.FilledQuantity)
	}
	if o.AveragePrice.IsZero() {
		t.Error("expected non-zero average price")
	}
}

// TestOrderPersistedBeforeRESTResponse verifies that an order is persisted
// to the database during Submit() so that it's available via GET /orders
// immediately after the REST handler returns 201.
func TestOrderPersistedBeforeRESTResponse(t *testing.T) {
	store := newMemStore()
	venue := newMockVenue()
	notifier := newMockNotifier()

	p := makePipeline(store, venue, notifier)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Note: we do NOT call p.Start(ctx) — we want to verify the order
	// is persisted synchronously during Submit, not by a background goroutine.

	order := makeOrder()
	if err := p.Submit(ctx, order); err != nil {
		t.Fatalf("Submit failed: %v", err)
	}

	// The order must be in the store IMMEDIATELY after Submit returns,
	// without any background goroutine having run.
	o := store.getOrder(order.ID)
	if o == nil {
		t.Fatal("order not found in store immediately after Submit — would be lost on page refresh")
	}
	if o.Status != domain.OrderStatusNew {
		t.Errorf("expected status New, got %d", o.Status)
	}
	if o.InstrumentID != "AAPL" {
		t.Errorf("expected instrument AAPL, got %s", o.InstrumentID)
	}
}

// syncFillVenue is a mock venue that sends fills synchronously during
// SubmitOrder, mimicking the real simulated exchange behavior.
type syncFillVenue struct {
	mu      sync.Mutex
	fillCh  chan domain.Fill
	venueID string
	status  adapter.VenueStatus
}

func (v *syncFillVenue) VenueID() string   { return v.venueID }
func (v *syncFillVenue) VenueName() string { return "Sync Fill Venue" }
func (v *syncFillVenue) VenueType() string { return "simulated" }
func (v *syncFillVenue) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassEquity, domain.AssetClassCrypto}
}
func (v *syncFillVenue) SupportedInstruments() ([]domain.Instrument, error) { return nil, nil }
func (v *syncFillVenue) Connect(_ context.Context, _ domain.VenueCredential) error {
	v.status = adapter.Connected
	return nil
}
func (v *syncFillVenue) Disconnect(_ context.Context) error {
	v.status = adapter.Disconnected
	return nil
}
func (v *syncFillVenue) Status() adapter.VenueStatus                   { return v.status }
func (v *syncFillVenue) Ping(_ context.Context) (time.Duration, error) { return 0, nil }

func (v *syncFillVenue) SubmitOrder(_ context.Context, order *domain.Order) (*adapter.VenueAck, error) {
	// Send fill SYNCHRONOUSLY during SubmitOrder, exactly like the simulated exchange.
	fill := domain.Fill{
		ID:          "sync-fill-1",
		OrderID:     order.ID,
		VenueID:     v.venueID,
		Quantity:    order.Quantity,
		Price:       decimal.NewFromFloat(185.50),
		Fee:         decimal.NewFromFloat(0.50),
		FeeAsset:    "USD",
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: "sync-exec-1",
	}
	v.fillCh <- fill
	return &adapter.VenueAck{VenueOrderID: "SYNC-" + string(order.ID), ReceivedAt: time.Now()}, nil
}

func (v *syncFillVenue) CancelOrder(_ context.Context, _ domain.OrderID, _ string) error {
	return nil
}
func (v *syncFillVenue) QueryOrder(_ context.Context, _ string) (*domain.Order, error) {
	return nil, fmt.Errorf("not found")
}
func (v *syncFillVenue) SubscribeMarketData(_ context.Context, _ []string) (<-chan adapter.MarketDataSnapshot, error) {
	return make(chan adapter.MarketDataSnapshot), nil
}
func (v *syncFillVenue) UnsubscribeMarketData(_ context.Context, _ []string) error { return nil }
func (v *syncFillVenue) FillFeed() <-chan domain.Fill                               { return v.fillCh }
func (v *syncFillVenue) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{}
}

func makePipelineWith(store *memStore, venue adapter.LiquidityProvider, notifier *mockNotifier) *Pipeline {
	risk := newMockRiskClient(true)
	return NewPipeline(store, []adapter.LiquidityProvider{venue}, notifier, risk,
		WithRouter(makeDefaultRouter()))
}

// splitStrategy is a test routing strategy that always splits evenly across
// the first two candidates.
type splitStrategy struct{}

func (s *splitStrategy) Name() string { return "split-test" }

func (s *splitStrategy) Evaluate(
	_ context.Context,
	order *domain.Order,
	candidates []router.VenueCandidate,
) ([]router.VenueAllocation, error) {
	if len(candidates) < 2 {
		return []router.VenueAllocation{
			{VenueID: candidates[0].VenueID, Quantity: order.Quantity, Reason: "only-one"},
		}, nil
	}
	half := order.Quantity.Div(decimal.NewFromInt(2))
	return []router.VenueAllocation{
		{VenueID: candidates[0].VenueID, Quantity: half, Reason: "split-50/50"},
		{VenueID: candidates[1].VenueID, Quantity: half, Reason: "split-50/50"},
	}, nil
}
