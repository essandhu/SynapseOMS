package router

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// VenueCandidate represents a venue that could receive an order,
// along with its current market data and quality metrics.
type VenueCandidate struct {
	VenueID      string
	BidPrice     decimal.Decimal
	AskPrice     decimal.Decimal
	DepthAtPrice decimal.Decimal // Available qty within 5bps of mid
	LatencyP50   time.Duration
	FillRate30d  float64
	FeeRate      decimal.Decimal // maker/taker fee as decimal (e.g. 0.001 = 10bps)
}

// VenueAllocation represents a decision to route a specific quantity
// to a particular venue, with a human-readable reason.
type VenueAllocation struct {
	VenueID  string
	Quantity decimal.Decimal
	Reason   string
}

// RoutingDecision captures the complete output of the routing process
// for a single order.
type RoutingDecision struct {
	OrderID     domain.OrderID
	Allocations []VenueAllocation
	Strategy    string
	Timestamp   time.Time
}
