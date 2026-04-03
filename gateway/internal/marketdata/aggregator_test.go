package marketdata

import (
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
)

func tick(instrumentID string, price float64, ts time.Time) adapter.MarketDataSnapshot {
	p := decimal.NewFromFloat(price)
	return adapter.MarketDataSnapshot{
		InstrumentID: instrumentID,
		VenueID:      "test-venue",
		LastPrice:    p,
		BidPrice:     p.Sub(decimal.NewFromFloat(0.01)),
		AskPrice:     p.Add(decimal.NewFromFloat(0.01)),
		Timestamp:    ts,
	}
}

func TestSingleTickOpensNewBar(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(time.Minute, out)

	ts := time.Date(2026, 4, 2, 10, 0, 30, 0, time.UTC) // 30s into the minute
	agg.Ingest(tick("AAPL", 150.0, ts))

	bar := agg.CurrentBar("AAPL")
	if bar == nil {
		t.Fatal("expected a current bar for AAPL")
	}
	if !bar.Open.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("open = %s, want 150", bar.Open)
	}
	if !bar.High.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("high = %s, want 150", bar.High)
	}
	if !bar.Low.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("low = %s, want 150", bar.Low)
	}
	if !bar.Close.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("close = %s, want 150", bar.Close)
	}
}

func TestMultipleTicksUpdateHighLowCloseVolume(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(time.Minute, out)

	base := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	agg.Ingest(tick("AAPL", 150.0, base))
	agg.Ingest(tick("AAPL", 152.0, base.Add(10*time.Second)))
	agg.Ingest(tick("AAPL", 148.0, base.Add(20*time.Second)))
	agg.Ingest(tick("AAPL", 151.0, base.Add(30*time.Second)))

	bar := agg.CurrentBar("AAPL")
	if bar == nil {
		t.Fatal("expected current bar")
	}
	if !bar.Open.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("open = %s, want 150", bar.Open)
	}
	if !bar.High.Equal(decimal.NewFromFloat(152.0)) {
		t.Errorf("high = %s, want 152", bar.High)
	}
	if !bar.Low.Equal(decimal.NewFromFloat(148.0)) {
		t.Errorf("low = %s, want 148", bar.Low)
	}
	if !bar.Close.Equal(decimal.NewFromFloat(151.0)) {
		t.Errorf("close = %s, want 151", bar.Close)
	}
	if bar.TickCount != 4 {
		t.Errorf("tick count = %d, want 4", bar.TickCount)
	}
}

func TestBarRolloverAtPeriodBoundary(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(time.Minute, out)

	// Tick in minute 10:00
	base := time.Date(2026, 4, 2, 10, 0, 30, 0, time.UTC)
	agg.Ingest(tick("AAPL", 150.0, base))

	// Tick in minute 10:01 — should trigger rollover
	next := time.Date(2026, 4, 2, 10, 1, 5, 0, time.UTC)
	agg.Ingest(tick("AAPL", 155.0, next))

	// The completed bar should be emitted to the output channel
	select {
	case completed := <-out:
		if !completed.Open.Equal(decimal.NewFromFloat(150.0)) {
			t.Errorf("completed bar open = %s, want 150", completed.Open)
		}
		if !completed.Complete {
			t.Error("completed bar should have Complete=true")
		}
		if completed.InstrumentID != "AAPL" {
			t.Errorf("instrument = %s, want AAPL", completed.InstrumentID)
		}
	default:
		t.Fatal("expected a completed bar on the output channel")
	}

	// Current bar should be the new period with price 155
	bar := agg.CurrentBar("AAPL")
	if bar == nil {
		t.Fatal("expected new current bar")
	}
	if !bar.Open.Equal(decimal.NewFromFloat(155.0)) {
		t.Errorf("new bar open = %s, want 155", bar.Open)
	}
}

func TestFlushEmitsPartialBars(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(time.Minute, out)

	base := time.Date(2026, 4, 2, 10, 0, 15, 0, time.UTC)
	agg.Ingest(tick("AAPL", 150.0, base))
	agg.Ingest(tick("BTC-USD", 60000.0, base))

	agg.Flush()

	// Should get two partial bars
	bars := drainChannel(out, 2)
	if len(bars) != 2 {
		t.Fatalf("expected 2 partial bars, got %d", len(bars))
	}

	for _, bar := range bars {
		if bar.Complete {
			t.Errorf("flushed bar for %s should have Complete=false", bar.InstrumentID)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	out := make(chan OHLCBar, 1000)
	agg := NewAggregator(time.Minute, out)

	base := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ts := base.Add(time.Duration(i) * time.Millisecond)
			agg.Ingest(tick("AAPL", 150.0+float64(i)*0.01, ts))
		}(i)
	}
	wg.Wait()

	bar := agg.CurrentBar("AAPL")
	if bar == nil {
		t.Fatal("expected current bar after concurrent writes")
	}
	if bar.TickCount != 100 {
		t.Errorf("tick count = %d, want 100", bar.TickCount)
	}
}

func TestMultipleInstrumentsIndependent(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(time.Minute, out)

	base := time.Date(2026, 4, 2, 10, 0, 15, 0, time.UTC)
	agg.Ingest(tick("AAPL", 150.0, base))
	agg.Ingest(tick("BTC-USD", 60000.0, base))

	aapl := agg.CurrentBar("AAPL")
	btc := agg.CurrentBar("BTC-USD")

	if aapl == nil || btc == nil {
		t.Fatal("expected bars for both instruments")
	}
	if !aapl.Open.Equal(decimal.NewFromFloat(150.0)) {
		t.Errorf("AAPL open = %s, want 150", aapl.Open)
	}
	if !btc.Open.Equal(decimal.NewFromFloat(60000.0)) {
		t.Errorf("BTC open = %s, want 60000", btc.Open)
	}
}

func TestPeriodStartAlignedToInterval(t *testing.T) {
	out := make(chan OHLCBar, 10)
	agg := NewAggregator(5*time.Minute, out)

	// 10:03:45 should align to period starting at 10:00:00
	ts := time.Date(2026, 4, 2, 10, 3, 45, 0, time.UTC)
	agg.Ingest(tick("AAPL", 150.0, ts))

	bar := agg.CurrentBar("AAPL")
	if bar == nil {
		t.Fatal("expected current bar")
	}
	expected := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	if !bar.PeriodStart.Equal(expected) {
		t.Errorf("period start = %v, want %v", bar.PeriodStart, expected)
	}
}

func drainChannel(ch <-chan OHLCBar, max int) []OHLCBar {
	var bars []OHLCBar
	for i := 0; i < max; i++ {
		select {
		case bar := <-ch:
			bars = append(bars, bar)
		default:
			return bars
		}
	}
	return bars
}
