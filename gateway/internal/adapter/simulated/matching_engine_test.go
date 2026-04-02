package simulated

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

// Test 1: GBM price walk produces changing prices with variance scaling with time
func TestPriceWalk_GBM_VarianceScalesWithTime(t *testing.T) {
	initialPrice := decimal.NewFromFloat(100.0)
	volatility := 0.30
	drift := 0.05
	interval := 100 * time.Millisecond

	// Run many steps and collect prices
	pw := NewPriceWalk(initialPrice, volatility, drift, interval)

	prices := make([]float64, 1000)
	for i := 0; i < 1000; i++ {
		pw.Step()
		prices[i] = pw.CurrentPrice().InexactFloat64()
	}

	// Compute variance of log-returns
	logReturns := make([]float64, len(prices)-1)
	for i := 1; i < len(prices); i++ {
		logReturns[i-1] = math.Log(prices[i] / prices[i-1])
	}

	mean := 0.0
	for _, r := range logReturns {
		mean += r
	}
	mean /= float64(len(logReturns))

	variance := 0.0
	for _, r := range logReturns {
		variance += (r - mean) * (r - mean)
	}
	variance /= float64(len(logReturns))

	// Variance should be positive (prices are changing)
	if variance <= 0 {
		t.Fatalf("expected positive variance in log returns, got %f", variance)
	}

	// The per-step variance should be approximately sigma^2 * dt
	dt := interval.Seconds() / (365.25 * 24 * 3600)
	expectedVariance := volatility * volatility * dt
	ratio := variance / expectedVariance
	// Allow wide tolerance (Monte Carlo), but should be in ballpark [0.3, 3.0]
	if ratio < 0.3 || ratio > 3.0 {
		t.Errorf("variance ratio out of expected range: got %f (variance=%f, expected=%f)", ratio, variance, expectedVariance)
	}

	// Verify no mean reversion: price should not be forced back to initial
	// (It's stochastic, so we just verify the final price differs from initial)
	finalPrice := pw.CurrentPrice()
	if finalPrice.Equal(initialPrice) {
		t.Error("after 1000 steps, price should not exactly equal initial price")
	}
}

