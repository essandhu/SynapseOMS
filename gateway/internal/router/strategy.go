package router

import (
	"context"

	"github.com/synapse-oms/gateway/internal/domain"
)

// RoutingStrategy evaluates an order against candidate venues and returns
// one or more allocations. Strategies are registered by name for hot-swapping.
type RoutingStrategy interface {
	// Name returns the unique identifier for this strategy.
	Name() string

	// Evaluate selects venues and quantities for the given order.
	// It must return at least one allocation on success.
	Evaluate(ctx context.Context, order *domain.Order, candidates []VenueCandidate) ([]VenueAllocation, error)
}
