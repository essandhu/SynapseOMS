package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

func TestPositionPricer_OnMarketData_UpdatesExistingPosition(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()

	// Seed a position
	pos := &domain.Position{
		InstrumentID: "AAPL",
		VenueID:      "alpaca",
		Quantity:     decimal.NewFromInt(100),
		AverageCost:  decimal.NewFromFloat(150.0),
		AssetClass:   domain.AssetClassEquity,
	}
	if err := store.UpsertPosition(context.Background(), pos); err != nil {
		t.Fatal(err)
	}

	pricer := NewPositionPricer(store, notifier)
	snap := adapter.MarketDataSnapshot{
		InstrumentID: "AAPL",
		VenueID:      "alpaca",
		LastPrice:    decimal.NewFromFloat(160.0),
		Timestamp:    time.Now(),
	}

	pricer.OnMarketData(context.Background(), snap)

	// Verify persisted position was updated
	updated := store.getPosition("AAPL", "alpaca")
	if updated == nil {
		t.Fatal("expected position to exist after market data update")
	}
	if !updated.MarketPrice.Equal(decimal.NewFromFloat(160.0)) {
		t.Errorf("MarketPrice = %s, want 160", updated.MarketPrice)
	}
	// unrealized = (160 - 150) * 100 = 1000
	wantPnL := decimal.NewFromInt(1000)
	if !updated.UnrealizedPnL.Equal(wantPnL) {
		t.Errorf("UnrealizedPnL = %s, want %s", updated.UnrealizedPnL, wantPnL)
	}

	// Verify notifier was called
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.positionUpdates) != 1 {
		t.Fatalf("expected 1 position notification, got %d", len(notifier.positionUpdates))
	}
	if !notifier.positionUpdates[0].MarketPrice.Equal(decimal.NewFromFloat(160.0)) {
		t.Errorf("notified MarketPrice = %s, want 160", notifier.positionUpdates[0].MarketPrice)
	}
}

func TestPositionPricer_OnMarketData_NoPositionIsNoOp(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()

	pricer := NewPositionPricer(store, notifier)
	snap := adapter.MarketDataSnapshot{
		InstrumentID: "NONEXISTENT",
		VenueID:      "nowhere",
		LastPrice:    decimal.NewFromFloat(100.0),
		Timestamp:    time.Now(),
	}

	pricer.OnMarketData(context.Background(), snap)

	// Should not notify or error
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.positionUpdates) != 0 {
		t.Errorf("expected 0 notifications, got %d", len(notifier.positionUpdates))
	}
}

func TestPositionPricer_OnMarketData_ZeroPriceIsNoOp(t *testing.T) {
	store := newMemStore()
	notifier := newMockNotifier()

	pos := &domain.Position{
		InstrumentID: "AAPL",
		VenueID:      "alpaca",
		Quantity:     decimal.NewFromInt(100),
		AverageCost:  decimal.NewFromFloat(150.0),
	}
	if err := store.UpsertPosition(context.Background(), pos); err != nil {
		t.Fatal(err)
	}

	pricer := NewPositionPricer(store, notifier)
	snap := adapter.MarketDataSnapshot{
		InstrumentID: "AAPL",
		VenueID:      "alpaca",
		LastPrice:    decimal.Zero,
		Timestamp:    time.Now(),
	}

	pricer.OnMarketData(context.Background(), snap)

	// Should not update when price is zero
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	if len(notifier.positionUpdates) != 0 {
		t.Errorf("expected 0 notifications for zero price, got %d", len(notifier.positionUpdates))
	}
}
