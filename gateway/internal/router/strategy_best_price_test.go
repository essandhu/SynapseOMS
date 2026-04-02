package router_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/router"
)

func newOrder(side domain.OrderSide, qty int64) *domain.Order {
	return &domain.Order{
		ID:       "ORD-BP-001",
		Side:     side,
		Type:     domain.OrderTypeLimit,
		Quantity: decimal.NewFromInt(qty),
	}
}

func TestBestPriceStrategy_Name(t *testing.T) {
	s := router.NewBestPriceStrategy()
	if s.Name() != "best-price" {
		t.Errorf("Name() = %q, want %q", s.Name(), "best-price")
	}
}

func TestBestPriceStrategy_BuySelectsLowestAsk(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 50)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "expensive",
			AskPrice:     decimal.NewFromFloat(101.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "cheap",
			AskPrice:     decimal.NewFromFloat(99.50),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   3 * time.Millisecond,
			FillRate30d:  0.85,
		},
		{
			VenueID:      "mid",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.92,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Best venue (cheap) has enough depth for full order -> single allocation
	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "cheap" {
		t.Errorf("VenueID = %q, want %q", allocs[0].VenueID, "cheap")
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(50)) {
		t.Errorf("Quantity = %s, want 50", allocs[0].Quantity)
	}
	if allocs[0].Reason != "best-price: lowest ask" {
		t.Errorf("Reason = %q, want %q", allocs[0].Reason, "best-price: lowest ask")
	}
}

func TestBestPriceStrategy_SellSelectsHighestBid(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideSell, 30)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "low-bid",
			BidPrice:     decimal.NewFromFloat(98.00),
			DepthAtPrice: decimal.NewFromInt(500),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "high-bid",
			BidPrice:     decimal.NewFromFloat(99.80),
			DepthAtPrice: decimal.NewFromInt(500),
			LatencyP50:   4 * time.Millisecond,
			FillRate30d:  0.88,
		},
		{
			VenueID:      "mid-bid",
			BidPrice:     decimal.NewFromFloat(99.00),
			DepthAtPrice: decimal.NewFromInt(500),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.95,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "high-bid" {
		t.Errorf("VenueID = %q, want %q", allocs[0].VenueID, "high-bid")
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(30)) {
		t.Errorf("Quantity = %s, want 30", allocs[0].Quantity)
	}
	if allocs[0].Reason != "best-price: highest bid" {
		t.Errorf("Reason = %q, want %q", allocs[0].Reason, "best-price: highest bid")
	}
}

func TestBestPriceStrategy_TieBreakByLatency(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 10)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "slow",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   10 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "fast",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.90,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "fast" {
		t.Errorf("VenueID = %q, want %q (lower latency should win tie-break)", allocs[0].VenueID, "fast")
	}
}

func TestBestPriceStrategy_TieBreakByFillRate(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 10)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "low-fill",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   5 * time.Millisecond,
			FillRate30d:  0.70,
		},
		{
			VenueID:      "high-fill",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   5 * time.Millisecond,
			FillRate30d:  0.95,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(allocs))
	}
	if allocs[0].VenueID != "high-fill" {
		t.Errorf("VenueID = %q, want %q (higher fill rate should win second tie-break)", allocs[0].VenueID, "high-fill")
	}
}

func TestBestPriceStrategy_SingleVenueCoversFullQty(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 100)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "deep",
			AskPrice:     decimal.NewFromFloat(99.00),
			DepthAtPrice: decimal.NewFromInt(100), // exactly covers order
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "shallow",
			AskPrice:     decimal.NewFromFloat(99.50),
			DepthAtPrice: decimal.NewFromInt(50),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.95,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 1 {
		t.Fatalf("allocations count = %d, want 1 (best venue covers full qty)", len(allocs))
	}
	if allocs[0].VenueID != "deep" {
		t.Errorf("VenueID = %q, want %q", allocs[0].VenueID, "deep")
	}
	if !allocs[0].Quantity.Equal(decimal.NewFromInt(100)) {
		t.Errorf("Quantity = %s, want 100", allocs[0].Quantity)
	}
}

