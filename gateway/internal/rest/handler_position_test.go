package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

func TestListPositions_ReturnsPositions(t *testing.T) {
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	ms := &mockStore{
		positions: []domain.Position{
			{
				InstrumentID:      "BTC-USD",
				VenueID:           "binance",
				Quantity:          decimal.NewFromFloat(1.5),
				AverageCost:       decimal.NewFromFloat(42000.00),
				MarketPrice:       decimal.NewFromFloat(43500.00),
				UnrealizedPnL:     decimal.NewFromFloat(2250.00),
				RealizedPnL:       decimal.Zero,
				UnsettledQuantity: decimal.Zero,
				SettledQuantity:   decimal.NewFromFloat(1.5),
				UpdatedAt:         now,
			},
			{
				InstrumentID:      "ETH-USD",
				VenueID:           "binance",
				Quantity:          decimal.NewFromFloat(10),
				AverageCost:       decimal.NewFromFloat(2200),
				MarketPrice:       decimal.NewFromFloat(2350),
				UnrealizedPnL:     decimal.NewFromFloat(1500),
				RealizedPnL:       decimal.NewFromFloat(500),
				UnsettledQuantity: decimal.Zero,
				SettledQuantity:   decimal.NewFromFloat(10),
				UpdatedAt:         now,
			},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var positions []map[string]interface{}
	decodeJSON(t, rec.Result(), &positions)

	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}

	if positions[0]["instrument_id"] != "BTC-USD" {
		t.Errorf("expected instrument_id 'BTC-USD', got %v", positions[0]["instrument_id"])
	}
	if positions[0]["venue_id"] != "binance" {
		t.Errorf("expected venue_id 'binance', got %v", positions[0]["venue_id"])
	}
	// Decimal fields should be serialized as strings.
	if positions[0]["quantity"] != "1.5" {
		t.Errorf("expected quantity '1.5', got %v", positions[0]["quantity"])
	}
}

func TestListPositions_EmptyList(t *testing.T) {
	ms := &mockStore{
		positions: []domain.Position{},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var positions []map[string]interface{}
	decodeJSON(t, rec.Result(), &positions)

	if len(positions) != 0 {
		t.Fatalf("expected 0 positions, got %d", len(positions))
	}
}

func TestGetPosition_Found(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	ms := &mockStore{
		positions: []domain.Position{
			{
				InstrumentID:      "AAPL",
				VenueID:           "alpaca",
				Quantity:          decimal.NewFromInt(100),
				AverageCost:       decimal.NewFromFloat(150.25),
				MarketPrice:       decimal.NewFromFloat(155.00),
				UnrealizedPnL:     decimal.NewFromFloat(475.00),
				RealizedPnL:       decimal.Zero,
				UnsettledQuantity: decimal.Zero,
				SettledQuantity:   decimal.NewFromInt(100),
				UpdatedAt:         now,
			},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions/AAPL", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	decodeJSON(t, rec.Result(), &resp)

	if resp["instrument_id"] != "AAPL" {
		t.Errorf("expected instrument_id 'AAPL', got %v", resp["instrument_id"])
	}
	if resp["venue_id"] != "alpaca" {
		t.Errorf("expected venue_id 'alpaca', got %v", resp["venue_id"])
	}
	if resp["quantity"] != "100" {
		t.Errorf("expected quantity '100', got %v", resp["quantity"])
	}
}

func TestGetPosition_NotFound(t *testing.T) {
	ms := &mockStore{
		positions: []domain.Position{},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/positions/NONEXISTENT", nil)
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

	if errResp.Error.Code != "POSITION_NOT_FOUND" {
		t.Errorf("expected error code POSITION_NOT_FOUND, got %s", errResp.Error.Code)
	}
}
