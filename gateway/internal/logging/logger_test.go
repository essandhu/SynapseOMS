package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// logEntry represents a parsed JSON log line for test assertions.
type logEntry struct {
	Timestamp     string `json:"timestamp"`
	Level         string `json:"level"`
	Msg           string `json:"msg"`
	Service       string `json:"service"`
	Component     string `json:"component"`
	CorrelationID string `json:"correlation_id"`
}

func parseLogEntry(t *testing.T, raw []byte) logEntry {
	t.Helper()
	var entry logEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("log output is not valid JSON: %s\nerror: %v", raw, err)
	}
	return entry
}

func TestNewLogger_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "gateway", "test-component")

	logger.Info("hello world")

	entry := parseLogEntry(t, buf.Bytes())

	if entry.Timestamp == "" {
		t.Error("expected timestamp to be present")
	}
	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %q", entry.Level)
	}
	if entry.Service != "gateway" {
		t.Errorf("expected service gateway, got %q", entry.Service)
	}
	if entry.Component != "test-component" {
		t.Errorf("expected component test-component, got %q", entry.Component)
	}
	if entry.Msg != "hello world" {
		t.Errorf("expected msg 'hello world', got %q", entry.Msg)
	}
}

func TestWithCorrelationID_PropagatesThroughContext(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()
	corrID := "test-corr-123"

	ctx = WithCorrelationID(ctx, corrID)

	logger := FromContext(ctx, &buf, "gateway", "handler")
	logger.Info("processing request")

	entry := parseLogEntry(t, buf.Bytes())

	if entry.CorrelationID != corrID {
		t.Errorf("expected correlation_id %q, got %q", corrID, entry.CorrelationID)
	}
}

func TestFromContext_WithoutCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	ctx := context.Background()

	logger := FromContext(ctx, &buf, "gateway", "handler")
	logger.Info("no correlation id set")

	entry := parseLogEntry(t, buf.Bytes())

	if entry.CorrelationID != "" {
		t.Errorf("expected empty correlation_id, got %q", entry.CorrelationID)
	}
	if entry.Service != "gateway" {
		t.Errorf("expected service gateway, got %q", entry.Service)
	}
}

func TestCorrelationIDMiddleware_ExtractsFromHeader(t *testing.T) {
	expectedID := "from-header-456"
	var capturedCtx context.Context

	handler := CorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Correlation-ID", expectedID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	got := CorrelationIDFromContext(capturedCtx)
	if got != expectedID {
		t.Errorf("expected correlation_id %q from header, got %q", expectedID, got)
	}
}

func TestCorrelationIDMiddleware_GeneratesUUID_WhenMissing(t *testing.T) {
	var capturedCtx context.Context

	handler := CorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	got := CorrelationIDFromContext(capturedCtx)
	if got == "" {
		t.Error("expected auto-generated correlation_id, got empty string")
	}
	// UUID v4 format: 8-4-4-4-12 hex chars
	if len(got) != 36 {
		t.Errorf("expected UUID format (36 chars), got %q (len %d)", got, len(got))
	}
}

func TestCorrelationIDMiddleware_SetsResponseHeader(t *testing.T) {
	handler := CorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	respHeader := rr.Header().Get("X-Correlation-ID")
	if respHeader == "" {
		t.Error("expected X-Correlation-ID in response header")
	}
}

func TestFromContext_IncludesCorrelationID_AfterMiddleware(t *testing.T) {
	var buf bytes.Buffer
	expectedID := "middleware-corr-789"

	handler := CorrelationIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger := FromContext(r.Context(), &buf, "gateway", "handler")
		logger.Info("inside handler")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Correlation-ID", expectedID)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	entry := parseLogEntry(t, buf.Bytes())
	if entry.CorrelationID != expectedID {
		t.Errorf("expected correlation_id %q in log, got %q", expectedID, entry.CorrelationID)
	}
}

func TestNew_ReturnsValidSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, "gateway", "rest")

	// Ensure it's a proper *slog.Logger
	var _ *slog.Logger = logger

	logger.Warn("test warning")
	entry := parseLogEntry(t, buf.Bytes())
	if entry.Level != "WARN" {
		t.Errorf("expected level WARN, got %q", entry.Level)
	}
}
