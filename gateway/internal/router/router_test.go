package router_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/router"
)

// --- mock strategy ---

type mockStrategy struct {
	name        string
	allocations []router.VenueAllocation
	err         error
}

func (m *mockStrategy) Name() string { return m.name }

func (m *mockStrategy) Evaluate(
	_ context.Context,
	_ *domain.Order,
	_ []router.VenueCandidate,
) ([]router.VenueAllocation, error) {
	return m.allocations, m.err
}

// --- helpers ---

func sampleOrder() *domain.Order {
	return &domain.Order{
		ID:       "ORD-001",
		Side:     domain.SideBuy,
		Type:     domain.OrderTypeLimit,
		Quantity: decimal.NewFromInt(100),
	}
}

func sampleCandidates() []router.VenueCandidate {
	return []router.VenueCandidate{
		{
			VenueID:      "venue-A",
			BidPrice:     decimal.NewFromFloat(99.50),
			AskPrice:     decimal.NewFromFloat(100.00),
			DepthAtPrice: decimal.NewFromInt(500),
			LatencyP50:   2 * time.Millisecond,
			FillRate30d:  0.95,
			FeeRate:      decimal.NewFromFloat(0.001),
		},
		{
			VenueID:      "venue-B",
			BidPrice:     decimal.NewFromFloat(99.60),
			AskPrice:     decimal.NewFromFloat(99.90),
			DepthAtPrice: decimal.NewFromInt(300),
			LatencyP50:   5 * time.Millisecond,
			FillRate30d:  0.88,
			FeeRate:      decimal.NewFromFloat(0.0008),
		},
	}
}

// --- tests ---

func TestRoute_WithMockStrategy_ReturnsExpectedAllocations(t *testing.T) {
	r := router.New()

	expected := []router.VenueAllocation{
		{VenueID: "venue-A", Quantity: decimal.NewFromInt(60), Reason: "best price"},
		{VenueID: "venue-B", Quantity: decimal.NewFromInt(40), Reason: "remainder"},
	}
	r.Register(&mockStrategy{name: "best-price", allocations: expected})

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "best-price")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.OrderID != "ORD-001" {
		t.Errorf("order ID = %q, want %q", decision.OrderID, "ORD-001")
	}
	if decision.Strategy != "best-price" {
		t.Errorf("strategy = %q, want %q", decision.Strategy, "best-price")
	}
	if decision.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
	if len(decision.Allocations) != 2 {
		t.Fatalf("allocations count = %d, want 2", len(decision.Allocations))
	}
	if decision.Allocations[0].VenueID != "venue-A" {
		t.Errorf("first allocation venue = %q, want %q", decision.Allocations[0].VenueID, "venue-A")
	}
	if !decision.Allocations[0].Quantity.Equal(decimal.NewFromInt(60)) {
		t.Errorf("first allocation qty = %s, want 60", decision.Allocations[0].Quantity)
	}
}

func TestRoute_UnknownStrategy_FallsBackToDefault(t *testing.T) {
	r := router.New()

	defaultAlloc := []router.VenueAllocation{
		{VenueID: "venue-A", Quantity: decimal.NewFromInt(100), Reason: "default fallback"},
	}
	r.Register(&mockStrategy{name: "best-price", allocations: defaultAlloc})
	// "best-price" is auto-set as default since it was first registered.

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "nonexistent-strategy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if decision.Strategy != "best-price" {
		t.Errorf("strategy = %q, want default fallback to %q", decision.Strategy, "best-price")
	}
	if len(decision.Allocations) != 1 {
		t.Fatalf("allocations count = %d, want 1", len(decision.Allocations))
	}
	if decision.Allocations[0].Reason != "default fallback" {
		t.Errorf("reason = %q, want %q", decision.Allocations[0].Reason, "default fallback")
	}
}

func TestRoute_EmptyStrategyName_UsesDefault(t *testing.T) {
	r := router.New()

	alloc := []router.VenueAllocation{
		{VenueID: "venue-B", Quantity: decimal.NewFromInt(100), Reason: "default"},
	}
	r.Register(&mockStrategy{name: "ml-scorer", allocations: alloc})

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Strategy != "ml-scorer" {
		t.Errorf("strategy = %q, want %q", decision.Strategy, "ml-scorer")
	}
}

