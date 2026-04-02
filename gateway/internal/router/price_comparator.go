package router

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
)

// OrderSide indicates whether we are comparing prices for a buy or sell.
type OrderSide int

const (
	SideBuy OrderSide = iota
	SideSell
)

var (
	// ErrNoAvailableVenues is returned when all venues are disconnected or have stale data.
	ErrNoAvailableVenues = errors.New("price_comparator: no available venues with fresh data")

	// ErrNoVenuesProvided is returned when the venue list is empty.
	ErrNoVenuesProvided = errors.New("price_comparator: no venues provided")
)

const defaultStaleThreshold = 5 * time.Second

var bpsFactor = decimal.NewFromInt(10000)

// MarketDataProvider abstracts market data retrieval for testability.
// Implementations may read from a live venue adapter, a Redis cache, or
// an in-memory snapshot store.
type MarketDataProvider interface {
	GetSnapshot(venueID, instrument string) (*adapter.MarketDataSnapshot, error)
}

// PricedVenueCandidate extends VenueCandidate with price-comparison metrics.
type PricedVenueCandidate struct {
	VenueCandidate

	SpreadBps         decimal.Decimal // ((ask - bid) / mid) * 10000
	CrossVenueDiffBps decimal.Decimal // divergence from best price, in bps
}

// PriceComparator queries market data from multiple venues and produces
// a ranked list of candidates with spread and cross-venue divergence metrics.
type PriceComparator struct {
	provider       MarketDataProvider
	staleThreshold time.Duration
	nowFunc        func() time.Time // injectable clock for testing
}

// Option configures a PriceComparator.
type Option func(*PriceComparator)

// WithStaleThreshold sets how old a snapshot can be before it is excluded.
func WithStaleThreshold(d time.Duration) Option {
	return func(pc *PriceComparator) {
		pc.staleThreshold = d
	}
}

// WithClock overrides the time source (useful for deterministic tests).
func WithClock(fn func() time.Time) Option {
	return func(pc *PriceComparator) {
		pc.nowFunc = fn
	}
}

// NewPriceComparator creates a PriceComparator with the given data provider.
func NewPriceComparator(provider MarketDataProvider, opts ...Option) *PriceComparator {
	pc := &PriceComparator{
		provider:       provider,
		staleThreshold: defaultStaleThreshold,
		nowFunc:        time.Now,
	}
	for _, o := range opts {
		o(pc)
	}
	return pc
}

// CompareVenuePrices fetches the current BBO from each venue, computes
// spread and cross-venue divergence in basis points, and returns candidates
// sorted by effective price (best first for the given side).
//
// Venues that return errors or have stale data (older than the stale
// threshold) are silently excluded. If no venues remain, ErrNoAvailableVenues
// is returned.
func (pc *PriceComparator) CompareVenuePrices(
	ctx context.Context,
	instrument string,
	venueIDs []string,
	side OrderSide,
) ([]PricedVenueCandidate, error) {
	if len(venueIDs) == 0 {
		return nil, ErrNoVenuesProvided
	}

	now := pc.nowFunc()
	cutoff := now.Add(-pc.staleThreshold)

	// Collect fresh snapshots.
	type venueSnap struct {
		id   string
		snap *adapter.MarketDataSnapshot
	}
	var fresh []venueSnap

	for _, vid := range venueIDs {
		snap, err := pc.provider.GetSnapshot(vid, instrument)
		if err != nil {
			continue // disconnected or unavailable
		}
		if snap.Timestamp.Before(cutoff) {
			continue // stale data
		}
		fresh = append(fresh, venueSnap{id: vid, snap: snap})
	}

	if len(fresh) == 0 {
		return nil, fmt.Errorf("%w: instrument=%s", ErrNoAvailableVenues, instrument)
	}

	// Find best price across all venues.
	var bestPrice decimal.Decimal
	for i, vs := range fresh {
		var price decimal.Decimal
		if side == SideBuy {
			price = vs.snap.AskPrice
		} else {
			price = vs.snap.BidPrice
		}
		if i == 0 {
			bestPrice = price
		} else if side == SideBuy && price.LessThan(bestPrice) {
			bestPrice = price
		} else if side == SideSell && price.GreaterThan(bestPrice) {
			bestPrice = price
		}
	}

	// Build candidates with metrics.
	candidates := make([]PricedVenueCandidate, 0, len(fresh))
	for _, vs := range fresh {
		snap := vs.snap

		// Spread in bps: ((ask - bid) / mid) * 10000
		mid := snap.BidPrice.Add(snap.AskPrice).Div(decimal.NewFromInt(2))
		var spreadBps decimal.Decimal
		if mid.IsPositive() {
			spreadBps = snap.AskPrice.Sub(snap.BidPrice).Div(mid).Mul(bpsFactor)
		}

		// Cross-venue price divergence in bps.
		var diffBps decimal.Decimal
		if bestPrice.IsPositive() {
			if side == SideBuy {
				// Higher ask = worse for buyer.
				diffBps = snap.AskPrice.Sub(bestPrice).Div(bestPrice).Mul(bpsFactor)
			} else {
				// Lower bid = worse for seller (best - venue) / best.
				diffBps = bestPrice.Sub(snap.BidPrice).Div(bestPrice).Mul(bpsFactor)
			}
		}

		candidates = append(candidates, PricedVenueCandidate{
			VenueCandidate: VenueCandidate{
				VenueID:  vs.id,
				BidPrice: snap.BidPrice,
				AskPrice: snap.AskPrice,
			},
			SpreadBps:         spreadBps,
			CrossVenueDiffBps: diffBps,
		})
	}

	// Sort by effective price (best first).
	sort.SliceStable(candidates, func(i, j int) bool {
		if side == SideBuy {
			return candidates[i].AskPrice.LessThan(candidates[j].AskPrice)
		}
		return candidates[i].BidPrice.GreaterThan(candidates[j].BidPrice)
	})

	return candidates, nil
}