// Test 2: Market order fills immediately within 5bps of synthetic price
func TestMatchingEngine_MarketOrderFill(t *testing.T) {
	fillCh := make(chan domain.Fill, 100)
	engine := NewMatchingEngine(fillCh)
	engine.RegisterInstrument("AAPL", decimal.NewFromFloat(185.0), 0.30, 0.05, 100*time.Millisecond)

	order := &domain.Order{
		ID:           "order-1",
		InstrumentID: "AAPL",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromInt(100),
		AssetClass:   domain.AssetClassEquity,
	}

	engine.ProcessOrder(order)

	select {
	case fill := <-fillCh:
		if fill.OrderID != "order-1" {
			t.Errorf("expected order ID order-1, got %s", fill.OrderID)
		}
		if !fill.Quantity.Equal(decimal.NewFromInt(100)) {
			t.Errorf("expected fill quantity 100, got %s", fill.Quantity)
		}
		// Verify fill price is within 5bps of the synthetic price (185.0)
		syntheticPrice := decimal.NewFromFloat(185.0)
		bps := slippageBps(syntheticPrice, fill.Price)
		if bps > 5.0 {
			t.Errorf("slippage %f bps exceeds 5bps limit", bps)
		}
		if fill.VenueID != "simulated" {
			t.Errorf("expected venue ID simulated, got %s", fill.VenueID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for market order fill")
	}
}

// Test 3: Limit order queues and fills when price crosses
func TestMatchingEngine_LimitOrderFill(t *testing.T) {
	fillCh := make(chan domain.Fill, 100)
	engine := NewMatchingEngine(fillCh)

	// Set initial price at 185, place buy limit at 190 (above current, should fill when price crosses)
	engine.RegisterInstrument("AAPL", decimal.NewFromFloat(185.0), 0.30, 0.05, 100*time.Millisecond)

	// Buy limit at 190 should fill immediately since 185 <= 190
	order := &domain.Order{
		ID:           "limit-1",
		InstrumentID: "AAPL",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeLimit,
		Price:        decimal.NewFromFloat(190.0),
		Quantity:     decimal.NewFromInt(50),
		AssetClass:   domain.AssetClassEquity,
	}

	engine.ProcessOrder(order)
	// Manually trigger a sweep
	engine.sweepAll()

	select {
	case fill := <-fillCh:
		if fill.OrderID != "limit-1" {
			t.Errorf("expected order ID limit-1, got %s", fill.OrderID)
		}
		if !fill.Quantity.Equal(decimal.NewFromInt(50)) {
			t.Errorf("expected fill quantity 50, got %s", fill.Quantity)
		}
		// Limit orders fill at limit price
		if !fill.Price.Equal(decimal.NewFromFloat(190.0)) {
			t.Errorf("expected fill at limit price 190, got %s", fill.Price)
		}
		// Limit orders are maker liquidity
		if fill.Liquidity != domain.LiquidityMaker {
			t.Errorf("expected maker liquidity, got %d", fill.Liquidity)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for limit order fill")
	}
}

// Test 4: Limit order that should NOT fill (buy limit below current price)
func TestMatchingEngine_LimitOrderNotFilled(t *testing.T) {
	fillCh := make(chan domain.Fill, 100)
	engine := NewMatchingEngine(fillCh)
	engine.RegisterInstrument("AAPL", decimal.NewFromFloat(185.0), 0.30, 0.05, 100*time.Millisecond)

	// Buy limit at 180, price is 185 - should NOT fill
	order := &domain.Order{
		ID:           "limit-nofill",
		InstrumentID: "AAPL",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeLimit,
		Price:        decimal.NewFromFloat(180.0),
		Quantity:     decimal.NewFromInt(50),
		AssetClass:   domain.AssetClassEquity,
	}

	engine.ProcessOrder(order)
	engine.sweepAll()

	select {
	case fill := <-fillCh:
		t.Fatalf("limit order should not have filled, but got fill: %+v", fill)
	case <-time.After(100 * time.Millisecond):
		// Expected: no fill
	}
}

// Test 5: Equity fee calculation ($0.005/share)
func TestFeeCalculation_Equity(t *testing.T) {
	quantity := decimal.NewFromInt(100)
	price := decimal.NewFromFloat(185.0)

	fee := calculateFee(domain.AssetClassEquity, quantity, price)
	expected := decimal.NewFromFloat(0.50) // 100 * $0.005

	if !fee.Equal(expected) {
		t.Errorf("expected equity fee %s, got %s", expected, fee)
	}
}

// Test 6: Crypto fee calculation (0.1% of notional)
func TestFeeCalculation_Crypto(t *testing.T) {
	quantity := decimal.NewFromFloat(0.5)
	price := decimal.NewFromFloat(65000.0)

	fee := calculateFee(domain.AssetClassCrypto, quantity, price)
	// notional = 0.5 * 65000 = 32500; fee = 32500 * 0.001 = 32.50
	expected := decimal.NewFromFloat(32.50)

	if !fee.Equal(expected) {
		t.Errorf("expected crypto fee %s, got %s", expected, fee)
	}
}

// Test 7: All six instruments pre-loaded with correct asset classes
func TestAdapter_SixInstrumentsPreloaded(t *testing.T) {
	a := NewAdapter(nil)

	instruments, err := a.SupportedInstruments()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(instruments) != 6 {
		t.Fatalf("expected 6 instruments, got %d", len(instruments))
	}

	expectedInstruments := map[string]domain.AssetClass{
		"AAPL":    domain.AssetClassEquity,
		"MSFT":    domain.AssetClassEquity,
		"GOOG":    domain.AssetClassEquity,
		"BTC-USD": domain.AssetClassCrypto,
		"ETH-USD": domain.AssetClassCrypto,
		"SOL-USD": domain.AssetClassCrypto,
	}

	for _, inst := range instruments {
		expectedClass, ok := expectedInstruments[inst.Symbol]
		if !ok {
			t.Errorf("unexpected instrument: %s", inst.Symbol)
			continue
		}
		if inst.AssetClass != expectedClass {
			t.Errorf("instrument %s: expected asset class %d, got %d", inst.Symbol, expectedClass, inst.AssetClass)
		}
	}
}

// Test 8: FillFeed channel receives fills asynchronously
func TestAdapter_FillFeedAsync(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)
	err := a.Connect(context.Background(), domain.VenueCredential{})
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	defer a.Disconnect(context.Background())

	order := &domain.Order{
		ID:           "async-1",
		InstrumentID: "AAPL",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromInt(10),
		AssetClass:   domain.AssetClassEquity,
	}

	ack, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("submit order failed: %v", err)
	}
	if ack == nil || ack.VenueOrderID == "" {
		t.Fatal("expected non-empty venue order ID in ack")
	}

	select {
	case fill := <-a.FillFeed():
		if fill.OrderID != "async-1" {
			t.Errorf("expected order ID async-1, got %s", fill.OrderID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for fill on FillFeed channel")
	}
}

// Test 9: Adapter registration
func TestAdapterRegistration(t *testing.T) {
	factory, ok := adapter.Get("simulated")
	if !ok {
		t.Fatal("simulated adapter not registered")
	}
	provider := factory(nil)
	if provider.VenueID() != "sim-exchange" {
		t.Errorf("expected venue ID sim-exchange, got %s", provider.VenueID())
	}
	if provider.VenueName() != "Simulated Exchange" {
		t.Errorf("expected venue name Simulated Exchange, got %s", provider.VenueName())
	}
}

// Test 10: Sell limit order fills when price >= limit
func TestMatchingEngine_SellLimitFill(t *testing.T) {
	fillCh := make(chan domain.Fill, 100)
	engine := NewMatchingEngine(fillCh)
	engine.RegisterInstrument("MSFT", decimal.NewFromFloat(420.0), 0.30, 0.05, 100*time.Millisecond)

	// Sell limit at 410, price is 420 - should fill since 420 >= 410
	order := &domain.Order{
		ID:           "sell-limit-1",
		InstrumentID: "MSFT",
		Side:         domain.SideSell,
		Type:         domain.OrderTypeLimit,
		Price:        decimal.NewFromFloat(410.0),
		Quantity:     decimal.NewFromInt(25),
		AssetClass:   domain.AssetClassEquity,
	}

	engine.ProcessOrder(order)
	engine.sweepAll()

	select {
	case fill := <-fillCh:
		if fill.OrderID != "sell-limit-1" {
			t.Errorf("expected order ID sell-limit-1, got %s", fill.OrderID)
		}
		if !fill.Price.Equal(decimal.NewFromFloat(410.0)) {
			t.Errorf("expected fill at limit price 410, got %s", fill.Price)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for sell limit fill")
	}
}

// Test 11: Market order slippage is within 5bps (statistical)
func TestMatchingEngine_MarketOrderSlippageWithin5bps(t *testing.T) {
	fillCh := make(chan domain.Fill, 1000)
	engine := NewMatchingEngine(fillCh)
	engine.RegisterInstrument("BTC-USD", decimal.NewFromFloat(65000.0), 0.80, 0.0, 100*time.Millisecond)

	syntheticPrice := decimal.NewFromFloat(65000.0)

	for i := 0; i < 100; i++ {
		order := &domain.Order{
			ID:           domain.OrderID("mkt-" + string(rune('A'+i%26))),
			InstrumentID: "BTC-USD",
			Side:         domain.SideBuy,
			Type:         domain.OrderTypeMarket,
			Quantity:     decimal.NewFromFloat(0.1),
			AssetClass:   domain.AssetClassCrypto,
		}
		engine.ProcessOrder(order)
	}

	for i := 0; i < 100; i++ {
		select {
		case fill := <-fillCh:
			bps := slippageBps(syntheticPrice, fill.Price)
			if bps > 5.01 { // tiny tolerance for float
				t.Errorf("fill %d: slippage %f bps exceeds 5bps", i, bps)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out on fill %d", i)
		}
	}
}

// Test 12: Adapter status transitions
func TestAdapter_StatusTransitions(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)

	if a.Status() != adapter.Disconnected {
		t.Error("expected initial status Disconnected")
	}

	_ = a.Connect(context.Background(), domain.VenueCredential{})
	if a.Status() != adapter.Connected {
		t.Error("expected status Connected after Connect")
	}

	_ = a.Disconnect(context.Background())
	if a.Status() != adapter.Disconnected {
		t.Error("expected status Disconnected after Disconnect")
	}
}
