package adapter_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/adapter/alpaca"
	"github.com/synapse-oms/gateway/internal/adapter/binance"
	"github.com/synapse-oms/gateway/internal/adapter/simulated"
)

// contractSuite runs shared contract tests against any LiquidityProvider.
func contractSuite(t *testing.T, provider adapter.LiquidityProvider) {
	t.Helper()

	t.Run("VenueID returns non-empty string", func(t *testing.T) {
		id := provider.VenueID()
		if id == "" {
			t.Error("VenueID() returned empty string")
		}
	})

	t.Run("VenueName returns non-empty string", func(t *testing.T) {
		name := provider.VenueName()
		if name == "" {
			t.Error("VenueName() returned empty string")
		}
	})

	t.Run("Status returns Disconnected before Connect", func(t *testing.T) {
		status := provider.Status()
		if status != adapter.Disconnected {
			t.Errorf("expected Disconnected, got %v", status)
		}
	})

	t.Run("SupportedInstruments returns at least one", func(t *testing.T) {
		instruments, err := provider.SupportedInstruments()
		if err != nil {
			t.Fatalf("SupportedInstruments() error: %v", err)
		}
		if len(instruments) == 0 {
			t.Error("SupportedInstruments() returned empty slice")
		}
	})

	t.Run("SupportedAssetClasses returns at least one", func(t *testing.T) {
		classes := provider.SupportedAssetClasses()
		if len(classes) == 0 {
			t.Error("SupportedAssetClasses() returned empty slice")
		}
	})

	t.Run("FillFeed returns non-nil channel", func(t *testing.T) {
		ch := provider.FillFeed()
		if ch == nil {
			t.Error("FillFeed() returned nil channel")
		}
	})

	t.Run("Capabilities returns valid capabilities", func(t *testing.T) {
		caps := provider.Capabilities()
		if len(caps.SupportedOrderTypes) == 0 {
			t.Error("no supported order types")
		}
		if len(caps.SupportedAssetClasses) == 0 {
			t.Error("no supported asset classes")
		}
	})
}

// ---------------------------------------------------------------------------
// Simulated adapter contract test
// ---------------------------------------------------------------------------

func TestSimulatedAdapterContract(t *testing.T) {
	a := simulated.NewAdapter(nil)
	contractSuite(t, a)
}

// ---------------------------------------------------------------------------
// Alpaca adapter contract test (mocked HTTP)
// ---------------------------------------------------------------------------

func newAlpacaMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// GET /v2/account — used by Ping / Connect verification.
	mux.HandleFunc("/v2/account", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"acct-1","status":"ACTIVE","currency":"USD","buying_power":"100000"}`))
	})

	// GET /v2/assets — used by SupportedInstruments.
	mux.HandleFunc("/v2/assets", func(w http.ResponseWriter, _ *http.Request) {
		type asset struct {
			ID       string `json:"id"`
			Symbol   string `json:"symbol"`
			Name     string `json:"name"`
			Class    string `json:"class"`
			Status   string `json:"status"`
			Tradable bool   `json:"tradable"`
		}
		assets := []asset{
			{ID: "a1", Symbol: "AAPL", Name: "Apple Inc.", Class: "us_equity", Status: "active", Tradable: true},
			{ID: "a2", Symbol: "MSFT", Name: "Microsoft Corp.", Class: "us_equity", Status: "active", Tradable: true},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(assets)
	})

	return httptest.NewServer(mux)
}

func TestAlpacaAdapterContract(t *testing.T) {
	srv := newAlpacaMockServer(t)
	defer srv.Close()

	// Create adapter with mock base URL. The Alpaca adapter accepts "base_url"
	// in its config map; its SupportedInstruments appends "/assets?..." to it.
	// The adapter's NewAdapter stores config["base_url"] as-is in a.baseURL,
	// and then calls base+"/assets?status=active&class=us_equity".
	// The mock server routes are mounted at /v2/assets, so we pass srv.URL+"/v2".
	a := alpaca.NewAdapter(map[string]string{
		"base_url": srv.URL + "/v2",
	})

	contractSuite(t, a)
}

// ---------------------------------------------------------------------------
// Binance adapter contract test (mocked HTTP)
// ---------------------------------------------------------------------------

func newBinanceMockServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	// GET /ping — used by Ping.
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	})

	// GET /exchangeInfo — included for completeness (not used by current
	// SupportedInstruments which is hardcoded, but future-proofs the mock).
	mux.HandleFunc("/exchangeInfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"symbols":[{"symbol":"BTCUSDT","status":"TRADING"},{"symbol":"ETHUSDT","status":"TRADING"},{"symbol":"SOLUSDT","status":"TRADING"}]}`))
	})

	return httptest.NewServer(mux)
}

func TestBinanceAdapterContract(t *testing.T) {
	srv := newBinanceMockServer(t)
	defer srv.Close()

	// The Binance NewAdapter does not accept a config map for base URL override,
	// so we create the adapter via NewAdapter then mutate the unexported baseURL
	// field. Since the Adapter type is in another package, we use the approach
	// from the existing binance tests: create via NewAdapter(nil) and then use
	// a wrapper that sets the base URL.
	//
	// However, the Binance SupportedInstruments is hardcoded (does not call HTTP),
	// and the contract suite only calls methods that don't require HTTP for most
	// tests. The only HTTP-dependent contract test is SupportedInstruments, which
	// for Binance returns a static list without an HTTP call.
	//
	// We still create the mock server to validate the pattern and in case the
	// implementation changes in the future.
	a := binance.NewAdapter(nil)

	contractSuite(t, a)
}
