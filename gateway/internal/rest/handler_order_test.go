package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/rest"
	"github.com/synapse-oms/gateway/internal/store"
)

// --- mock pipeline ---

type mockPipeline struct {
	submitFn func(ctx context.Context, order *domain.Order) error
}

func (m *mockPipeline) Submit(ctx context.Context, order *domain.Order) error {
	if m.submitFn != nil {
		return m.submitFn(ctx, order)
	}
	order.ID = "mock-order-id"
	order.Status = domain.OrderStatusNew
	order.CreatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	order.UpdatedAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return nil
}

// --- mock store ---

type mockStore struct {
	orders      []domain.Order
	positions   []domain.Position
	instruments []domain.Instrument
	fills       []domain.Fill
}

func (m *mockStore) CreateOrder(_ context.Context, _ *domain.Order) error { return nil }
func (m *mockStore) UpdateOrder(_ context.Context, _ *domain.Order) error { return nil }
func (m *mockStore) CreateFill(_ context.Context, _ *domain.Fill) error   { return nil }
func (m *mockStore) UpsertPosition(_ context.Context, _ *domain.Position) error {
	return nil
}

func (m *mockStore) GetOrder(_ context.Context, id domain.OrderID) (*domain.Order, error) {
	for i := range m.orders {
		if m.orders[i].ID == id {
			o := m.orders[i]
			return &o, nil
		}
	}
	return nil, fmt.Errorf("scanning order: no rows in result set")
}

func (m *mockStore) ListOrders(_ context.Context, filter store.OrderFilter) ([]domain.Order, error) {
	var result []domain.Order
	for _, o := range m.orders {
		if filter.Status != nil && o.Status != *filter.Status {
			continue
		}
		if filter.InstrumentID != nil && o.InstrumentID != *filter.InstrumentID {
			continue
		}
		result = append(result, o)
	}
	return result, nil
}

func (m *mockStore) GetPosition(_ context.Context, instrumentID, venueID string) (*domain.Position, error) {
	for i := range m.positions {
		if m.positions[i].InstrumentID == instrumentID {
			p := m.positions[i]
			return &p, nil
		}
	}
	return nil, fmt.Errorf("scanning position: no rows in result set")
}

func (m *mockStore) ListPositions(_ context.Context) ([]domain.Position, error) {
	return m.positions, nil
}

