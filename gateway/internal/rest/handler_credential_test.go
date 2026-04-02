package rest_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/rest"
)

// --- mock credential manager ---

type mockCredentialManager struct {
	storeFn  func(ctx context.Context, cred domain.VenueCredential) error
	deleteFn func(ctx context.Context, venueID string) error
}

func (m *mockCredentialManager) Store(ctx context.Context, cred domain.VenueCredential) error {
	if m.storeFn != nil {
		return m.storeFn(ctx, cred)
	}
	return nil
}

func (m *mockCredentialManager) Delete(ctx context.Context, venueID string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, venueID)
	}
	return nil
}

// --- helpers ---

func setupRouterWithCredentials(ms *mockStore, mp *mockPipeline, cm *mockCredentialManager) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ch := rest.NewCredentialHandler(cm, logger)
	return rest.NewRouter(mp, ms, rest.WithCredentialHandler(ch))
}

// --- tests ---

func TestStoreCredential_Success(t *testing.T) {
	var storedCred domain.VenueCredential
	cm := &mockCredentialManager{
		storeFn: func(_ context.Context, cred domain.VenueCredential) error {
			storedCred = cred
			return nil
		},
	}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{"venueId":"binance","apiKey":"key123","apiSecret":"secret456","passphrase":"pass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["venueId"] != "binance" {
		t.Errorf("expected venueId 'binance', got %v", resp["venueId"])
	}
	if resp["stored"] != true {
		t.Errorf("expected stored true, got %v", resp["stored"])
	}

	// Verify the credential was passed to the manager correctly.
	if storedCred.VenueID != "binance" {
		t.Errorf("expected stored VenueID 'binance', got %s", storedCred.VenueID)
	}
	if storedCred.APIKey != "key123" {
		t.Errorf("expected stored APIKey 'key123', got %s", storedCred.APIKey)
	}
	if storedCred.APISecret != "secret456" {
		t.Errorf("expected stored APISecret 'secret456', got %s", storedCred.APISecret)
	}
	if storedCred.Passphrase != "pass" {
		t.Errorf("expected stored Passphrase 'pass', got %s", storedCred.Passphrase)
	}
}

func TestStoreCredential_MissingVenueID(t *testing.T) {
	cm := &mockCredentialManager{}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{"venueId":"","apiKey":"key123","apiSecret":"secret456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", errResp.Error.Code)
	}
}

func TestStoreCredential_MissingAPIKey(t *testing.T) {
	cm := &mockCredentialManager{}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{"venueId":"binance","apiKey":"","apiSecret":"secret456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", errResp.Error.Code)
	}
}

func TestStoreCredential_MissingAPISecret(t *testing.T) {
	cm := &mockCredentialManager{}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{"venueId":"binance","apiKey":"key123","apiSecret":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", errResp.Error.Code)
	}
}

func TestStoreCredential_InvalidJSON(t *testing.T) {
	cm := &mockCredentialManager{}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", errResp.Error.Code)
	}
}

func TestStoreCredential_StoreFails(t *testing.T) {
	cm := &mockCredentialManager{
		storeFn: func(_ context.Context, _ domain.VenueCredential) error {
			return errors.New("database connection lost")
		},
	}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	body := `{"venueId":"binance","apiKey":"key123","apiSecret":"secret456"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/credentials", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "CREDENTIAL_STORE_ERROR" {
		t.Errorf("expected error code CREDENTIAL_STORE_ERROR, got %s", errResp.Error.Code)
	}
}

func TestDeleteCredential_Success(t *testing.T) {
	var deletedVenueID string
	cm := &mockCredentialManager{
		deleteFn: func(_ context.Context, venueID string) error {
			deletedVenueID = venueID
			return nil
		},
	}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/credentials/binance", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if deletedVenueID != "binance" {
		t.Errorf("expected deleted venue ID 'binance', got %s", deletedVenueID)
	}
}

func TestDeleteCredential_DeleteFails(t *testing.T) {
	cm := &mockCredentialManager{
		deleteFn: func(_ context.Context, _ string) error {
			return errors.New("vault unavailable")
		},
	}
	router := setupRouterWithCredentials(&mockStore{}, &mockPipeline{}, cm)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/credentials/binance", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "CREDENTIAL_DELETE_ERROR" {
		t.Errorf("expected error code CREDENTIAL_DELETE_ERROR, got %s", errResp.Error.Code)
	}
}
