package router

import (
	"context"
	"log/slog"
	"sort"

	"github.com/synapse-oms/gateway/internal/domain"
)

// MLStrategy routes orders using the ML scoring sidecar.
// When the sidecar is unavailable or returns an error,
// it falls back to the provided fallback strategy.
type MLStrategy struct {
	scorer   *MLScorer
	fallback RoutingStrategy
}

// NewMLStrategy creates an MLStrategy with the given scorer and fallback.
func NewMLStrategy(scorer *MLScorer, fallback RoutingStrategy) *MLStrategy {
	return &MLStrategy{scorer: scorer, fallback: fallback}
}

// Name returns "ml-scored".
func (s *MLStrategy) Name() string { return "ml-scored" }

// Evaluate scores venues via the ML sidecar and returns allocations
// ranked by score. Falls back to the fallback strategy on any error.
func (s *MLStrategy) Evaluate(
	ctx context.Context,
	order *domain.Order,
	candidates []VenueCandidate,
) ([]VenueAllocation, error) {
	if len(candidates) == 0 {
		return nil, ErrNoCandidatesForStrategy
	}

	scores, err := s.scorer.ScoreVenues(ctx, order, candidates)
	if err != nil {
		slog.Warn("ml scorer unavailable, falling back",
			"strategy", s.fallback.Name(),
			"error", err,
		)
		return s.fallback.Evaluate(ctx, order, candidates)
	}

	if len(scores) == 0 {
		slog.Warn("ml scorer returned empty scores, falling back",
			"strategy", s.fallback.Name(),
		)
		return s.fallback.Evaluate(ctx, order, candidates)
	}

	// Sort scores by rank ascending (rank 1 = best).
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Rank < scores[j].Rank
	})

	// Build a depth map from candidates for allocation.
	depthMap := make(map[string]VenueCandidate, len(candidates))
	for _, c := range candidates {
		depthMap[c.VenueID] = c
	}

	// If best-scored venue covers the full order, single allocation.
	if best, ok := depthMap[scores[0].VenueID]; ok {
		if best.DepthAtPrice.GreaterThanOrEqual(order.Quantity) {
			return []VenueAllocation{
				{
					VenueID:  scores[0].VenueID,
					Quantity: order.Quantity,
					Reason:   "ml-scored: highest scoring venue",
				},
			}, nil
		}
	}

	// Return ranked allocations with depth for the splitter.
	allocs := make([]VenueAllocation, 0, len(scores))
	for _, sc := range scores {
		c, ok := depthMap[sc.VenueID]
		if !ok {
			continue
		}
		allocs = append(allocs, VenueAllocation{
			VenueID:  sc.VenueID,
			Quantity: c.DepthAtPrice,
			Reason:   "ml-scored: ranked by ML model",
		})
	}

	if len(allocs) == 0 {
		return s.fallback.Evaluate(ctx, order, candidates)
	}

	return allocs, nil
}
