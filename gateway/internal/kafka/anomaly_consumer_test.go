package kafka

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"
)

// NOTE: This file requires CGO to compile because it shares the kafka package
// which imports confluent-kafka-go. Run tests with CGO_ENABLED=1 or via Docker.

func TestProcessMessage_ValidJSON(t *testing.T) {
	var received AnomalyAlert
	consumer := &AnomalyConsumer{
		callback: func(alert AnomalyAlert) { received = alert },
		logger:   slog.Default(),
	}

	alert := AnomalyAlert{
		ID:           "test-123",
		InstrumentID: "ETH-USD",
		VenueID:      "binance",
		AnomalyScore: -0.65,
		Severity:     "warning",
		Features:     map[string]float64{"volume_zscore": 4.2},
		Description:  "ETH-USD volume spike",
		Timestamp:    time.Now().UTC().Truncate(time.Second),
	}
	data, err := json.Marshal(alert)
	if err != nil {
		t.Fatalf("failed to marshal test alert: %v", err)
	}

	consumer.processMessage(data)

	if received.ID != "test-123" {
		t.Errorf("expected ID test-123, got %s", received.ID)
	}
	if received.InstrumentID != "ETH-USD" {
		t.Errorf("expected InstrumentID ETH-USD, got %s", received.InstrumentID)
	}
	if received.VenueID != "binance" {
		t.Errorf("expected VenueID binance, got %s", received.VenueID)
	}
	if received.AnomalyScore != -0.65 {
		t.Errorf("expected AnomalyScore -0.65, got %f", received.AnomalyScore)
	}
	if received.Severity != "warning" {
		t.Errorf("expected Severity warning, got %s", received.Severity)
	}
	if received.Features["volume_zscore"] != 4.2 {
		t.Errorf("expected volume_zscore 4.2, got %f", received.Features["volume_zscore"])
	}
	if received.Description != "ETH-USD volume spike" {
		t.Errorf("expected Description 'ETH-USD volume spike', got %s", received.Description)
	}
	if received.Acknowledged {
		t.Error("expected Acknowledged to be false")
	}
}

func TestProcessMessage_InvalidJSON(t *testing.T) {
	callCount := 0
	consumer := &AnomalyConsumer{
		callback: func(alert AnomalyAlert) { callCount++ },
		logger:   slog.Default(),
	}

	consumer.processMessage([]byte("not valid json"))

	if callCount != 0 {
		t.Error("callback should not be called for invalid JSON")
	}
}

func TestProcessMessage_EmptyPayload(t *testing.T) {
	callCount := 0
	consumer := &AnomalyConsumer{
		callback: func(alert AnomalyAlert) { callCount++ },
		logger:   slog.Default(),
	}

	consumer.processMessage([]byte{})

	if callCount != 0 {
		t.Error("callback should not be called for empty payload")
	}
}

func TestProcessMessage_PartialJSON(t *testing.T) {
	var received AnomalyAlert
	consumer := &AnomalyConsumer{
		callback: func(alert AnomalyAlert) { received = alert },
		logger:   slog.Default(),
	}

	// Only some fields present — should still deserialize successfully.
	partial := `{"id":"partial-1","severity":"critical","anomaly_score":-0.95}`
	consumer.processMessage([]byte(partial))

	if received.ID != "partial-1" {
		t.Errorf("expected ID partial-1, got %s", received.ID)
	}
	if received.Severity != "critical" {
		t.Errorf("expected Severity critical, got %s", received.Severity)
	}
	if received.AnomalyScore != -0.95 {
		t.Errorf("expected AnomalyScore -0.95, got %f", received.AnomalyScore)
	}
	if received.InstrumentID != "" {
		t.Errorf("expected empty InstrumentID, got %s", received.InstrumentID)
	}
}

func TestAnomalyAlertJSONRoundTrip(t *testing.T) {
	original := AnomalyAlert{
		ID:           "rt-001",
		InstrumentID: "BTC-USD",
		VenueID:      "coinbase",
		AnomalyScore: -0.82,
		Severity:     "critical",
		Features: map[string]float64{
			"spread_zscore": 3.5,
			"volume_zscore": 2.1,
		},
		Description:  "BTC-USD spread anomaly",
		Timestamp:    time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),
		Acknowledged: true,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded AnomalyAlert
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: got %s, want %s", decoded.ID, original.ID)
	}
	if decoded.AnomalyScore != original.AnomalyScore {
		t.Errorf("AnomalyScore mismatch: got %f, want %f", decoded.AnomalyScore, original.AnomalyScore)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if len(decoded.Features) != len(original.Features) {
		t.Errorf("Features length mismatch: got %d, want %d", len(decoded.Features), len(original.Features))
	}
	if !decoded.Acknowledged {
		t.Error("expected Acknowledged to be true")
	}
}
