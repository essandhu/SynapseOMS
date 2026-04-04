package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/apperror"
	"github.com/synapse-oms/gateway/internal/domain"
)

// CredentialChecker is the minimal interface needed by VenueHandler.
type CredentialChecker interface {
	HasCredential(ctx context.Context, venueID string) (bool, error)
}

// CredentialRetriever extends CredentialChecker with the ability to load
// decrypted credentials for connecting to a venue.
type CredentialRetriever interface {
	CredentialChecker
	Retrieve(ctx context.Context, venueID string) (*domain.VenueCredential, error)
}

// MarketDataIngester processes market data ticks (implemented by Aggregator).
type MarketDataIngester interface {
	Ingest(snap adapter.MarketDataSnapshot)
}

// VenueHandler implements REST endpoints for venue management.
type VenueHandler struct {
	credMgr    CredentialRetriever
	logger     *slog.Logger
	aggregators []MarketDataIngester
	mdCancels  map[string]context.CancelFunc // venueID -> cancel func for market data goroutine
}

// NewVenueHandler creates a VenueHandler with the given dependencies.
func NewVenueHandler(credMgr CredentialRetriever, logger *slog.Logger, aggregators ...MarketDataIngester) *VenueHandler {
	return &VenueHandler{
		credMgr:    credMgr,
		logger:     logger,
		aggregators: aggregators,
		mdCancels:  make(map[string]context.CancelFunc),
	}
}

// venueResponse is the JSON response for a single venue.
type venueResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	Status          string   `json:"status"`
	SupportedAssets []string `json:"supportedAssets"`
	HasCredentials  bool     `json:"hasCredentials"`
	LatencyP50Ms    int64    `json:"latencyP50Ms"`
	LatencyP99Ms    int64    `json:"latencyP99Ms"`
	FillRate        float64  `json:"fillRate"`
	LastHeartbeat   string   `json:"lastHeartbeat"`
}