func (m *mockStore) GetInstrument(_ context.Context, id string) (*domain.Instrument, error) {
	for i := range m.instruments {
		if m.instruments[i].ID == id {
			inst := m.instruments[i]
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("scanning instrument: no rows in result set")
}

func (m *mockStore) ListInstruments(_ context.Context) ([]domain.Instrument, error) {
	return m.instruments, nil
}

func (m *mockStore) ListFillsByOrder(_ context.Context, orderID domain.OrderID) ([]domain.Fill, error) {
	var result []domain.Fill
	for _, f := range m.fills {
		if f.OrderID == orderID {
			result = append(result, f)
		}
	}
	return result, nil
}

// --- helpers ---

func setupRouter(ms *mockStore, mp *mockPipeline) http.Handler {
	return rest.NewRouter(mp, ms)
}

func decodeJSON(t *testing.T, resp *http.Response, v interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode json: %v", err)
	}
}

// --- tests ---

func TestHealthCheck(t *testing.T) {
	router := setupRouter(&mockStore{}, &mockPipeline{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]string
	decodeJSON(t, rec.Result(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", body["status"])
	}
}

func TestSubmitOrder_Success(t *testing.T) {
	// Register a connected venue that supports equity so the venue check passes.
	venueID := "test-order-venue-01"
	mlp := &mockLiquidityProvider{
		venueID:      venueID,
		venueName:    "Order Test Exchange",
		status:       adapter.Connected,
		assetClasses: []domain.AssetClass{domain.AssetClassEquity},
	}
	adapter.RegisterInstance(venueID, mlp)

	ms := &mockStore{
		instruments: []domain.Instrument{
			{ID: "AAPL", Symbol: "AAPL", Name: "Apple Inc", AssetClass: domain.AssetClassEquity},
		},
	}
	mp := &mockPipeline{}

	router := setupRouter(ms, mp)

	body := `{"instrument_id":"AAPL","side":"buy","type":"market","quantity":"10","price":"0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["id"] != "mock-order-id" {
		t.Errorf("expected order id mock-order-id, got %v", resp["id"])
	}
	if resp["instrument_id"] != "AAPL" {
		t.Errorf("expected instrument AAPL, got %v", resp["instrument_id"])
	}
	// Decimal values must be strings
	if resp["quantity"] != "10" {
		t.Errorf("expected quantity '10', got %v (type %T)", resp["quantity"], resp["quantity"])
	}
	// Asset class should be populated from instrument
	if resp["asset_class"] != "equity" {
		t.Errorf("expected asset_class 'equity', got %v", resp["asset_class"])
	}
}

func TestSubmitOrder_PopulatesAssetClassFromInstrument(t *testing.T) {
	venueID := "test-order-venue-crypto"
	mlp := &mockLiquidityProvider{
		venueID:      venueID,
		venueName:    "Crypto Exchange",
		status:       adapter.Connected,
		assetClasses: []domain.AssetClass{domain.AssetClassCrypto},
	}
	adapter.RegisterInstance(venueID, mlp)

	ms := &mockStore{
		instruments: []domain.Instrument{
			{ID: "BTC-USD", Symbol: "BTC-USD", Name: "Bitcoin", AssetClass: domain.AssetClassCrypto},
		},
	}
	mp := &mockPipeline{}

	router := setupRouter(ms, mp)

	body := `{"instrument_id":"BTC-USD","side":"buy","type":"market","quantity":"1.5","price":"0"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["asset_class"] != "crypto" {
		t.Errorf("expected asset_class 'crypto', got %v", resp["asset_class"])
	}
	if resp["instrument_id"] != "BTC-USD" {
		t.Errorf("expected instrument_id 'BTC-USD', got %v", resp["instrument_id"])
	}
}

func TestSubmitOrder_ValidationErrors(t *testing.T) {
	ms := &mockStore{
		instruments: []domain.Instrument{
			{ID: "AAPL", Symbol: "AAPL", Name: "Apple Inc"},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "missing instrument",
			body: `{"instrument_id":"","side":"buy","type":"market","quantity":"10","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "invalid side",
			body: `{"instrument_id":"AAPL","side":"short","type":"market","quantity":"10","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "invalid type",
			body: `{"instrument_id":"AAPL","side":"buy","type":"stop","quantity":"10","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "zero quantity",
			body: `{"instrument_id":"AAPL","side":"buy","type":"market","quantity":"0","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "negative quantity",
			body: `{"instrument_id":"AAPL","side":"buy","type":"market","quantity":"-5","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "limit order without price",
			body: `{"instrument_id":"AAPL","side":"buy","type":"limit","quantity":"10","price":"0"}`,
			code: "VALIDATION_ERROR",
		},
		{
			name: "unknown instrument",
			body: `{"instrument_id":"ZZZZ","side":"buy","type":"market","quantity":"10","price":"0"}`,
			code: "INSTRUMENT_NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
				t.Fatalf("expected 4xx, got %d; body: %s", rec.Code, rec.Body.String())
			}

			var errResp struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			decodeJSON(t, rec.Result(), &errResp)

			if errResp.Error.Code != tt.code {
				t.Errorf("expected error code %s, got %s", tt.code, errResp.Error.Code)
			}
		})
	}
}

func TestListOrders(t *testing.T) {
	ms := &mockStore{
		orders: []domain.Order{
			{
				ID:           "order-1",
				InstrumentID: "AAPL",
				Side:         domain.SideBuy,
				Type:         domain.OrderTypeMarket,
				Quantity:     decimal.NewFromInt(10),
				Price:        decimal.Zero,
				Status:       domain.OrderStatusNew,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
			{
				ID:           "order-2",
				InstrumentID: "MSFT",
				Side:         domain.SideSell,
				Type:         domain.OrderTypeLimit,
				Quantity:     decimal.NewFromInt(5),
				Price:        decimal.NewFromInt(350),
				Status:       domain.OrderStatusFilled,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var orders []map[string]interface{}
	decodeJSON(t, rec.Result(), &orders)
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// Check decimal serialization as strings
	if orders[0]["quantity"] != "10" {
		t.Errorf("expected quantity '10' as string, got %v (type %T)", orders[0]["quantity"], orders[0]["quantity"])
	}
}

func TestGetOrder(t *testing.T) {
	ms := &mockStore{
		orders: []domain.Order{
			{
				ID:             "order-1",
				InstrumentID:   "AAPL",
				Side:           domain.SideBuy,
				Type:           domain.OrderTypeMarket,
				Quantity:       decimal.NewFromInt(10),
				Price:          decimal.Zero,
				FilledQuantity: decimal.NewFromInt(10),
				AveragePrice:   decimal.NewFromFloat(150.50),
				Status:         domain.OrderStatusFilled,
				CreatedAt:      time.Now(),
				UpdatedAt:      time.Now(),
			},
		},
		fills: []domain.Fill{
			{
				ID:       "fill-1",
				OrderID:  "order-1",
				Quantity: decimal.NewFromInt(10),
				Price:    decimal.NewFromFloat(150.50),
			},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/order-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)
	if resp["id"] != "order-1" {
		t.Errorf("expected order-1, got %v", resp["id"])
	}

	fills, ok := resp["fills"].([]interface{})
	if !ok || len(fills) != 1 {
		t.Errorf("expected 1 fill, got %v", resp["fills"])
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	ms := &mockStore{}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/nonexistent", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestCORSHeaders(t *testing.T) {
	ms := &mockStore{}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	origin := rec.Header().Get("Access-Control-Allow-Origin")
	if origin != "http://localhost:3000" {
		t.Errorf("expected CORS origin http://localhost:3000, got %s", origin)
	}
}

func TestCorrelationIDHeader(t *testing.T) {
	ms := &mockStore{}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	corrID := rec.Header().Get("X-Correlation-ID")
	if corrID == "" {
		t.Error("expected X-Correlation-ID header to be set")
	}
}
