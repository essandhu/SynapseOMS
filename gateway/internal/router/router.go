package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/synapse-oms/gateway/internal/domain"
)

var (
	// ErrNoStrategies is returned when no strategies are registered.
	ErrNoStrategies = errors.New("router: no strategies registered")

	// ErrStrategyNotFound is returned when the requested strategy is not registered
	// and no default strategy is available.
	ErrStrategyNotFound = errors.New("router: strategy not found")

	// ErrNoCandidates is returned when Route is called with an empty candidate list.
	ErrNoCandidates = errors.New("router: no venue candidates provided")
)

const DefaultStrategyKey = "default"

// Router selects which venue(s) receive an order by delegating to a
// named RoutingStrategy. Strategies are registered by name and can be
// swapped at runtime (hot-swap). If an order specifies a strategy name,
// that strategy is used; otherwise the "default" strategy is used.
type Router struct {
	mu         sync.RWMutex
	strategies map[string]RoutingStrategy
}

// New creates a Router with no strategies registered.
func New() *Router {
	return &Router{
		strategies: make(map[string]RoutingStrategy),
	}
}

// Register adds or replaces a strategy by name. The first strategy
// registered is also set as the default if no default exists yet.
func (r *Router) Register(s RoutingStrategy) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.strategies[s.Name()] = s

	if _, hasDefault := r.strategies[DefaultStrategyKey]; !hasDefault {
		r.strategies[DefaultStrategyKey] = s
	}
}

// SetDefault designates an already-registered strategy as the default.
func (r *Router) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.strategies[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrStrategyNotFound, name)
	}
	r.strategies[DefaultStrategyKey] = s
	return nil
}

// Route selects a strategy and evaluates the order against candidates.
// The strategyName parameter selects the strategy; if empty, the default is used.
func (r *Router) Route(
	ctx context.Context,
	order *domain.Order,
	candidates []VenueCandidate,
	strategyName string,
) (*RoutingDecision, error) {
	if len(candidates) == 0 {
		return nil, ErrNoCandidates
	}

	strategy, err := r.resolve(strategyName)
	if err != nil {
		return nil, err
	}

	allocations, err := strategy.Evaluate(ctx, order, candidates)
	if err != nil {
		return nil, fmt.Errorf("router: strategy %q failed: %w", strategy.Name(), err)
	}

	return &RoutingDecision{
		OrderID:     order.ID,
		Allocations: allocations,
		Strategy:    strategy.Name(),
		Timestamp:   time.Now(),
	}, nil
}

// resolve picks the strategy to use: named strategy if found, then default,
// then error.
func (r *Router) resolve(name string) (RoutingStrategy, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.strategies) == 0 {
		return nil, ErrNoStrategies
	}

	// If a specific strategy was requested, look it up first.
	if name != "" {
		if s, ok := r.strategies[name]; ok {
			return s, nil
		}
	}

	// Fall back to default.
	if s, ok := r.strategies[DefaultStrategyKey]; ok {
		return s, nil
	}

	return nil, fmt.Errorf("%w: %s", ErrStrategyNotFound, name)
}

// Strategies returns the names of all registered strategies (excluding the
// internal "default" alias).
func (r *Router) Strategies() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.strategies))
	for name := range r.strategies {
		if name == DefaultStrategyKey {
			continue
		}
		names = append(names, name)
	}
	return names
}
