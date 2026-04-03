package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	OrdersSubmittedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_orders_submitted_total",
		Help: "Total orders submitted",
	}, []string{"asset_class", "venue"})

	OrderLatencySeconds = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "gateway_order_latency_seconds",
		Help:    "End-to-end order processing time (submit to venue ack)",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
	})

	FillsReceivedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_fills_received_total",
		Help: "Total fills received",
	}, []string{"venue", "liquidity_type"})

	VenueLatencySeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "gateway_venue_latency_seconds",
		Help:    "Per-venue round-trip latency",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1},
	}, []string{"venue"})

	VenueStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "gateway_venue_status",
		Help: "Venue connection status (1=connected, 0=disconnected)",
	}, []string{"venue"})

	ActiveWebSocketConnections = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "gateway_active_websocket_connections",
		Help: "Current WebSocket client count",
	})
)
