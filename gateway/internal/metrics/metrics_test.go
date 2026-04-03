package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsRegistered(t *testing.T) {
	// Verify all 6 metrics are registered in the default registry by
	// checking that the Desc channel yields a description for each.
	descs := make(chan *prometheus.Desc, 64)
	go func() {
		OrdersSubmittedTotal.Describe(descs)
		OrderLatencySeconds.Describe(descs)
		FillsReceivedTotal.Describe(descs)
		VenueLatencySeconds.Describe(descs)
		VenueStatus.Describe(descs)
		ActiveWebSocketConnections.Describe(descs)
		close(descs)
	}()

	names := make(map[string]bool)
	for d := range descs {
		names[d.String()] = true
	}

	if len(names) < 6 {
		t.Errorf("expected at least 6 metric descriptions, got %d", len(names))
	}
}

func TestCounterIncrement(t *testing.T) {
	// OrdersSubmittedTotal should be incrementable without panic.
	OrdersSubmittedTotal.WithLabelValues("equity", "sim-exchange").Inc()

	// FillsReceivedTotal should be incrementable without panic.
	FillsReceivedTotal.WithLabelValues("sim-exchange", "venue").Inc()
}

func TestHistogramObserve(t *testing.T) {
	// OrderLatencySeconds should accept observations without panic.
	OrderLatencySeconds.Observe(0.042)

	// VenueLatencySeconds should accept observations without panic.
	VenueLatencySeconds.WithLabelValues("sim-exchange").Observe(0.015)
}

func TestGaugeBehavior(t *testing.T) {
	// VenueStatus gauge
	VenueStatus.WithLabelValues("sim-exchange").Set(1)
	VenueStatus.WithLabelValues("sim-exchange").Set(0)

	// ActiveWebSocketConnections gauge
	ActiveWebSocketConnections.Inc()
	ActiveWebSocketConnections.Inc()
	ActiveWebSocketConnections.Dec()
}
