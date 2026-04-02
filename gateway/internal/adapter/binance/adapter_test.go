package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

const (
	testAPIKey    = "test-api-key-12345"
	testAPISecret = "test-api-secret-67890"
)

// newTestAdapter creates an Adapter pointed at the given test server URL.
func newTestAdapter(baseURL string) *Adapter {
	a := NewAdapter(nil).(*Adapter)
	a.baseURL = baseURL
	a.apiKey = testAPIKey
	a.apiSecret = testAPISecret
	return a
}

// ------------------------------------------------------------------
// Test: Symbol mapping
// ------------------------------------------------------------------

func TestSymbolMapping(t *testing.T) {
	tests := []struct {
		internal string
		binance  string
	}{
		{"BTC-USD", "BTCUSDT"},
		{"ETH-USD", "ETHUSDT"},
		{"SOL-USD", "SOLUSDT"},
	}

	for _, tt := range tests {
		sym, err := ToSymbol(tt.internal)
		if err != nil {
			t.Fatalf("ToSymbol(%q) unexpected error: %v", tt.internal, err)
		}
		if sym != tt.binance {
			t.Errorf("ToSymbol(%q) = %q, want %q", tt.internal, sym, tt.binance)
		}

		id, err := FromSymbol(tt.binance)
		if err != nil {
			t.Fatalf("FromSymbol(%q) unexpected error: %v", tt.binance, err)
		}
		if id != tt.internal {
			t.Errorf("FromSymbol(%q) = %q, want %q", tt.binance, id, tt.internal)
		}
	}
}

func TestSymbolMapping_Unknown(t *testing.T) {
	_, err := ToSymbol("DOGE-USD")
	if err == nil {
		t.Error("ToSymbol(DOGE-USD) expected error, got nil")
	}

	_, err = FromSymbol("DOGEUSDT")
	if err == nil {
		t.Error("FromSymbol(DOGEUSDT) expected error, got nil")
	}
}

// ------------------------------------------------------------------
// Test: HMAC-SHA256 signature
// ------------------------------------------------------------------

func TestSign(t *testing.T) {
	a := &Adapter{apiSecret: testAPISecret}

	queryString := "symbol=BTCUSDT&side=BUY&type=MARKET&quantity=0.01&timestamp=1234567890"

	// Compute expected signature.
	mac := hmac.New(sha256.New, []byte(testAPISecret))
	mac.Write([]byte(queryString))
	expected := hex.EncodeToString(mac.Sum(nil))

	got := a.sign(queryString)
	if got != expected {
		t.Errorf("sign() = %q, want %q", got, expected)
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	a1 := &Adapter{apiSecret: "secret1"}
	a2 := &Adapter{apiSecret: "secret2"}

	qs := "symbol=ETHUSDT&timestamp=1234567890"
	sig1 := a1.sign(qs)
	sig2 := a2.sign(qs)

	if sig1 == sig2 {
		t.Error("signatures with different secrets should differ")
	}
}

// ------------------------------------------------------------------
// Test: Ping
// ------------------------------------------------------------------

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	latency, err := a.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping() unexpected error: %v", err)
	}
	if latency <= 0 {
		t.Errorf("Ping() latency = %v, want > 0", latency)
	}
}

func TestPing_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	_, err := a.Ping(context.Background())
	if err == nil {
		t.Error("Ping() expected error on 500 response, got nil")
	}
}

// ------------------------------------------------------------------
// Test: Connect (listen key creation)
// ------------------------------------------------------------------

func TestConnect(t *testing.T) {
	var receivedAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/userDataStream":
			receivedAPIKey = r.Header.Get("X-MBX-APIKEY")
			w.WriteHeader(http.StatusOK)
			resp := map[string]string{"listenKey": "test-listen-key-abc"}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	a := NewAdapter(nil).(*Adapter)
	a.baseURL = server.URL
	// Override wsURL to something that won't actually connect.
	// We mock just the REST part; the WS dial will fail but we handle that.
	a.wsURL = "ws://127.0.0.1:1" // unreachable

	cred := domain.VenueCredential{
		VenueID:   venueID,
		APIKey:    testAPIKey,
		APISecret: testAPISecret,
	}

	// Connect will succeed creating the listen key but fail on WS dial.
	// That's expected in tests without a real WS server.
	err := a.Connect(context.Background(), cred)
	// We expect a WS connection error since we can't dial the fake address.
	if err == nil {
		// If it somehow connected, verify the listen key was set.
		if a.listenKey != "test-listen-key-abc" {
			t.Errorf("listenKey = %q, want %q", a.listenKey, "test-listen-key-abc")
		}
		_ = a.Disconnect(context.Background())
	} else {
		// The error should be about the WS connection, not the listen key creation.
		if !strings.Contains(err.Error(), "user data feed") {
			t.Fatalf("Connect() unexpected error type: %v", err)
		}
		// Verify the listen key was obtained before the WS failure.
		if a.listenKey != "test-listen-key-abc" {
			t.Errorf("listenKey = %q, want %q", a.listenKey, "test-listen-key-abc")
		}
	}

	if receivedAPIKey != testAPIKey {
		t.Errorf("API key header = %q, want %q", receivedAPIKey, testAPIKey)
	}
}

