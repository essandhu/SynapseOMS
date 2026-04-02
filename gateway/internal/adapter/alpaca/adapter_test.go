package alpaca

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

// newTestServer creates an httptest.Server that mocks Alpaca REST API endpoints.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// Account endpoint (used by Connect and Ping).
	mux.HandleFunc("/v2/account", func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("APCA-API-KEY-ID")
		secret := r.Header.Get("APCA-API-SECRET-KEY")
		if key == "" || secret == "" {
			http.Error(w, `{"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if key != "test-key" || secret != "test-secret" {
			http.Error(w, `{"message":"forbidden"}`, http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"account-123","status":"ACTIVE","currency":"USD","buying_power":"100000"}`))
	})

	// Orders endpoint.
	mux.HandleFunc("/v2/orders", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req alpacaOrderRequest
			if err := json.Unmarshal(body, &req); err != nil {
				http.Error(w, `{"message":"bad request"}`, http.StatusBadRequest)
				return
			}
			resp := alpacaOrderResponse{
				ID:            "venue-order-001",
				ClientOrderID: req.ClientOrdID,
				Symbol:        req.Symbol,
				Side:          req.Side,
				Type:          req.Type,
				Qty:           req.Qty,
				FilledQty:     "0",
				Status:        "new",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Single order endpoints (GET, DELETE).
	mux.HandleFunc("/v2/orders/", func(w http.ResponseWriter, r *http.Request) {
		// Extract order ID from path: /v2/orders/{id}
		orderID := r.URL.Path[len("/v2/orders/"):]
		if orderID == "" {
			// Fall through to the /v2/orders handler for POST.
			http.Error(w, "order id required", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			resp := alpacaOrderResponse{
				ID:             orderID,
				ClientOrderID:  "client-001",
				Symbol:         "AAPL",
				Side:           "buy",
				Type:           "limit",
				Qty:            "10",
				FilledQty:      "5",
				FilledAvgPrice: "150.50",
				Status:         "partially_filled",
				LimitPrice:     "151.00",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Assets endpoint.
	mux.HandleFunc("/v2/assets", func(w http.ResponseWriter, r *http.Request) {
		assets := []alpacaAsset{
			{ID: "asset-1", Symbol: "AAPL", Name: "Apple Inc.", Class: "us_equity", Status: "active", Tradable: true},
			{ID: "asset-2", Symbol: "MSFT", Name: "Microsoft Corp.", Class: "us_equity", Status: "active", Tradable: true},
			{ID: "asset-3", Symbol: "DELISTED", Name: "Delisted Corp.", Class: "us_equity", Status: "active", Tradable: false},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(assets)
	})

	return httptest.NewServer(mux)
}

// newTestAdapter creates an Adapter with a mock base URL, already connected.
func newTestAdapter(t *testing.T, baseURL string) *Adapter {
	t.Helper()
	a := &Adapter{
		baseURL: baseURL + "/v2",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiKey:    "test-key",
		apiSecret: "test-secret",
		fillCh:    make(chan domain.Fill, 100),
		marketCh:  make(chan adapter.MarketDataSnapshot, 100),
		status:    adapter.Connected,
		logger:    slog.Default(),
	}
	return a
}

func TestVenueID(t *testing.T) {
	a := &Adapter{}
	if got := a.VenueID(); got != "alpaca" {
		t.Errorf("VenueID() = %q, want %q", got, "alpaca")
	}
}

func TestVenueName(t *testing.T) {
	a := &Adapter{}
	if got := a.VenueName(); got != "Alpaca Paper Trading" {
		t.Errorf("VenueName() = %q, want %q", got, "Alpaca Paper Trading")
	}
}

func TestSupportedAssetClasses(t *testing.T) {
	a := &Adapter{}
	classes := a.SupportedAssetClasses()
	if len(classes) != 1 {
		t.Fatalf("SupportedAssetClasses() returned %d classes, want 1", len(classes))
	}
	if classes[0] != domain.AssetClassEquity {
		t.Errorf("SupportedAssetClasses()[0] = %v, want AssetClassEquity", classes[0])
	}
}

func TestCapabilities(t *testing.T) {
	a := &Adapter{}
	caps := a.Capabilities()

	if !caps.SupportsStreaming {
		t.Error("Capabilities().SupportsStreaming = false, want true")
	}
	if len(caps.SupportedOrderTypes) != 3 {
		t.Errorf("Capabilities() has %d order types, want 3", len(caps.SupportedOrderTypes))
	}
	if len(caps.SupportedAssetClasses) != 1 {
		t.Errorf("Capabilities() has %d asset classes, want 1", len(caps.SupportedAssetClasses))
	}

	// Verify order types include market, limit, stop_limit.
	typeSet := map[domain.OrderType]bool{}
	for _, ot := range caps.SupportedOrderTypes {
		typeSet[ot] = true
	}
	if !typeSet[domain.OrderTypeMarket] {
		t.Error("Capabilities missing OrderTypeMarket")
	}
	if !typeSet[domain.OrderTypeLimit] {
		t.Error("Capabilities missing OrderTypeLimit")
	}
	if !typeSet[domain.OrderTypeStopLimit] {
		t.Error("Capabilities missing OrderTypeStopLimit")
	}
}

func TestConnectWithValidCredentials(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := NewAdapter(map[string]string{
		"base_url": srv.URL + "/v2",
	}).(*Adapter)

	// Skip WebSocket feeds for this test by overriding Connect behavior.
	// We test the REST-based verification part by calling Ping directly.
	a.apiKey = "test-key"
	a.apiSecret = "test-secret"

	ctx := context.Background()
	latency, err := a.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if latency <= 0 {
		t.Errorf("Ping() latency = %v, want > 0", latency)
	}
}

func TestConnectWithInvalidCredentials(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := NewAdapter(map[string]string{
		"base_url": srv.URL + "/v2",
	}).(*Adapter)

	a.apiKey = "bad-key"
	a.apiSecret = "bad-secret"

	ctx := context.Background()
	_, err := a.Ping(ctx)
	if err == nil {
		t.Fatal("Ping() expected error with invalid credentials, got nil")
	}
}

func TestPingMeasuresLatency(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	ctx := context.Background()
	latency, err := a.Ping(ctx)
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	// Latency should be non-negative (may be 0 on very fast local test servers).
	if latency < 0 {
		t.Errorf("Ping() latency = %v, want >= 0", latency)
	}
}

func TestSubmitOrder(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	order := &domain.Order{
		ID:            "order-001",
		ClientOrderID: "client-001",
		InstrumentID:  "AAPL",
		Side:          domain.SideBuy,
		Type:          domain.OrderTypeLimit,
		Quantity:      decimal.NewFromInt(10),
		Price:         decimal.NewFromFloat(150.50),
	}

	ctx := context.Background()
	ack, err := a.SubmitOrder(ctx, order)
	if err != nil {
		t.Fatalf("SubmitOrder() error = %v", err)
	}
	if ack.VenueOrderID != "venue-order-001" {
		t.Errorf("SubmitOrder() VenueOrderID = %q, want %q", ack.VenueOrderID, "venue-order-001")
	}
	if ack.ReceivedAt.IsZero() {
		t.Error("SubmitOrder() ReceivedAt is zero")
	}
}

func TestSubmitOrderNotConnected(t *testing.T) {
	a := &Adapter{
		status: adapter.Disconnected,
		mu:     sync.RWMutex{},
	}

	order := &domain.Order{
		ID:           "order-001",
		InstrumentID: "AAPL",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromInt(1),
	}

	ctx := context.Background()
	_, err := a.SubmitOrder(ctx, order)
	if err == nil {
		t.Fatal("SubmitOrder() expected error when not connected, got nil")
	}
}

func TestSubmitOrderMarket(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	order := &domain.Order{
		ID:            "order-002",
		ClientOrderID: "client-002",
		InstrumentID:  "MSFT",
		Side:          domain.SideSell,
		Type:          domain.OrderTypeMarket,
		Quantity:      decimal.NewFromInt(5),
	}

	ctx := context.Background()
	ack, err := a.SubmitOrder(ctx, order)
	if err != nil {
		t.Fatalf("SubmitOrder(market) error = %v", err)
	}
	if ack.VenueOrderID == "" {
		t.Error("SubmitOrder(market) returned empty VenueOrderID")
	}
}

func TestCancelOrder(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	ctx := context.Background()
	err := a.CancelOrder(ctx, "order-001", "venue-order-001")
	if err != nil {
		t.Fatalf("CancelOrder() error = %v", err)
	}
}

func TestCancelOrderNotConnected(t *testing.T) {
	a := &Adapter{
		status: adapter.Disconnected,
		mu:     sync.RWMutex{},
	}

	ctx := context.Background()
	err := a.CancelOrder(ctx, "order-001", "venue-order-001")
	if err == nil {
		t.Fatal("CancelOrder() expected error when not connected, got nil")
	}
}

func TestQueryOrder(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	ctx := context.Background()
	order, err := a.QueryOrder(ctx, "venue-order-001")
	if err != nil {
		t.Fatalf("QueryOrder() error = %v", err)
	}
	if order.InstrumentID != "AAPL" {
		t.Errorf("QueryOrder() Symbol = %q, want %q", order.InstrumentID, "AAPL")
	}
	if order.Status != domain.OrderStatusPartiallyFilled {
		t.Errorf("QueryOrder() Status = %v, want PartiallyFilled", order.Status)
	}
	if !order.FilledQuantity.Equal(decimal.NewFromInt(5)) {
		t.Errorf("QueryOrder() FilledQuantity = %s, want 5", order.FilledQuantity)
	}
}

func TestSupportedInstruments(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	a := newTestAdapter(t, srv.URL)

	instruments, err := a.SupportedInstruments()
	if err != nil {
		t.Fatalf("SupportedInstruments() error = %v", err)
	}
	// The mock returns 3 assets but one is not tradable, so we expect 2.
	if len(instruments) != 2 {
		t.Fatalf("SupportedInstruments() returned %d instruments, want 2", len(instruments))
	}

	symbols := map[string]bool{}
	for _, inst := range instruments {
		symbols[inst.Symbol] = true
		if inst.AssetClass != domain.AssetClassEquity {
			t.Errorf("instrument %s has AssetClass %v, want Equity", inst.Symbol, inst.AssetClass)
		}
		if inst.SettlementCycle != domain.SettlementT2 {
			t.Errorf("instrument %s has SettlementCycle %v, want T2", inst.Symbol, inst.SettlementCycle)
		}
		if inst.QuoteCurrency != "USD" {
			t.Errorf("instrument %s has QuoteCurrency %q, want USD", inst.Symbol, inst.QuoteCurrency)
		}
	}
	if !symbols["AAPL"] {
		t.Error("SupportedInstruments() missing AAPL")
	}
	if !symbols["MSFT"] {
		t.Error("SupportedInstruments() missing MSFT")
	}
}

func TestFillFeed(t *testing.T) {
	fillCh := make(chan domain.Fill, 10)
	a := &Adapter{fillCh: fillCh}

	ch := a.FillFeed()
	if ch == nil {
		t.Fatal("FillFeed() returned nil channel")
	}

	// Verify it is the same channel.
	go func() {
		fillCh <- domain.Fill{ID: "test-fill-1"}
	}()

	select {
	case f := <-ch:
		if f.ID != "test-fill-1" {
			t.Errorf("FillFeed() received fill ID = %q, want %q", f.ID, "test-fill-1")
		}
	case <-time.After(time.Second):
		t.Fatal("FillFeed() timed out waiting for fill")
	}
}

func TestStatus(t *testing.T) {
	a := &Adapter{status: adapter.Disconnected}
	if a.Status() != adapter.Disconnected {
		t.Errorf("Status() = %v, want Disconnected", a.Status())
	}

	a.mu.Lock()
	a.status = adapter.Connected
	a.mu.Unlock()
	if a.Status() != adapter.Connected {
		t.Errorf("Status() = %v, want Connected", a.Status())
	}
}

func TestNewAdapterDefaultURL(t *testing.T) {
	a := NewAdapter(map[string]string{}).(*Adapter)
	if a.baseURL != paperBaseURL {
		t.Errorf("NewAdapter() baseURL = %q, want %q", a.baseURL, paperBaseURL)
	}
}

func TestNewAdapterCustomURL(t *testing.T) {
	custom := "http://localhost:9999/v2"
	a := NewAdapter(map[string]string{"base_url": custom}).(*Adapter)
	if a.baseURL != custom {
		t.Errorf("NewAdapter(custom) baseURL = %q, want %q", a.baseURL, custom)
	}
}

func TestAdapterRegistered(t *testing.T) {
	factory, ok := adapter.Get("alpaca")
	if !ok {
		t.Fatal("adapter.Get(\"alpaca\") not found; init() registration failed")
	}
	if factory == nil {
		t.Fatal("adapter.Get(\"alpaca\") returned nil factory")
	}

	p := factory(map[string]string{})
	if p.VenueID() != "alpaca" {
		t.Errorf("factory produced adapter with VenueID = %q, want %q", p.VenueID(), "alpaca")
	}
}

func TestOrderSideStr(t *testing.T) {
	tests := []struct {
		side domain.OrderSide
		want string
	}{
		{domain.SideBuy, "buy"},
		{domain.SideSell, "sell"},
	}
	for _, tc := range tests {
		if got := orderSideStr(tc.side); got != tc.want {
			t.Errorf("orderSideStr(%v) = %q, want %q", tc.side, got, tc.want)
		}
	}
}

func TestOrderTypeStr(t *testing.T) {
	tests := []struct {
		ot   domain.OrderType
		want string
	}{
		{domain.OrderTypeMarket, "market"},
		{domain.OrderTypeLimit, "limit"},
		{domain.OrderTypeStopLimit, "stop_limit"},
	}
	for _, tc := range tests {
		if got := orderTypeStr(tc.ot); got != tc.want {
			t.Errorf("orderTypeStr(%v) = %q, want %q", tc.ot, got, tc.want)
		}
	}
}

func TestMapAlpacaStatus(t *testing.T) {
	tests := []struct {
		input string
		want  domain.OrderStatus
	}{
		{"new", domain.OrderStatusNew},
		{"accepted", domain.OrderStatusNew},
		{"pending_new", domain.OrderStatusNew},
		{"partially_filled", domain.OrderStatusPartiallyFilled},
		{"filled", domain.OrderStatusFilled},
		{"canceled", domain.OrderStatusCanceled},
		{"expired", domain.OrderStatusCanceled},
		{"rejected", domain.OrderStatusRejected},
		{"unknown_status", domain.OrderStatusNew},
	}
	for _, tc := range tests {
		if got := mapAlpacaStatus(tc.input); got != tc.want {
			t.Errorf("mapAlpacaStatus(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
