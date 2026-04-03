package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/synapse-oms/gateway/internal/apperror"
)

// SettingStore is the interface for reading/writing app settings.
type SettingStore interface {
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
}

// SettingHandler implements REST endpoints for app settings.
type SettingHandler struct {
	store SettingStore
}

// NewSettingHandler creates a SettingHandler.
func NewSettingHandler(s SettingStore) *SettingHandler {
	return &SettingHandler{store: s}
}

// getOnboardingCompleted handles GET /api/v1/settings/onboarding_completed.
func (h *SettingHandler) getOnboardingCompleted(w http.ResponseWriter, r *http.Request) {
	val, err := h.store.GetSetting(r.Context(), "onboarding_completed")
	if err != nil {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "SETTING_READ_ERROR",
			Message:    "failed to read setting: " + err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"completed": val == "true",
	})
}

// completeOnboarding handles POST /api/v1/settings/onboarding_completed.
func (h *SettingHandler) completeOnboarding(w http.ResponseWriter, r *http.Request) {
	if err := h.store.SetSetting(r.Context(), "onboarding_completed", "true"); err != nil {
		apperror.WriteError(w, &apperror.AppError{
			Code:       "SETTING_WRITE_ERROR",
			Message:    "failed to save setting: " + err.Error(),
			HTTPStatus: http.StatusInternalServerError,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]bool{
		"completed": true,
	})
}
