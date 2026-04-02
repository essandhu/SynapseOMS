package rest

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/synapse-oms/gateway/internal/apperror"
	"github.com/synapse-oms/gateway/internal/domain"
)

// CredentialManager is the interface for storing and deleting credentials.
type CredentialManager interface {
	Store(ctx context.Context, cred domain.VenueCredential) error
	Delete(ctx context.Context, venueID string) error
}

// CredentialHandler implements REST endpoints for credential management.
type CredentialHandler struct {
	credMgr CredentialManager
	logger  *slog.Logger
}

// NewCredentialHandler creates a CredentialHandler with the given dependencies.
func NewCredentialHandler(credMgr CredentialManager, logger *slog.Logger) *CredentialHandler {
	return &CredentialHandler{
		credMgr: credMgr,
		logger:  logger,
	}
}

// storeCredentialRequest is the JSON request body for POST /credentials.
type storeCredentialRequest struct {
	VenueID    string `json:"venueId"`
	APIKey     string `json:"apiKey"`
	APISecret  string `json:"apiSecret"`
	Passphrase string `json:"passphrase"`
}

// storeCredentialResponse is the JSON response for POST /credentials.
type storeCredentialResponse struct {
	VenueID string `json:"venueId"`
	Stored  bool   `json:"stored"`
}

// storeCredential handles POST /api/v1/credentials.
func (h *CredentialHandler) storeCredential(w http.ResponseWriter, r *http.Request) {
	var req storeCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "invalid JSON body",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	if strings.TrimSpace(req.VenueID) == "" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "venueId is required",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	if strings.TrimSpace(req.APIKey) == "" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "apiKey is required",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	if strings.TrimSpace(req.APISecret) == "" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "apiSecret is required",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	cred := domain.VenueCredential{
		VenueID:    req.VenueID,
		APIKey:     req.APIKey,
		APISecret:  req.APISecret,
		Passphrase: req.Passphrase,
	}

	if err := h.credMgr.Store(r.Context(), cred); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to store credentials",
			slog.String("venue_id", req.VenueID),
			slog.String("error", err.Error()),
		)
		apperror.WriteError(w, &apperror.AppError{
			Code:       "CREDENTIAL_STORE_ERROR",
			Message:    "failed to store credentials",
			HTTPStatus: http.StatusInternalServerError,
		})
		return
	}

	h.logger.InfoContext(r.Context(), "credentials stored",
		slog.String("venue_id", req.VenueID),
	)

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(storeCredentialResponse{
		VenueID: req.VenueID,
		Stored:  true,
	})
}

// deleteCredential handles DELETE /api/v1/credentials/{venue_id}.
func (h *CredentialHandler) deleteCredential(w http.ResponseWriter, r *http.Request) {
	venueID := chi.URLParam(r, "venue_id")

	if strings.TrimSpace(venueID) == "" {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "VALIDATION_ERROR",
			Message:    "venue_id is required",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	if err := h.credMgr.Delete(r.Context(), venueID); err != nil {
		h.logger.ErrorContext(r.Context(), "failed to delete credentials",
			slog.String("venue_id", venueID),
			slog.String("error", err.Error()),
		)
		apperror.WriteError(w, &apperror.AppError{
			Code:       "CREDENTIAL_DELETE_ERROR",
			Message:    "failed to delete credentials",
			HTTPStatus: http.StatusInternalServerError,
		})
		return
	}

	h.logger.InfoContext(r.Context(), "credentials deleted",
		slog.String("venue_id", venueID),
	)

	w.WriteHeader(http.StatusNoContent)
}
