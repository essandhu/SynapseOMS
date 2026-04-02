package router

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// splitThreshold is the fraction of best venue's depth above which splitting is triggered.
// Per architecture Section 5.1: order qty > 50% of displayed quantity at price.
var splitThreshold = decimal.NewFromFloat(0.5)

// SplitOrder splits a large order across top-N venues proportionally to their
// available depth. Splitting triggers when order qty > 50% of the best venue's
// displayed quantity at price.
//
// Parameters:
//   - orderQty:    total quantity to fill
//   - rankedVenues: venues in priority order (index 0 = best)
//   - depthMap:     venue ID → available depth at price
//   - lotSize:      minimum child order size (instrument lot size)
//
// Returns a slice of VenueAllocation. Residual quantity after lot-size rounding
// is assigned to the best-priced venue.
func SplitOrder(
	orderQty decimal.Decimal,
	rankedVenues []VenueAllocation,
	depthMap map[string]decimal.Decimal,
	lotSize decimal.Decimal,
) []VenueAllocation {
	if len(rankedVenues) == 0 {
		return nil
	}

	bestVenue := rankedVenues[0]

	// Single venue or no split needed: return full allocation to best venue.
	if len(rankedVenues) == 1 {
		return []VenueAllocation{{
			VenueID:  bestVenue.VenueID,
			Quantity: orderQty,
			Reason:   "single-venue",
		}}
	}

	bestDepth := depthMap[bestVenue.VenueID]
	threshold := bestDepth.Mul(splitThreshold)

	// If order qty <= 50% of best venue's depth, no split needed.
	if orderQty.LessThanOrEqual(threshold) {
		return []VenueAllocation{{
			VenueID:  bestVenue.VenueID,
			Quantity: orderQty,
			Reason:   "no-split: within depth threshold",
		}}
	}

	// Calculate total depth across all ranked venues.
	totalDepth := decimal.Zero
	for _, v := range rankedVenues {
		if d, ok := depthMap[v.VenueID]; ok {
			totalDepth = totalDepth.Add(d)
		}
	}

	// Proportional allocation, rounded down to lot size.
	allocations := make([]VenueAllocation, 0, len(rankedVenues))
	allocated := decimal.Zero

	for _, v := range rankedVenues {
		depth := depthMap[v.VenueID]
		// proportion = depth / totalDepth * orderQty
		raw := depth.Div(totalDepth).Mul(orderQty)

		// Round down to lot size: floor(raw / lotSize) * lotSize
		lots := raw.Div(lotSize).Floor()
		qty := lots.Mul(lotSize)

		// Skip sub-lot allocations (minimum child order = 1 lot).
		if qty.LessThan(lotSize) {
			continue
		}

		allocations = append(allocations, VenueAllocation{
			VenueID:  v.VenueID,
			Quantity: qty,
			Reason:   fmt.Sprintf("split: %.1f%% depth proportion", depth.Div(totalDepth).Mul(decimal.NewFromInt(100)).InexactFloat64()),
		})
		allocated = allocated.Add(qty)
	}

	// Assign residual to best venue.
	residual := orderQty.Sub(allocated)
	if residual.GreaterThan(decimal.Zero) {
		if len(allocations) > 0 && allocations[0].VenueID == bestVenue.VenueID {
			allocations[0].Quantity = allocations[0].Quantity.Add(residual)
		} else {
			// Best venue was dropped (sub-lot); add it back with residual.
			allocations = append([]VenueAllocation{{
				VenueID:  bestVenue.VenueID,
				Quantity: residual,
				Reason:   "split: residual to best venue",
			}}, allocations...)
		}
	}

	return allocations
}
