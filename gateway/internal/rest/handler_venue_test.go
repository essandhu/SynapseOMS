package rest_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/rest"
)

// --- mock liquidity provider ---

type mockLiquidityProvider struct {
	venueID     string
	venueName   string
	venueType   string
	status      adapter.VenueStatus
	assetClasses []domain.AssetClass
	connectFn   func(ctx context.Context, cred domain.VenueCredential) error
	disconnFn   func(ctx context.Context) error
	pingFn      func(ctx context.Context) (time.Duration, error)
}

func (m *mockLiquidityProvider) VenueID() string  { return m.venueID }
func (m *mockLiquidityProvider) VenueName() string { return m.venueName }
func (m *mockLiquidityProvider) VenueType() string {
	if m.venueType != "" {
		return m.venueType
	}
	return "exchange"
}
func (m *mockLiquidityProvider) SupportedAssetClasses() []domain.AssetClass {
	if len(m.assetClasses) > 0 {
		return m.assetClasses
	}
	return []domain.AssetClass{domain.AssetClassCrypto}
}
func (m *mockLiquidityProvider) SupportedInstruments() ([]domain.Instrument, error) {
	return nil, nil
}
func (m *mockLiquidityProvider) Connect(ctx context.Context, cred domain.VenueCredential) error {
	if m.connectFn != nil {
		return m.connectFn(ctx, cred)
	}
	return nil
}
func (m *mockLiquidityProvider) Disconnect(ctx context.Context) error {
	if m.disconnFn != nil {
		return m.disconnFn(ctx)
	}
	return nil
}
func (m *mockLiquidityProvider) Status() adapter.VenueStatus { return m.status }
func (m *mockLiquidityProvider) Ping(ctx context.Context) (time.Duration, error) {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return 0, nil
}
func (m *mockLiquidityProvider) SubmitOrder(_ context.Context, _ *domain.Order) (*adapter.VenueAck, error) {
	return nil, nil
}
func (m *mockLiquidityProvider) CancelOrder(_ context.Context, _ domain.OrderID, _ string) error {
	return nil
}
func (m *mockLiquidityProvider) QueryOrder(_ context.Context, _ string) (*domain.Order, error) {
	return nil, nil
}
func (m *mockLiquidityProvider) SubscribeMarketData(_ context.Context, _ []string) (<-chan adapter.MarketDataSnapshot, error) {
	return nil, nil
}
func (m *mockLiquidityProvider) UnsubscribeMarketData(_ context.Context, _ []string) error {
	return nil
}
func (m *mockLiquidityProvider) FillFeed() <-chan domain.Fill { return nil }
func (m *mockLiquidityProvider) Capabilities() adapter.VenueCapabilities {
	classes := m.SupportedAssetClasses()
	return adapter.VenueCapabilities{
		SupportedAssetClasses: classes,
		SupportsStreaming:     true,
		MaxOrdersPerSecond:    100,
	}
}

// --- mock credential retriever ---

type mockCredRetriever struct {
	hasCredFn func(ctx context.Context, venueID string) (bool, error)
	retrieveFn func(ctx context.Context, venueID string) (*domain.VenueCredential, error)
}

func (m *mockCredRetriever) HasCredential(ctx context.Context, venueID string) (bool, error) {
	if m.hasCredFn != nil {
		return m.hasCredFn(ctx, venueID)
	}
	return true, nil
}

func (m *mockCredRetriever) Retrieve(ctx context.Context, venueID string) (*domain.VenueCredential, error) {
	if m.retrieveFn != nil {
		return m.retrieveFn(ctx, venueID)
	}
	return &domain.VenueCredential{
		VenueID:   venueID,
		APIKey:    "test-key",
		APISecret: "test-secret",
	}, nil
}

// --- helpers ---

func setupRouterWithVenue(ms *mockStore, mp *mockPipeline, cr *mockCredRetriever) http.Handler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vh := rest.NewVenueHandler(cr, logger)
	return rest.NewRouter(mp, ms, rest.WithVenueHandler(vh))
}

// --- tests ---

