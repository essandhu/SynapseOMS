package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
)

// mockMarketDataProvider is a test double for MarketDataProvider.
type mockMarketDataProvider struct {
	snapshots map[string]*adapter.MarketDataSnapshot // keyed by venueID
	errs      map[string]error
}

func (m *mockMarketDataProvider) GetSnapshot(venueID, instrument string) (*adapter.MarketDataSnapshot, error) {
	if err, ok := m.errs[venueID]; ok {
		return nil, err
	}
	snap, ok := m.snapshots[venueID]
	if !ok {
		return nil, errors.New("no snapshot")
	}
	return snap, nil
}

func newSnap(venueID string, bid, ask float64, ts time.Time) *adapter.MarketDataSnapshot {
	return &adapter.MarketDataSnapshot{
		InstrumentID: "BTC-USD",
		VenueID:      venueID,
		BidPrice:     decimal.NewFromFloat(bid),
		AskPrice:     decimal.NewFromFloat(ask),
		LastPrice:    decimal.NewFromFloat((bid + ask) / 2),
		Volume24h:    decimal.NewFromFloat(1000),
		Timestamp:    ts,
	}
}

func TestCompareVenuePrices_ThreeVenues_BuySide(t *testing.T) {
	now := time.Now()
	provider := &mockMarketDataProvider{
		snapshots: map[string]*adapter.MarketDataSnapshot{
			"venue-a": newSnap("venue-a", 100.0, 100.10, now), // spread: 10bps, ask 100.10
			"venue-b": newSnap("venue-b", 99.90, 100.20, now), // spread: ~30bps, ask 100.20
			"venue-c": newSnap("venue-c", 100.05, 100.05, now), // spread: 0bps, ask 100.05 (best)
		},
	}

	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	venueIDs := []string{"venue-a", "venue-b", "venue-c"}
	candidates, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", venueIDs, SideBuy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// For buy side, sorted by lowest ask price: venue-c (100.05), venue-a (100.10), venue-b (100.20)
	if candidates[0].VenueID != "venue-c" {
		t.Errorf("expected venue-c first (lowest ask), got %s", candidates[0].VenueID)
	}
	if candidates[1].VenueID != "venue-a" {
		t.Errorf("expected venue-a second, got %s", candidates[1].VenueID)
	}
	if candidates[2].VenueID != "venue-b" {
		t.Errorf("expected venue-b third, got %s", candidates[2].VenueID)
	}

	// Verify spread_bps for venue-c: ((100.05 - 100.05) / 100.05) * 10000 = 0
	if candidates[0].SpreadBps.IntPart() != 0 {
		t.Errorf("expected 0 spread_bps for venue-c, got %s", candidates[0].SpreadBps)
	}

	// Verify spread_bps for venue-a: ((100.10 - 100.0) / 100.05) * 10000 ~ 9.995
	spreadA := candidates[1].SpreadBps
	if spreadA.LessThan(decimal.NewFromFloat(9.0)) || spreadA.GreaterThan(decimal.NewFromFloat(11.0)) {
		t.Errorf("expected ~10 spread_bps for venue-a, got %s", spreadA)
	}

	// Verify cross-venue price diff for venue-a (buy side):
	// (venueAsk - bestAsk) / bestAsk * 10000 = (100.10 - 100.05) / 100.05 * 10000 ~ 4.998
	diffA := candidates[1].CrossVenueDiffBps
	if diffA.LessThan(decimal.NewFromFloat(4.0)) || diffA.GreaterThan(decimal.NewFromFloat(6.0)) {
		t.Errorf("expected ~5 cross-venue diff bps for venue-a, got %s", diffA)
	}

	// Best venue should have 0 cross-venue diff
	if !candidates[0].CrossVenueDiffBps.IsZero() {
		t.Errorf("expected 0 cross-venue diff for best venue, got %s", candidates[0].CrossVenueDiffBps)
	}
}

