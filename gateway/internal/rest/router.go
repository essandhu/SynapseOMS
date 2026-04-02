// Package rest provides the HTTP REST API for the gateway.
package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
	"github.com/synapse-oms/gateway/internal/store"
)

// OrderSubmitter is the interface used to submit orders to the pipeline.
type OrderSubmitter interface {
	Submit(ctx context.Context, order *domain.Order) error
}

// ReadStore is the interface for read queries used by REST handlers.
type ReadStore interface {
	GetOrder(ctx context.Context, id domain.OrderID) (*domain.Order, error)
	ListOrders(ctx context.Context, filter store.OrderFilter) ([]domain.Order, error)
	GetPosition(ctx context.Context, instrumentID, venueID string) (*domain.Position, error)
	ListPositions(ctx context.Context) ([]domain.Position, error)
	GetInstrument(ctx context.Context, id string) (*domain.Instrument, error)
	ListInstruments(ctx context.Context) ([]domain.Instrument, error)
	ListFillsByOrder(ctx context.Context, orderID domain.OrderID) ([]domain.Fill, error)
}

// NewRouter creates a new chi router with all REST endpoints and middleware.
func NewRouter(pipeline OrderSubmitter, store ReadStore, opts ...RouterOption) http.Handler {
	cfg := &routerConfig{}
	for _, o := range opts {
		o(cfg)
	}

	r := chi.NewRouter()

	// Middleware stack
	r.Use(middleware.Recoverer)
	r.Use(logging.CorrelationIDMiddleware)
	r.Use(corsMiddleware)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	h := &handler{
		pipeline: pipeline,
		store:    store,
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", healthHandler)

		r.Post("/orders", h.submitOrder)
		r.Delete("/orders/{id}", h.cancelOrder)
		r.Get("/orders", h.listOrders)
		r.Get("/orders/{id}", h.getOrder)

		r.Get("/positions", h.listPositions)
		r.Get("/positions/{instrumentID}", h.getPosition)

		r.Get("/instruments", h.listInstruments)

		// Venue endpoints
		if cfg.venueHandler != nil {
			r.Get("/venues", cfg.venueHandler.listVenues)
			r.Post("/venues/{id}/connect", cfg.venueHandler.connectVenue)
			r.Post("/venues/{id}/disconnect", cfg.venueHandler.disconnectVenue)
		}

		// Credential endpoints
		if cfg.credentialHandler != nil {
			r.Post("/credentials", cfg.credentialHandler.storeCredential)
			r.Delete("/credentials/{venue_id}", cfg.credentialHandler.deleteCredential)
		}
	})

	return r
}

// RouterOption configures optional dependencies for the router.
type RouterOption func(*routerConfig)

type routerConfig struct {
	venueHandler      *VenueHandler
	credentialHandler *CredentialHandler
}

// WithVenueHandler adds venue endpoints to the router.
func WithVenueHandler(vh *VenueHandler) RouterOption {
	return func(c *routerConfig) {
		c.venueHandler = vh
	}
}

// WithCredentialHandler adds credential endpoints to the router.
func WithCredentialHandler(ch *CredentialHandler) RouterOption {
	return func(c *routerConfig) {
		c.credentialHandler = ch
	}
}

// handler holds the dependencies for REST handlers.
type handler struct {
	pipeline OrderSubmitter
	store    ReadStore
}

// healthHandler responds with {"status":"ok"}.
func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// corsMiddleware sets CORS headers allowing localhost:3000.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Correlation-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