func TestListVenues_ReturnsRegisteredVenue(t *testing.T) {
	venueID := "test-list-venue-01"
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "Test Exchange",
		status:    adapter.Disconnected,
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{
		hasCredFn: func(_ context.Context, vid string) (bool, error) {
			return vid == venueID, nil
		},
	}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/venues", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var venues []map[string]interface{}
	decodeJSON(t, rec.Result(), &venues)

	// Find our test venue in the response (other tests may have registered venues).
	var found map[string]interface{}
	for _, v := range venues {
		if v["id"] == venueID {
			found = v
			break
		}
	}
	if found == nil {
		t.Fatalf("expected venue %s in response, not found", venueID)
	}

	if found["name"] != "Test Exchange" {
		t.Errorf("expected name 'Test Exchange', got %v", found["name"])
	}
	if found["status"] != "disconnected" {
		t.Errorf("expected status 'disconnected', got %v", found["status"])
	}
	if found["hasCredentials"] != true {
		t.Errorf("expected hasCredentials true, got %v", found["hasCredentials"])
	}

	assets, ok := found["supportedAssets"].([]interface{})
	if !ok || len(assets) == 0 {
		t.Errorf("expected supportedAssets to be non-empty, got %v", found["supportedAssets"])
	}
}

func TestConnectVenue_Success(t *testing.T) {
	venueID := "test-connect-venue-01"
	var connectCalled bool
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "Connect Exchange",
		status:    adapter.Disconnected,
		connectFn: func(_ context.Context, _ domain.VenueCredential) error {
			connectCalled = true
			return nil
		},
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/"+venueID+"/connect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if !connectCalled {
		t.Error("expected Connect to be called on the provider")
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["id"] != venueID {
		t.Errorf("expected id %s, got %v", venueID, resp["id"])
	}
	if resp["status"] != "connected" {
		t.Errorf("expected status 'connected', got %v", resp["status"])
	}
}

func TestConnectVenue_NotFound(t *testing.T) {
	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/nonexistent-venue-99/connect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VENUE_NOT_FOUND" {
		t.Errorf("expected error code VENUE_NOT_FOUND, got %s", errResp.Error.Code)
	}
}

func TestConnectVenue_CredentialRetrievalFails(t *testing.T) {
	venueID := "test-connect-credfail-01"
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "CredFail Exchange",
		status:    adapter.Disconnected,
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{
		retrieveFn: func(_ context.Context, _ string) (*domain.VenueCredential, error) {
			return nil, errors.New("credentials not found in vault")
		},
	}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/"+venueID+"/connect", nil)
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

	if errResp.Error.Code != "CREDENTIAL_ERROR" {
		t.Errorf("expected error code CREDENTIAL_ERROR, got %s", errResp.Error.Code)
	}
}

func TestConnectVenue_ConnectFails(t *testing.T) {
	venueID := "test-connect-fail-01"
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "FailConnect Exchange",
		status:    adapter.Disconnected,
		connectFn: func(_ context.Context, _ domain.VenueCredential) error {
			return errors.New("connection timeout")
		},
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/"+venueID+"/connect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "CONNECT_ERROR" {
		t.Errorf("expected error code CONNECT_ERROR, got %s", errResp.Error.Code)
	}
}

func TestDisconnectVenue_Success(t *testing.T) {
	venueID := "test-disconnect-venue-01"
	var disconnCalled bool
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "Disconnect Exchange",
		status:    adapter.Connected,
		disconnFn: func(_ context.Context) error {
			disconnCalled = true
			return nil
		},
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/"+venueID+"/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	if !disconnCalled {
		t.Error("expected Disconnect to be called on the provider")
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["id"] != venueID {
		t.Errorf("expected id %s, got %v", venueID, resp["id"])
	}
	if resp["status"] != "disconnected" {
		t.Errorf("expected status 'disconnected', got %v", resp["status"])
	}
}

func TestDisconnectVenue_NotFound(t *testing.T) {
	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/nonexistent-venue-88/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "VENUE_NOT_FOUND" {
		t.Errorf("expected error code VENUE_NOT_FOUND, got %s", errResp.Error.Code)
	}
}

func TestDisconnectVenue_DisconnectFails(t *testing.T) {
	venueID := "test-disconnect-fail-01"
	mlp := &mockLiquidityProvider{
		venueID:   venueID,
		venueName: "FailDisconn Exchange",
		status:    adapter.Connected,
		disconnFn: func(_ context.Context) error {
			return errors.New("graceful shutdown failed")
		},
	}
	adapter.RegisterInstance(venueID, mlp)

	cr := &mockCredRetriever{}
	router := setupRouterWithVenue(&mockStore{}, &mockPipeline{}, cr)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/venues/"+venueID+"/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var errResp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, rec.Result(), &errResp)

	if errResp.Error.Code != "DISCONNECT_ERROR" {
		t.Errorf("expected error code DISCONNECT_ERROR, got %s", errResp.Error.Code)
	}
}
