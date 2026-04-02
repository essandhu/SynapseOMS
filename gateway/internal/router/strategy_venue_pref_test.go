package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/router"
)

func newVenuePrefOrder(side domain.OrderSide, qty int64, venueID string) *domain.Order {
	return &domain.Order{
		ID:       "ORD-VP-001",
		Side:     side,
		Type:     domain.OrderTypeLimit,
		Quantity: decimal.NewFromInt(qty),
		VenueID:  venueID,
	}
}

func defaultCandidates() []router.VenueCandidate {
	return []router.VenueCandidate{
		{
			VenueID:      "venue-A",
			AskPrice:     decimal.NewFromFloat(101.00),
			BidPrice:     decimal.NewFromFloat(100.50),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "venue-B",
			AskPrice:     decimal.NewFromFloat(99.50),
			BidPrice:     decimal.NewFromFloat(99.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   3 * time.Millisecond,
			FillRate30d:  0.85,
		},
		{
			VenueID:      "venue-C",
			AskPrice:     decimal.NewFromFloat(100.00),
			BidPrice:     decimal.NewFromFloat(99.50),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.92,
		},
	}
}

func TestVenuePreferenceStrategy_Name(t *testing.T) {
	s := router.NewVenuePreferenceStrategy(router.NewBestPriceStrategy())
	if s.Name() != "venue-preference" {
		t.Errorf("Name() = %q, want %q", s.Name(), "venue-preference")
	}
}

func TestVenuePreferenceStrategy_PreferredVenueGetsFullAllocation(t *testing.T) {
	fallback := router.NewBestPriceStrategy()
	s := router.NewVenuePreferenceStrategy(fallback)

	order := newVenuePrefOrder(domain.SideBuy, 50, "venue-B")
	candidates := defaultCandidates()

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "venue-B" {
		t.Errorf("VenueID = %q, want %q", allocs[0].VenueID, "venue-B")
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(50)) {
		t.Errorf("Quantity = %s, want 50", allocs[0].Quantity)
	}
	if allocs[0].Reason != "venue-preference: preferred venue venue-B" {
		t.Errorf("Reason = %q, want %q", allocs[0].Reason, "venue-preference: preferred venue venue-B")
	}
}

func TestVenuePreferenceStrategy_UnavailablePreferredFallsToBestPrice(t *testing.T) {
	fallback := router.NewBestPriceStrategy()
	s := router.NewVenuePreferenceStrategy(fallback)

	// Preferred venue "venue-X" is not among the candidates
	order := newVenuePrefOrder(domain.SideBuy, 50, "venue-X")
	candidates := defaultCandidates()

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to best-price, which selects venue-B (lowest ask)
	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "venue-B" {
		t.Errorf("VenueID = %q, want %q (fallback to best-price should pick lowest ask)", allocs[0].VenueID, "venue-B")
	}
}

func TestVenuePreferenceStrategy_EmptyVenueIDFallsToBestPrice(t *testing.T) {
	fallback := router.NewBestPriceStrategy()
	s := router.NewVenuePreferenceStrategy(fallback)

	// No preferred venue set on order
	order := newVenuePrefOrder(domain.SideBuy, 50, "")
	candidates := defaultCandidates()

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to best-price
	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "venue-B" {
		t.Errorf("VenueID = %q, want %q (fallback to best-price)", allocs[0].VenueID, "venue-B")
	}
}

func TestVenuePreferenceStrategy_EmptyCandidates_ReturnsError(t *testing.T) {
	fallback := router.NewBestPriceStrategy()
	s := router.NewVenuePreferenceStrategy(fallback)

	order := newVenuePrefOrder(domain.SideBuy, 100, "venue-A")

	_, err := s.Evaluate(context.Background(), order, nil)
	if err == nil {
		t.Fatal("expected error for nil candidates, got nil")
	}

	_, err = s.Evaluate(context.Background(), order, []router.VenueCandidate{})
	if err == nil {
		t.Fatal("expected error for empty candidates slice, got nil")
	}
}

func TestVenuePreferenceStrategy_SellPreferredVenue(t *testing.T) {
	fallback := router.NewBestPriceStrategy()
	s := router.NewVenuePreferenceStrategy(fallback)

	order := newVenuePrefOrder(domain.SideSell, 30, "venue-A")
	candidates := defaultCandidates()

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "venue-A" {
		t.Errorf("VenueID = %q, want %q", allocs[0].VenueID, "venue-A")
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Quantity = %s, want 30", allocs[0].Quantity)
	}
}
