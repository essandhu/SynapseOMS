package router

import (
	"context"
	"errors"
	"sort"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// ErrNoCandidatesForStrategy is returned when Evaluate receives an empty candidate list.
var ErrNoCandidatesForStrategy = errors.New("no venue candidates provided")

// BestPriceStrategy is the cold-start / default routing strategy.
// For BUY orders it ranks venues by lowest ask price; for SELL orders
// by highest bid price. Tie-breaking: lower latency wins, then higher
// fill rate.
type BestPriceStrategy struct{}

// NewBestPriceStrategy creates a new BestPriceStrategy.
func NewBestPriceStrategy() *BestPriceStrategy {
	return &BestPriceStrategy{}
}

// Name returns "best-price".
func (s *BestPriceStrategy) Name() string { return "best-price" }

// Evaluate ranks candidates by price and returns allocations.
//
// If the best venue's depth covers the full order quantity, a single
// allocation with Quantity = order.Quantity is returned.
//
// If depth is insufficient, all candidates are returned ranked by price
// (best to worst), each with Quantity = their DepthAtPrice. The splitter
// (P3-04) will handle proportional allocation later.
func (s *BestPriceStrategy) Evaluate(
	_ context.Context,
	order *domain.Order,
	candidates []VenueCandidate,
) ([]VenueAllocation, error) {
	if len(candidates) == 0 {
		return nil, ErrNoCandidatesForStrategy
	}

	isBuy := order.Side == domain.SideBuy

	// Sort candidates by price with tie-breaking.
	sorted := make([]VenueCandidate, len(candidates))
	copy(sorted, candidates)

	sort.SliceStable(sorted, func(i, j int) bool {
		var priceI, priceJ decimal.Decimal
		if isBuy {
			priceI = sorted[i].AskPrice
			priceJ = sorted[j].AskPrice
		} else {
			priceI = sorted[i].BidPrice
			priceJ = sorted[j].BidPrice
		}

		cmp := priceI.Cmp(priceJ)
		if isBuy {
			// Ascending: lower ask is better.
			if cmp != 0 {
				return cmp < 0
			}
		} else {
			// Descending: higher bid is better.
			if cmp != 0 {
				return cmp > 0
			}
		}

		// Tie-break 1: lower latency wins.
		if sorted[i].LatencyP50 != sorted[j].LatencyP50 {
			return sorted[i].LatencyP50 < sorted[j].LatencyP50
		}

		// Tie-break 2: higher fill rate wins.
		return sorted[i].FillRate30d > sorted[j].FillRate30d
	})

	// Determine reason string.
	reason := "best-price: highest bid"
	if isBuy {
		reason = "best-price: lowest ask"
	}

	// If best venue covers the full order, return a single allocation.
	if sorted[0].DepthAtPrice.GreaterThanOrEqual(order.Quantity) {
		return []VenueAllocation{
			{
				VenueID:  sorted[0].VenueID,
				Quantity: order.Quantity,
				Reason:   reason,
			},
		}, nil
	}

	// Depth insufficient at best venue — return all ranked with their depth.
	allocs := make([]VenueAllocation, len(sorted))
	for i, c := range sorted {
		allocs[i] = VenueAllocation{
			VenueID:  c.VenueID,
			Quantity: c.DepthAtPrice,
			Reason:   reason,
		}
	}

	return allocs, nil
}