func TestConnect_ListenKeyFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":-2015,"msg":"Invalid API-key"}`))
	}))
	defer server.Close()

	a := NewAdapter(nil).(*Adapter)
	a.baseURL = server.URL

	cred := domain.VenueCredential{
		VenueID:   venueID,
		APIKey:    "bad-key",
		APISecret: "bad-secret",
	}

	err := a.Connect(context.Background(), cred)
	if err == nil {
		t.Fatal("Connect() expected error with bad credentials, got nil")
	}
	if !strings.Contains(err.Error(), "create listen key") {
		t.Errorf("error should mention listen key creation: %v", err)
	}
}

// ------------------------------------------------------------------
// Test: SubmitOrder (signed request)
// ------------------------------------------------------------------

func TestSubmitOrder(t *testing.T) {
	var (
		receivedSymbol string
		receivedSide   string
		receivedType   string
		receivedQty    string
		receivedSig    string
		receivedTS     string
		receivedKey    string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/order" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		q := r.URL.Query()
		receivedSymbol = q.Get("symbol")
		receivedSide = q.Get("side")
		receivedType = q.Get("type")
		receivedQty = q.Get("quantity")
		receivedSig = q.Get("signature")
		receivedTS = q.Get("timestamp")
		receivedKey = r.Header.Get("X-MBX-APIKEY")

		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"orderId": 12345,
			"status":  "NEW",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	a.status = adapter.Connected

	order := &domain.Order{
		ID:           "test-order-1",
		InstrumentID: "BTC-USD",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromFloat(0.01),
	}

	ack, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() unexpected error: %v", err)
	}

	if ack.VenueOrderID != "12345" {
		t.Errorf("VenueOrderID = %q, want %q", ack.VenueOrderID, "12345")
	}
	if ack.ReceivedAt.IsZero() {
		t.Error("ReceivedAt should not be zero")
	}

	// Verify request params.
	if receivedSymbol != "BTCUSDT" {
		t.Errorf("symbol = %q, want BTCUSDT", receivedSymbol)
	}
	if receivedSide != "BUY" {
		t.Errorf("side = %q, want BUY", receivedSide)
	}
	if receivedType != "MARKET" {
		t.Errorf("type = %q, want MARKET", receivedType)
	}
	if receivedQty != "0.01" {
		t.Errorf("quantity = %q, want 0.01", receivedQty)
	}
	if receivedKey != testAPIKey {
		t.Errorf("API key = %q, want %q", receivedKey, testAPIKey)
	}
	if receivedTS == "" {
		t.Error("timestamp should not be empty")
	}
	if receivedSig == "" {
		t.Error("signature should not be empty")
	}
}

func TestSubmitOrder_LimitOrder(t *testing.T) {
	var (
		receivedPrice string
		receivedTIF   string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		receivedPrice = q.Get("price")
		receivedTIF = q.Get("timeInForce")

		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"orderId": 67890,
			"status":  "NEW",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	a.status = adapter.Connected

	order := &domain.Order{
		ID:           "test-limit-1",
		InstrumentID: "ETH-USD",
		Side:         domain.SideSell,
		Type:         domain.OrderTypeLimit,
		Quantity:     decimal.NewFromFloat(1.5),
		Price:        decimal.NewFromFloat(3500.50),
	}

	ack, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() unexpected error: %v", err)
	}
	if ack.VenueOrderID != "67890" {
		t.Errorf("VenueOrderID = %q, want %q", ack.VenueOrderID, "67890")
	}
	if receivedPrice != "3500.5" {
		t.Errorf("price = %q, want 3500.5", receivedPrice)
	}
	if receivedTIF != "GTC" {
		t.Errorf("timeInForce = %q, want GTC", receivedTIF)
	}
}

