package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

func TestListInstruments_ReturnsInstruments(t *testing.T) {
	ms := &mockStore{
		instruments: []domain.Instrument{
			{
				ID:              "BTC-USD",
				Symbol:          "BTC-USD",
				Name:            "Bitcoin / US Dollar",
				AssetClass:      domain.AssetClassCrypto,
				QuoteCurrency:   "USD",
				BaseCurrency:    "BTC",
				TickSize:        decimal.NewFromFloat(0.01),
				LotSize:         decimal.NewFromFloat(0.00001),
				SettlementCycle: domain.SettlementT0,
				Venues:          []string{"binance", "coinbase"},
				MarginRequired:  decimal.NewFromFloat(0.1),
			},
			{
				ID:              "AAPL",
				Symbol:          "AAPL",
				Name:            "Apple Inc",
				AssetClass:      domain.AssetClassEquity,
				QuoteCurrency:   "USD",
				TickSize:        decimal.NewFromFloat(0.01),
				LotSize:         decimal.NewFromInt(1),
				SettlementCycle: domain.SettlementT2,
				Venues:          []string{"alpaca"},
				MarginRequired:  decimal.NewFromFloat(0.25),
			},
		},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instruments", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var instruments []map[string]interface{}
	decodeJSON(t, rec.Result(), &instruments)

	if len(instruments) != 2 {
		t.Fatalf("expected 2 instruments, got %d", len(instruments))
	}

	// Check first instrument (crypto).
	btc := instruments[0]
	if btc["id"] != "BTC-USD" {
		t.Errorf("expected id 'BTC-USD', got %v", btc["id"])
	}
	if btc["symbol"] != "BTC-USD" {
		t.Errorf("expected symbol 'BTC-USD', got %v", btc["symbol"])
	}
	if btc["asset_class"] != "crypto" {
		t.Errorf("expected asset_class 'crypto', got %v", btc["asset_class"])
	}
	if btc["quote_currency"] != "USD" {
		t.Errorf("expected quote_currency 'USD', got %v", btc["quote_currency"])
	}
	if btc["base_currency"] != "BTC" {
		t.Errorf("expected base_currency 'BTC', got %v", btc["base_currency"])
	}
	if btc["settlement_cycle"] != "T+0" {
		t.Errorf("expected settlement_cycle 'T+0', got %v", btc["settlement_cycle"])
	}

	// Check decimal fields are serialized as strings.
	if btc["tick_size"] != "0.01" {
		t.Errorf("expected tick_size '0.01', got %v", btc["tick_size"])
	}

	// Check venues array.
	venues, ok := btc["venues"].([]interface{})
	if !ok || len(venues) != 2 {
		t.Errorf("expected 2 venues, got %v", btc["venues"])
	}

	// Check second instrument (equity).
	aapl := instruments[1]
	if aapl["asset_class"] != "equity" {
		t.Errorf("expected asset_class 'equity', got %v", aapl["asset_class"])
	}
	if aapl["settlement_cycle"] != "T+2" {
		t.Errorf("expected settlement_cycle 'T+2', got %v", aapl["settlement_cycle"])
	}
}

func TestListInstruments_EmptyList(t *testing.T) {
	ms := &mockStore{
		instruments: []domain.Instrument{},
	}
	mp := &mockPipeline{}
	router := setupRouter(ms, mp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/instruments", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var instruments []map[string]interface{}
	decodeJSON(t, rec.Result(), &instruments)

	if len(instruments) != 0 {
		t.Fatalf("expected 0 instruments, got %d", len(instruments))
	}
}
