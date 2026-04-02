package router

import (
	"context"
	"fmt"

	"github.com/synapse-oms/gateway/internal/domain"
)

// VenuePreferenceStrategy routes 100% of the order to the user's
// explicitly preferred venue (Order.VenueID). If the preferred venue
// is not among the available candidates, or if no preference is set,
// it delegates to a fallback strategy (typically BestPriceStrategy).
type VenuePreferenceStrategy struct {
	fallback RoutingStrategy
}

// NewVenuePreferenceStrategy creates a VenuePreferenceStrategy with the
// given fallback strategy used when the preferred venue is unavailable.
func NewVenuePreferenceStrategy(fallback RoutingStrategy) *VenuePreferenceStrategy {
	return &VenuePreferenceStrategy{fallback: fallback}
}

// Name returns "venue-preference".
func (s *VenuePreferenceStrategy) Name() string { return "venue-preference" }

// Evaluate routes the full order quantity to the preferred venue if it
// appears in the candidate list. Otherwise it delegates to the fallback.
func (s *VenuePreferenceStrategy) Evaluate(
	ctx context.Context,
	order *domain.Order,
	candidates []VenueCandidate,
) ([]VenueAllocation, error) {
	if len(candidates) == 0 {
		return nil, ErrNoCandidatesForStrategy
	}

	// If a preferred venue is set, look for it among candidates.
	if order.VenueID != "" {
		for _, c := range candidates {
			if c.VenueID == order.VenueID {
				return []VenueAllocation{
					{
						VenueID:  c.VenueID,
						Quantity: order.Quantity,
						Reason:   fmt.Sprintf("venue-preference: preferred venue %s", c.VenueID),
					},
				}, nil
			}
		}
	}

	// Preferred venue not found or not set — delegate to fallback.
	return s.fallback.Evaluate(ctx, order, candidates)
}
