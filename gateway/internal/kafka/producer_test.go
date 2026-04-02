package kafka

import (
	"context"
	"testing"

	"github.com/synapse-oms/gateway/internal/logging"
)

func TestTopicConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      string
		expected string
	}{
		{"OrderLifecycle", TopicOrderLifecycle, "order-lifecycle"},
		{"MarketData", TopicMarketData, "market-data"},
		{"VenueStatus", TopicVenueStatus, "venue-status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %q, want %q", tt.got, tt.expected)
			}
		})
	}
}

func TestBuildHeaders_WithCorrelationID(t *testing.T) {
	ctx := logging.WithCorrelationID(context.Background(), "test-corr-123")
	headers := buildHeaders(ctx)

	if len(headers) != 1 {
		t.Fatalf("expected 1 header, got %d", len(headers))
	}
	if headers[0].Key != headerCorrelationID {
		t.Errorf("header key = %q, want %q", headers[0].Key, headerCorrelationID)
	}
	if string(headers[0].Value) != "test-corr-123" {
		t.Errorf("header value = %q, want %q", string(headers[0].Value), "test-corr-123")
	}
}

func TestBuildHeaders_WithoutCorrelationID(t *testing.T) {
	headers := buildHeaders(context.Background())
	if len(headers) != 0 {
		t.Errorf("expected 0 headers for empty context, got %d", len(headers))
	}
}

func TestBuildHeaders_EmptyCorrelationID(t *testing.T) {
	ctx := logging.WithCorrelationID(context.Background(), "")
	headers := buildHeaders(ctx)
	if len(headers) != 0 {
		t.Errorf("expected 0 headers for empty correlation ID, got %d", len(headers))
	}
}
