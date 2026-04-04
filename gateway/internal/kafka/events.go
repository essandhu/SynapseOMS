package kafka

import "time"

// OrderLifecycleEvent is the JSON envelope published to the order-lifecycle
// Kafka topic. The "type" field determines which payload fields are populated.
// This format matches what the risk engine's PortfolioStateBuilder consumer expects.
type OrderLifecycleEvent struct {
	Type    string `json:"type"`
	OrderID string `json:"order_id"`

	// Populated only for fill_received events.
	Fill *FillPayload `json:"fill,omitempty"`
}

// FillPayload contains the fill details needed by the risk engine to update
// portfolio positions and cash balances.
type FillPayload struct {
	InstrumentID    string `json:"instrument_id"`
	VenueID         string `json:"venue_id"`
	Side            string `json:"side"`
	Quantity        string `json:"quantity"`
	Price           string `json:"price"`
	Fee             string `json:"fee,omitempty"`
	AssetClass      string `json:"asset_class"`
	SettlementCycle string `json:"settlement_cycle"`
	Timestamp       string `json:"timestamp,omitempty"`
}

// Event type constants matching the risk engine consumer's expectations.
const (
	EventOrderCreated      = "order_created"
	EventOrderAcknowledged = "order_acknowledged"
	EventFillReceived      = "fill_received"
	EventOrderFilled       = "order_filled"
	EventOrderCanceled     = "order_canceled"
	EventOrderRejected     = "order_rejected"
)

// settlementCycleForAssetClass returns the settlement cycle based on the
// asset class string. Crypto settles instantly (T0), equities use T2.
func SettlementCycleForAssetClass(assetClass string) string {
	switch assetClass {
	case "crypto":
		return "T0"
	case "tokenized_security":
		return "T0"
	default:
		return "T2"
	}
}

// FormatTimestamp formats a time.Time for Kafka event payloads.
func FormatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