func TestBestPriceStrategy_InsufficientDepth_ReturnsRankedList(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 500)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "worst-price",
			AskPrice:     decimal.NewFromFloat(101.00),
			DepthAtPrice: decimal.NewFromInt(200),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "best-price",
			AskPrice:     decimal.NewFromFloat(99.00),
			DepthAtPrice: decimal.NewFromInt(100), // not enough for 500
			LatencyP50:   3 * time.Millisecond,
			FillRate30d:  0.85,
		},
		{
			VenueID:      "mid-price",
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(150),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.92,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return all 3 ranked by ask price ascending
	if len(allocs) != 3 {
		t.Fatalf("allocations count = %d, want 3", len(allocs))
	}

	// Ranked: best-price (99), mid-price (100), worst-price (101)
	expected := []struct {
		venueID string
		qty     int64
	}{
		{"best-price", 100},
		{"mid-price", 150},
		{"worst-price", 200},
	}

	for i, exp := range expected {
		if allocs[i].VenueID != exp.venueID {
			t.Errorf("allocs[%d].VenueID = %q, want %q", i, allocs[i].VenueID, exp.venueID)
		}
		if !allocs[i].Quantity.Equal(decimal.NewFromInt(exp.qty)) {
			t.Errorf("allocs[%d].Quantity = %s, want %d", i, allocs[i].Quantity, exp.qty)
		}
	}
}

func TestBestPriceStrategy_SellInsufficientDepth_RankedByBidDesc(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideSell, 300)

	candidates := []router.VenueCandidate{
		{
			VenueID:      "low-bid",
			BidPrice:     decimal.NewFromFloat(97.00),
			DepthAtPrice: decimal.NewFromInt(80),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.90,
		},
		{
			VenueID:      "high-bid",
			BidPrice:     decimal.NewFromFloat(99.50),
			DepthAtPrice: decimal.NewFromInt(100),
			LatencyP50:   3 * time.Millisecond,
			FillRate30d:  0.85,
		},
		{
			VenueID:      "mid-bid",
			BidPrice:     decimal.NewFromFloat(98.00),
			DepthAtPrice: decimal.NewFromInt(120),
			LatencyP50:   1 * time.Millisecond,
			FillRate30d:  0.88,
		},
	}

	allocs, err := s.Evaluate(context.Background(), order, candidates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(allocs) != 3 {
		t.Fatalf("allocations count = %d, want 3", len(allocs))
	}

	// Ranked: high-bid (99.50), mid-bid (98.00), low-bid (97.00)
	expected := []struct {
		venueID string
		qty     int64
	}{
		{"high-bid", 100},
		{"mid-bid", 120},
		{"low-bid", 80},
	}

	for i, exp := range expected {
		if allocs[i].VenueID != exp.venueID {
			t.Errorf("allocs[%d].VenueID = %q, want %q", i, allocs[i].VenueID, exp.venueID)
		}
		if !allocs[i].Quantity.Equal(decimal.NewFromInt(exp.qty)) {
			t.Errorf("allocs[%d].Quantity = %s, want %d", i, allocs[i].Quantity, exp.qty)
		}
	}
}

func TestBestPriceStrategy_EmptyCandidates_ReturnsError(t *testing.T) {
	s := router.NewBestPriceStrategy()
	order := newOrder(domain.SideBuy, 100)

	_, err := s.Evaluate(context.Background(), order, nil)
	if err == nil {
		t.Fatal("expected error for empty candidates, got nil")
	}

	_, err = s.Evaluate(context.Background(), order, []router.VenueCandidate{})
	if err == nil {
		t.Fatal("expected error for empty candidates slice, got nil")
	}
}
