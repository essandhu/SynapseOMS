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
	if btc["assetClass"] != "crypto" {
		t.Errorf("expected assetClass 'crypto', got %v", btc["assetClass"])
	}
	if btc["quoteCurrency"] != "USD" {
		t.Errorf("expected quoteCurrency 'USD', got %v", btc["quoteCurrency"])
	}
	if btc["baseCurrency"] != "BTC" {
		t.Errorf("expected baseCurrency 'BTC', got %v", btc["baseCurrency"])
	}
	if btc["settlementCycle"] != "T+0" {
		t.Errorf("expected settlementCycle 'T+0', got %v", btc["settlementCycle"])
	}

	// Check decimal fields are serialized as strings.
	if btc["tickSize"] != "0.01" {
		t.Errorf("expected tickSize '0.01', got %v", btc["tickSize"])
	}

	// Check venues array.
	venues, ok := btc["venues"].([]interface{})
	if !ok || len(venues) != 2 {
		t.Errorf("expected 2 venues, got %v", btc["venues"])
	}

	// Check venueId is populated from first venue.
	if btc["venueId"] != "binance" {
		t.Errorf("expected venueId 'binance', got %v", btc["venueId"])
	}

	// Check second instrument (equity).
	aapl := instruments[1]
	if aapl["assetClass"] != "equity" {
		t.Errorf("expected assetClass 'equity', got %v", aapl["assetClass"])
	}
	if aapl["settlementCycle"] != "T+2" {
		t.Errorf("expected settlementCycle 'T+2', got %v", aapl["settlementCycle"])
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
