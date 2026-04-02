// Package logging provides structured JSON logging with correlation ID propagation.
package logging

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

type contextKey string

const correlationIDKey contextKey = "correlation_id"

// New creates a new *slog.Logger that writes JSON to w with service and component pre-populated.
func New(w io.Writer, service, component string) *slog.Logger {
	handler := slog.NewJSONHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Key = "timestamp"
			}
			return a
		},
	})
	return slog.New(handler).With(
		slog.String("service", service),
		slog.String("component", component),
	)
}

// NewDefault creates a logger that writes to os.Stdout.
func NewDefault(service, component string) *slog.Logger {
	return New(os.Stdout, service, component)
}

// WithCorrelationID stores a correlation ID in the context.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, correlationIDKey, id)
}

// CorrelationIDFromContext extracts the correlation ID from context, returning "" if absent.
func CorrelationIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(correlationIDKey).(string); ok {
		return id
	}
	return ""
}

// FromContext returns a *slog.Logger with correlation_id from context (if present),
// plus service and component pre-populated.
func FromContext(ctx context.Context, w io.Writer, service, component string) *slog.Logger {
	logger := New(w, service, component)
	if id := CorrelationIDFromContext(ctx); id != "" {
		logger = logger.With(slog.String("correlation_id", id))
	}
	return logger
}

// CorrelationIDMiddleware extracts X-Correlation-ID from the request header,
// or generates a new UUID if missing. It stores the ID in context and sets
// the response header.
func CorrelationIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corrID := r.Header.Get("X-Correlation-ID")
		if corrID == "" {
			corrID = newUUID()
		}
		w.Header().Set("X-Correlation-ID", corrID)
		ctx := WithCorrelationID(r.Context(), corrID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// newUUID generates a UUID v4 using crypto/rand.
func newUUID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