func TestRoute_NoCandidates_ReturnsError(t *testing.T) {
	r := router.New()
	r.Register(&mockStrategy{name: "x", allocations: nil})

	_, err := r.Route(context.Background(), sampleOrder(), nil, "")
	if !errors.Is(err, router.ErrNoCandidates) {
		t.Errorf("err = %v, want ErrNoCandidates", err)
	}
}

func TestRoute_NoStrategies_ReturnsError(t *testing.T) {
	r := router.New()

	_, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "anything")
	if !errors.Is(err, router.ErrNoStrategies) {
		t.Errorf("err = %v, want ErrNoStrategies", err)
	}
}

func TestRoute_StrategyError_PropagatesError(t *testing.T) {
	r := router.New()

	stratErr := errors.New("venue data stale")
	r.Register(&mockStrategy{name: "broken", err: stratErr})

	_, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "broken")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, stratErr) {
		t.Errorf("err = %v, want wrapped %v", err, stratErr)
	}
}

func TestRegister_FirstStrategy_BecomesDefault(t *testing.T) {
	r := router.New()

	r.Register(&mockStrategy{name: "alpha"})
	r.Register(&mockStrategy{name: "beta"})

	// With empty strategy name, default (alpha) should be used.
	alloc := []router.VenueAllocation{
		{VenueID: "v", Quantity: decimal.NewFromInt(1), Reason: "test"},
	}
	r.Register(&mockStrategy{name: "alpha", allocations: alloc})

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Strategy != "alpha" {
		t.Errorf("strategy = %q, want %q", decision.Strategy, "alpha")
	}
}

func TestSetDefault_SwitchesDefaultStrategy(t *testing.T) {
	r := router.New()

	allocA := []router.VenueAllocation{{VenueID: "v", Quantity: decimal.NewFromInt(1), Reason: "A"}}
	allocB := []router.VenueAllocation{{VenueID: "v", Quantity: decimal.NewFromInt(1), Reason: "B"}}

	r.Register(&mockStrategy{name: "alpha", allocations: allocA})
	r.Register(&mockStrategy{name: "beta", allocations: allocB})

	if err := r.SetDefault("beta"); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Strategy != "beta" {
		t.Errorf("strategy = %q, want %q", decision.Strategy, "beta")
	}
}

func TestSetDefault_UnknownStrategy_ReturnsError(t *testing.T) {
	r := router.New()
	err := r.SetDefault("nonexistent")
	if !errors.Is(err, router.ErrStrategyNotFound) {
		t.Errorf("err = %v, want ErrStrategyNotFound", err)
	}
}

func TestStrategies_ReturnsRegisteredNames(t *testing.T) {
	r := router.New()
	r.Register(&mockStrategy{name: "alpha"})
	r.Register(&mockStrategy{name: "beta"})

	names := r.Strategies()
	if len(names) != 2 {
		t.Fatalf("strategies count = %d, want 2", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("strategies = %v, want alpha and beta", names)
	}
}

func TestRoute_SelectsNamedStrategyOverDefault(t *testing.T) {
	r := router.New()

	allocDefault := []router.VenueAllocation{{VenueID: "v", Quantity: decimal.NewFromInt(1), Reason: "default"}}
	allocSpecific := []router.VenueAllocation{{VenueID: "v", Quantity: decimal.NewFromInt(1), Reason: "specific"}}

	r.Register(&mockStrategy{name: "default-strat", allocations: allocDefault})
	r.Register(&mockStrategy{name: "ml-scorer", allocations: allocSpecific})

	decision, err := r.Route(context.Background(), sampleOrder(), sampleCandidates(), "ml-scorer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Strategy != "ml-scorer" {
		t.Errorf("strategy = %q, want %q", decision.Strategy, "ml-scorer")
	}
	if decision.Allocations[0].Reason != "specific" {
		t.Errorf("reason = %q, want %q", decision.Allocations[0].Reason, "specific")
	}
}