func TestSubmitOrder_NotConnected(t *testing.T) {
	a := newTestAdapter("http://localhost:1")
	a.status = adapter.Disconnected

	order := &domain.Order{
		ID:           "test-order-2",
		InstrumentID: "BTC-USD",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromFloat(0.01),
	}

	_, err := a.SubmitOrder(context.Background(), order)
	if err == nil {
		t.Error("SubmitOrder() expected error when disconnected, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention not connected: %v", err)
	}
}

func TestSubmitOrder_UnsupportedInstrument(t *testing.T) {
	a := newTestAdapter("http://localhost:1")
	a.status = adapter.Connected

	order := &domain.Order{
		ID:           "test-order-3",
		InstrumentID: "DOGE-USD",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromFloat(100),
	}

	_, err := a.SubmitOrder(context.Background(), order)
	if err == nil {
		t.Error("SubmitOrder() expected error for unsupported instrument, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported instrument") {
		t.Errorf("error should mention unsupported instrument: %v", err)
	}
}

// ------------------------------------------------------------------
// Test: SubmitOrder signature verification
// ------------------------------------------------------------------

func TestSubmitOrder_SignatureIsValid(t *testing.T) {
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery

		w.WriteHeader(http.StatusOK)
		resp := map[string]interface{}{
			"orderId": 99999,
			"status":  "NEW",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	a.status = adapter.Connected

	order := &domain.Order{
		ID:           "sig-test-1",
		InstrumentID: "SOL-USD",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromFloat(10),
	}

	_, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() unexpected error: %v", err)
	}

	// Parse the query string to extract the signature and reconstruct the
	// pre-signature query string. The adapter builds params with url.Values
	// then appends the signature, so we need to extract the signature value
	// and verify it against the query string that was signed (everything
	// before adding the signature key).
	parsedQuery, err := url.ParseQuery(capturedQuery)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}

	receivedSig := parsedQuery.Get("signature")
	if receivedSig == "" {
		t.Fatal("signature not found in query")
	}

	// Remove signature from params to reconstruct what was signed.
	parsedQuery.Del("signature")
	queryWithoutSig := parsedQuery.Encode()

	// Recompute expected signature.
	mac := hmac.New(sha256.New, []byte(testAPISecret))
	mac.Write([]byte(queryWithoutSig))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if receivedSig != expectedSig {
		t.Errorf("signature mismatch:\n  got:  %s\n  want: %s\n  query: %s", receivedSig, expectedSig, queryWithoutSig)
	}
}

// ------------------------------------------------------------------
// Test: VenueID, VenueName, Capabilities, SupportedAssetClasses
// ------------------------------------------------------------------

func TestAdapterMetadata(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)

	if a.VenueID() != "binance_testnet" {
		t.Errorf("VenueID() = %q, want %q", a.VenueID(), "binance_testnet")
	}
	if a.VenueName() != "Binance Testnet" {
		t.Errorf("VenueName() = %q, want %q", a.VenueName(), "Binance Testnet")
	}

	classes := a.SupportedAssetClasses()
	if len(classes) != 1 || classes[0] != domain.AssetClassCrypto {
		t.Errorf("SupportedAssetClasses() = %v, want [AssetClassCrypto]", classes)
	}

	caps := a.Capabilities()
	if !caps.SupportsStreaming {
		t.Error("Capabilities().SupportsStreaming should be true")
	}
	if len(caps.SupportedOrderTypes) != 2 {
		t.Errorf("expected 2 supported order types, got %d", len(caps.SupportedOrderTypes))
	}
}

func TestSupportedInstruments(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)
	instruments, err := a.SupportedInstruments()
	if err != nil {
		t.Fatalf("SupportedInstruments() unexpected error: %v", err)
	}
	if len(instruments) != 3 {
		t.Fatalf("expected 3 instruments, got %d", len(instruments))
	}

	ids := make(map[string]bool)
	for _, inst := range instruments {
		ids[inst.ID] = true
		if inst.AssetClass != domain.AssetClassCrypto {
			t.Errorf("instrument %s: AssetClass = %v, want Crypto", inst.ID, inst.AssetClass)
		}
		if inst.SettlementCycle != domain.SettlementT0 {
			t.Errorf("instrument %s: SettlementCycle = %v, want T0", inst.ID, inst.SettlementCycle)
		}
		if !inst.TradingHours.Is24x7 {
			t.Errorf("instrument %s: TradingHours.Is24x7 should be true", inst.ID)
		}
	}

	for _, expected := range []string{"BTC-USD", "ETH-USD", "SOL-USD"} {
		if !ids[expected] {
			t.Errorf("missing instrument %s", expected)
		}
	}
}

