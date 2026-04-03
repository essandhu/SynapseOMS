// Package marketdata provides OHLC bar aggregation from market data ticks.
package marketdata

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
)

// OHLCBar represents a single OHLC candlestick bar.
type OHLCBar struct {
	InstrumentID string
	VenueID      string
	Open         decimal.Decimal
	High         decimal.Decimal
	Low          decimal.Decimal
	Close        decimal.Decimal
	Volume       decimal.Decimal
	PeriodStart  time.Time
	PeriodEnd    time.Time
	TickCount    int
	Complete     bool
}

// Aggregator collects MarketDataSnapshot ticks and produces OHLC bars.
// Completed bars are sent to the output channel. Thread-safe.
type Aggregator struct {
	interval time.Duration
	out      chan<- OHLCBar

	mu   sync.RWMutex
	bars map[string]*OHLCBar // instrumentID -> current bar
}

// NewAggregator creates an aggregator that produces bars at the given interval.
// Completed and partial bars are sent to the out channel.
func NewAggregator(interval time.Duration, out chan<- OHLCBar) *Aggregator {
	return &Aggregator{
		interval: interval,
		out:      out,
		bars:     make(map[string]*OHLCBar),
	}
}

// Ingest processes a single market data tick, updating the current bar
// for the tick's instrument. If the tick falls in a new period, the
// previous bar is completed and emitted.
func (a *Aggregator) Ingest(snap adapter.MarketDataSnapshot) {
	periodStart := a.alignToPeriod(snap.Timestamp)

	a.mu.Lock()
	defer a.mu.Unlock()

	bar, exists := a.bars[snap.InstrumentID]

	if !exists {
		a.bars[snap.InstrumentID] = a.newBar(snap, periodStart)
		return
	}

	// Check if this tick belongs to a new period
	if !periodStart.Equal(bar.PeriodStart) {
		bar.Complete = true
		bar.PeriodEnd = bar.PeriodStart.Add(a.interval)
		a.emit(*bar)
		a.bars[snap.InstrumentID] = a.newBar(snap, periodStart)
		return
	}

	// Update current bar
	if snap.LastPrice.GreaterThan(bar.High) {
		bar.High = snap.LastPrice
	}
	if snap.LastPrice.LessThan(bar.Low) {
		bar.Low = snap.LastPrice
	}
	bar.Close = snap.LastPrice
	bar.TickCount++
}

// CurrentBar returns a copy of the current (incomplete) bar for an instrument.
// Returns nil if no bar exists.
func (a *Aggregator) CurrentBar(instrumentID string) *OHLCBar {
	a.mu.RLock()
	defer a.mu.RUnlock()

	bar, exists := a.bars[instrumentID]
	if !exists {
		return nil
	}
	copy := *bar
	return &copy
}

// Flush emits all current (partial) bars to the output channel.
// Used for periodic real-time updates to the frontend.
func (a *Aggregator) Flush() {
	a.mu.RLock()
	defer a.mu.RUnlock()

	for _, bar := range a.bars {
		partial := *bar
		partial.Complete = false
		partial.PeriodEnd = bar.PeriodStart.Add(a.interval)
		a.emit(partial)
	}
}

func (a *Aggregator) newBar(snap adapter.MarketDataSnapshot, periodStart time.Time) *OHLCBar {
	return &OHLCBar{
		InstrumentID: snap.InstrumentID,
		VenueID:      snap.VenueID,
		Open:         snap.LastPrice,
		High:         snap.LastPrice,
		Low:          snap.LastPrice,
		Close:        snap.LastPrice,
		Volume:       decimal.Zero,
		PeriodStart:  periodStart,
		TickCount:    1,
	}
}

func (a *Aggregator) alignToPeriod(ts time.Time) time.Time {
	secs := int64(a.interval.Seconds())
	unix := ts.Unix()
	aligned := unix - (unix % secs)
	return time.Unix(aligned, 0).UTC()
}

func (a *Aggregator) emit(bar OHLCBar) {
	select {
	case a.out <- bar:
	default:
		// Drop bar if channel full — don't block ingestion
	}
}