func TestCompareVenuePrices_ThreeVenues_SellSide(t *testing.T) {
	now := time.Now()
	provider := &mockMarketDataProvider{
		snapshots: map[string]*adapter.MarketDataSnapshot{
			"venue-a": newSnap("venue-a", 100.0, 100.10, now),  // bid 100.0
			"venue-b": newSnap("venue-b", 100.20, 100.30, now), // bid 100.20 (best)
			"venue-c": newSnap("venue-c", 99.90, 100.05, now),  // bid 99.90
		},
	}

	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	venueIDs := []string{"venue-a", "venue-b", "venue-c"}
	candidates, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", venueIDs, SideSell)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}

	// For sell side, sorted by highest bid: venue-b (100.20), venue-a (100.0), venue-c (99.90)
	if candidates[0].VenueID != "venue-b" {
		t.Errorf("expected venue-b first (highest bid), got %s", candidates[0].VenueID)
	}
	if candidates[1].VenueID != "venue-a" {
		t.Errorf("expected venue-a second, got %s", candidates[1].VenueID)
	}
	if candidates[2].VenueID != "venue-c" {
		t.Errorf("expected venue-c third, got %s", candidates[2].VenueID)
	}
}

func TestCompareVenuePrices_StaleVenueExcluded(t *testing.T) {
	now := time.Now()
	staleTime := now.Add(-10 * time.Second) // 10 seconds ago, well beyond 5s threshold

	provider := &mockMarketDataProvider{
		snapshots: map[string]*adapter.MarketDataSnapshot{
			"venue-fresh":  newSnap("venue-fresh", 100.0, 100.10, now),
			"venue-stale":  newSnap("venue-stale", 99.50, 99.60, staleTime),
			"venue-fresh2": newSnap("venue-fresh2", 100.05, 100.15, now),
		},
	}

	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	venueIDs := []string{"venue-fresh", "venue-stale", "venue-fresh2"}
	candidates, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", venueIDs, SideBuy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates (stale excluded), got %d", len(candidates))
	}

	for _, c := range candidates {
		if c.VenueID == "venue-stale" {
			t.Error("stale venue should have been excluded")
		}
	}
}

func TestCompareVenuePrices_DisconnectedVenueExcluded(t *testing.T) {
	now := time.Now()
	provider := &mockMarketDataProvider{
		snapshots: map[string]*adapter.MarketDataSnapshot{
			"venue-ok": newSnap("venue-ok", 100.0, 100.10, now),
		},
		errs: map[string]error{
			"venue-down": errors.New("venue disconnected"),
		},
	}

	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	venueIDs := []string{"venue-ok", "venue-down"}
	candidates, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", venueIDs, SideBuy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate (disconnected excluded), got %d", len(candidates))
	}
	if candidates[0].VenueID != "venue-ok" {
		t.Errorf("expected venue-ok, got %s", candidates[0].VenueID)
	}
}

func TestCompareVenuePrices_AllVenuesUnavailable(t *testing.T) {
	provider := &mockMarketDataProvider{
		errs: map[string]error{
			"venue-a": errors.New("down"),
			"venue-b": errors.New("down"),
		},
	}

	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	venueIDs := []string{"venue-a", "venue-b"}
	_, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", venueIDs, SideBuy)
	if err == nil {
		t.Fatal("expected error when all venues unavailable")
	}
	if !errors.Is(err, ErrNoAvailableVenues) {
		t.Errorf("expected ErrNoAvailableVenues, got %v", err)
	}
}

func TestCompareVenuePrices_EmptyVenueList(t *testing.T) {
	provider := &mockMarketDataProvider{}
	pc := NewPriceComparator(provider, WithStaleThreshold(5*time.Second))

	_, err := pc.CompareVenuePrices(context.Background(), "BTC-USD", nil, SideBuy)
	if err == nil {
		t.Fatal("expected error for empty venue list")
	}
}
