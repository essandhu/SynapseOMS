package kafka

import (
	"encoding/json"
	"testing"
	"time"
)

func TestOrderLifecycleEvent_FillReceived_SerializesForRiskEngine(t *testing.T) {
	event := OrderLifecycleEvent{
		Type:    EventFillReceived,
		OrderID: "order-123",
		Fill: &FillPayload{
			InstrumentID:    "AAPL",
			VenueID:         "alpaca",
			Side:            "buy",
			Quantity:        "100",
			Price:           "185.03",
			Fee:             "0.50",
			AssetClass:      "equity",
			SettlementCycle: "T2",
			Timestamp:       "2026-04-04T12:00:00Z",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Verify the JSON can be parsed by the risk engine consumer format
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["type"] != "fill_received" {
		t.Errorf("type = %v, want fill_received", parsed["type"])
	}
	if parsed["order_id"] != "order-123" {
		t.Errorf("order_id = %v, want order-123", parsed["order_id"])
	}

	fill, ok := parsed["fill"].(map[string]interface{})
	if !ok {
		t.Fatal("fill field missing or not an object")
	}
	if fill["instrument_id"] != "AAPL" {
		t.Errorf("fill.instrument_id = %v, want AAPL", fill["instrument_id"])
	}
	if fill["venue_id"] != "alpaca" {
		t.Errorf("fill.venue_id = %v, want alpaca", fill["venue_id"])
	}
	if fill["side"] != "buy" {
		t.Errorf("fill.side = %v, want buy", fill["side"])
	}
	if fill["quantity"] != "100" {
		t.Errorf("fill.quantity = %v, want 100", fill["quantity"])
	}
	if fill["price"] != "185.03" {
		t.Errorf("fill.price = %v, want 185.03", fill["price"])
	}
	if fill["asset_class"] != "equity" {
		t.Errorf("fill.asset_class = %v, want equity", fill["asset_class"])
	}
	if fill["settlement_cycle"] != "T2" {
		t.Errorf("fill.settlement_cycle = %v, want T2", fill["settlement_cycle"])
	}
}

func TestOrderLifecycleEvent_StatusOnly_OmitsFill(t *testing.T) {
	event := OrderLifecycleEvent{
		Type:    EventOrderCreated,
		OrderID: "order-456",
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if parsed["type"] != "order_created" {
		t.Errorf("type = %v, want order_created", parsed["type"])
	}
	if _, exists := parsed["fill"]; exists {
		t.Error("fill field should be omitted for status-only events")
	}
}

func TestSettlementCycleForAssetClass(t *testing.T) {
	tests := []struct {
		assetClass string
		want       string
	}{
		{"equity", "T2"},
		{"crypto", "T0"},
		{"tokenized_security", "T0"},
		{"future", "T2"},
		{"option", "T2"},
	}
	for _, tt := range tests {
		t.Run(tt.assetClass, func(t *testing.T) {
			got := SettlementCycleForAssetClass(tt.assetClass)
			if got != tt.want {
				t.Errorf("SettlementCycleForAssetClass(%q) = %q, want %q", tt.assetClass, got, tt.want)
			}
		})
	}
}

func TestFormatTimestamp(t *testing.T) {
	ts := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	got := FormatTimestamp(ts)
	if got != "2026-04-04T12:00:00Z" {
		t.Errorf("FormatTimestamp = %q, want 2026-04-04T12:00:00Z", got)
	}
}

func TestEventTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"OrderCreated", EventOrderCreated, "order_created"},
		{"OrderAcknowledged", EventOrderAcknowledged, "order_acknowledged"},
		{"FillReceived", EventFillReceived, "fill_received"},
		{"OrderFilled", EventOrderFilled, "order_filled"},
		{"OrderCanceled", EventOrderCanceled, "order_canceled"},
		{"OrderRejected", EventOrderRejected, "order_rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}