// ------------------------------------------------------------------
// Test: Status
// ------------------------------------------------------------------

func TestStatus_DefaultDisconnected(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)
	if a.Status() != adapter.Disconnected {
		t.Errorf("Status() = %v, want Disconnected", a.Status())
	}
}

// ------------------------------------------------------------------
// Test: FillFeed returns channel
// ------------------------------------------------------------------

func TestFillFeed(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)
	ch := a.FillFeed()
	if ch == nil {
		t.Error("FillFeed() returned nil channel")
	}
}

// ------------------------------------------------------------------
// Test: init() registration
// ------------------------------------------------------------------

func TestRegistration(t *testing.T) {
	// The init() function in adapter.go registers "binance_testnet".
	// Verify by creating from the registry.
	factory, ok := adapter.Get("binance_testnet")
	if !ok || factory == nil {
		t.Fatal("binance_testnet not registered in adapter registry")
	}

	p := factory(nil)
	if p.VenueID() != "binance_testnet" {
		t.Errorf("factory created adapter with VenueID = %q, want %q", p.VenueID(), "binance_testnet")
	}
}

// ------------------------------------------------------------------
// Test: CancelOrder
// ------------------------------------------------------------------

func TestCancelOrder(t *testing.T) {
	var receivedMethod string
	var receivedOrderID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/order" {
			receivedMethod = r.Method
			receivedOrderID = r.URL.Query().Get("orderId")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)
	a.status = adapter.Connected

	err := a.CancelOrder(context.Background(), "test-order-1", "12345")
	if err != nil {
		t.Fatalf("CancelOrder() unexpected error: %v", err)
	}
	if receivedMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", receivedMethod)
	}
	if receivedOrderID != "12345" {
		t.Errorf("orderId = %q, want 12345", receivedOrderID)
	}
}

// ------------------------------------------------------------------
// Test: keepAliveListenKey
// ------------------------------------------------------------------

func TestKeepAliveListenKey(t *testing.T) {
	var receivedMethod string
	var receivedListenKey string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/userDataStream" {
			receivedMethod = r.Method
			receivedListenKey = r.URL.Query().Get("listenKey")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	err := a.keepAliveListenKey(context.Background(), "my-listen-key")
	if err != nil {
		t.Fatalf("keepAliveListenKey() unexpected error: %v", err)
	}
	if receivedMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", receivedMethod)
	}
	if receivedListenKey != "my-listen-key" {
		t.Errorf("listenKey = %q, want my-listen-key", receivedListenKey)
	}
}

// ------------------------------------------------------------------
// Test: mapBinanceStatus
// ------------------------------------------------------------------

func TestMapBinanceStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.OrderStatus
	}{
		{"NEW", domain.OrderStatusNew},
		{"PARTIALLY_FILLED", domain.OrderStatusPartiallyFilled},
		{"FILLED", domain.OrderStatusFilled},
		{"CANCELED", domain.OrderStatusCanceled},
		{"EXPIRED", domain.OrderStatusCanceled},
		{"REJECTED", domain.OrderStatusCanceled},
		{"UNKNOWN", domain.OrderStatusNew},
	}

	for _, tt := range tests {
		got := mapBinanceStatus(tt.input)
		if got != tt.expected {
			t.Errorf("mapBinanceStatus(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// ------------------------------------------------------------------
// Test: Disconnect
// ------------------------------------------------------------------

func TestDisconnect(t *testing.T) {
	a := NewAdapter(nil).(*Adapter)
	// Should not panic even when not connected.
	err := a.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("Disconnect() unexpected error: %v", err)
	}
	if a.Status() != adapter.Disconnected {
		t.Errorf("Status() = %v, want Disconnected", a.Status())
	}
}

// ------------------------------------------------------------------
// Test: CreateListenKey
// ------------------------------------------------------------------

func TestCreateListenKey(t *testing.T) {
	expectedKey := "pqia91ma19a5s61cv6a81va65sdf19v8a65a1a5s61cv6a81va65sdf19v8a65a1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/userDataStream" && r.Method == http.MethodPost {
			if r.Header.Get("X-MBX-APIKEY") == "" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			resp := map[string]string{"listenKey": expectedKey}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	a := newTestAdapter(server.URL)

	key, err := a.createListenKey(context.Background())
	if err != nil {
		t.Fatalf("createListenKey() unexpected error: %v", err)
	}
	if key != expectedKey {
		t.Errorf("listenKey = %q, want %q", key, expectedKey)
	}
}

// Ensure the test takes no time-related imports without use.
var _ = time.Now