// listVenues handles GET /api/v1/venues.
func (h *VenueHandler) listVenues(w http.ResponseWriter, r *http.Request) {
	instances := adapter.ListInstances()

	result := make([]venueResponse, 0, len(instances))
	for _, p := range instances {
		hasCred, err := h.credMgr.HasCredential(r.Context(), p.VenueID())
		if err != nil {
			h.logger.WarnContext(r.Context(), "failed to check credentials",
				slog.String("venue_id", p.VenueID()),
				slog.String("error", err.Error()),
			)
			hasCred = false
		}

		var latencyP50Ms int64
		var lastHeartbeat string
		if p.Status() == adapter.Connected {
			latency, err := p.Ping(r.Context())
			if err == nil {
				latencyP50Ms = latency.Milliseconds()
			}
			lastHeartbeat = time.Now().UTC().Format(time.RFC3339)
		}

		// Estimate P99 as ~3x P50 when no histogram data is available.
		var latencyP99Ms int64
		if latencyP50Ms > 0 {
			latencyP99Ms = latencyP50Ms * 3
		}

		// Default fill rate: 1.0 for simulated venues (deterministic fills),
		// 0 for disconnected venues (no data).
		var fillRate float64
		if p.Status() == adapter.Connected {
			fillRate = 1.0
			if p.VenueType() != "simulated" {
				fillRate = 0.95 // default for real exchanges until metric aggregation is implemented
			}
		}

		assets := make([]string, 0, len(p.SupportedAssetClasses()))
		for _, ac := range p.SupportedAssetClasses() {
			assets = append(assets, assetClassToString(ac))
		}

		result = append(result, venueResponse{
			ID:              p.VenueID(),
			Name:            p.VenueName(),
			Type:            p.VenueType(),
			Status:          strings.ToLower(p.Status().String()),
			SupportedAssets: assets,
			HasCredentials:  hasCred,
			LatencyP50Ms:    latencyP50Ms,
			LatencyP99Ms:    latencyP99Ms,
			FillRate:        fillRate,
			LastHeartbeat:   lastHeartbeat,
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

// connectVenue handles POST /api/v1/venues/{id}/connect.
func (h *VenueHandler) connectVenue(w http.ResponseWriter, r *http.Request) {
	venueID := chi.URLParam(r, "id")

	p, ok := adapter.GetInstance(venueID)
	if !ok {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VENUE_NOT_FOUND",
			Message:    "venue not found: " + venueID,
			HTTPStatus: http.StatusNotFound,
		})
		return
	}

	cred, err := h.credMgr.Retrieve(r.Context(), venueID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "failed to retrieve credentials",
			slog.String("venue_id", venueID),
			slog.String("error", err.Error()),
		)
		apperror.WriteError(w, &apperror.AppError{
			Code:       "CREDENTIAL_ERROR",
			Message:    "credentials not found for venue: " + venueID,
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	if err := p.Connect(r.Context(), *cred); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to connect to venue",
			slog.String("venue_id", venueID),
			slog.String("error", err.Error()),
		)
		apperror.WriteError(w, &apperror.AppError{
			Code:       "CONNECT_ERROR",
			Message:    "failed to connect to venue: " + err.Error(),
			HTTPStatus: http.StatusBadGateway,
		})
		return
	}

	h.logger.InfoContext(r.Context(), "venue connected",
		slog.String("venue_id", venueID),
	)

	// Subscribe to market data and feed to aggregators
	if len(h.aggregators) > 0 {
		h.subscribeMarketData(venueID, p)
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":     venueID,
		"status": "connected",
	})
}

// disconnectVenue handles POST /api/v1/venues/{id}/disconnect.
func (h *VenueHandler) disconnectVenue(w http.ResponseWriter, r *http.Request) {
	venueID := chi.URLParam(r, "id")

	p, ok := adapter.GetInstance(venueID)
	if !ok {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VENUE_NOT_FOUND",
			Message:    "venue not found: " + venueID,
			HTTPStatus: http.StatusNotFound,
		})
		return
	}

	// Cancel market data subscription before disconnecting
	if cancel, ok := h.mdCancels[venueID]; ok {
		cancel()
		delete(h.mdCancels, venueID)
	}

	if err := p.Disconnect(r.Context()); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to disconnect from venue",
			slog.String("venue_id", venueID),
			slog.String("error", err.Error()),
		)
		apperror.WriteError(w, &apperror.AppError{
			Code:       "DISCONNECT_ERROR",
			Message:    "failed to disconnect from venue: " + err.Error(),
			HTTPStatus: http.StatusBadGateway,
		})
		return
	}

	h.logger.InfoContext(r.Context(), "venue disconnected",
		slog.String("venue_id", venueID),
	)

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":     venueID,
		"status": "disconnected",
	})
}

// subscribeMarketData starts a goroutine that feeds market data from a venue
// to all registered aggregators. The goroutine is cancelable via mdCancels.
func (h *VenueHandler) subscribeMarketData(venueID string, p adapter.LiquidityProvider) {
	ctx, cancel := context.WithCancel(context.Background())
	h.mdCancels[venueID] = cancel

	mdChan, err := p.SubscribeMarketData(ctx, nil)
	if err != nil {
		h.logger.Warn("failed to subscribe to market data on connect",
			slog.String("venue_id", venueID),
			slog.String("error", err.Error()),
		)
		cancel()
		delete(h.mdCancels, venueID)
		return
	}

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case snap, ok := <-mdChan:
				if !ok {
					return
				}
				for _, agg := range h.aggregators {
					agg.Ingest(snap)
				}
			}
		}
	}()

	h.logger.Info("market data subscribed on venue connect",
		slog.String("venue_id", venueID),
	)
}

// assetClassToString converts a domain.AssetClass to its JSON-friendly string.
func assetClassToString(ac domain.AssetClass) string {
	switch ac {
	case domain.AssetClassEquity:
		return "equity"
	case domain.AssetClassCrypto:
		return "crypto"
	case domain.AssetClassTokenizedSecurity:
		return "tokenized_security"
	case domain.AssetClassFuture:
		return "future"
	case domain.AssetClassOption:
		return "option"
	default:
		return "unknown"
	}
}
