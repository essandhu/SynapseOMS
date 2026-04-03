package tokenized_test

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/adapter/tokenized"
	"github.com/synapse-oms/gateway/internal/domain"
)

func newAdapter() adapter.LiquidityProvider {
	return tokenized.NewAdapter(nil)
}

func TestVenueID(t *testing.T) {
	a := newAdapter()
	if got := a.VenueID(); got != "tokenized_sim" {
		t.Errorf("VenueID() = %q, want %q", got, "tokenized_sim")
	}
}

func TestVenueName(t *testing.T) {
	a := newAdapter()
	want := "Tokenized Securities (Simulated)"
	if got := a.VenueName(); got != want {
		t.Errorf("VenueName() = %q, want %q", got, want)
	}
}

func TestSupportedAssetClasses(t *testing.T) {
	a := newAdapter()
	classes := a.SupportedAssetClasses()
	if len(classes) != 1 {
		t.Fatalf("expected 1 asset class, got %d", len(classes))
	}
	if classes[0] != domain.AssetClassTokenizedSecurity {
		t.Errorf("expected AssetClassTokenizedSecurity, got %v", classes[0])
	}
}

func TestConnectChangesStatusToConnected(t *testing.T) {
	a := newAdapter()
	if a.Status() != adapter.Disconnected {
		t.Fatal("expected Disconnected before Connect")
	}
	err := a.Connect(context.Background(), domain.VenueCredential{})
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	if a.Status() != adapter.Connected {
		t.Errorf("expected Connected after Connect, got %v", a.Status())
	}
	_ = a.Disconnect(context.Background())
}

func TestDisconnectChangesStatusToDisconnected(t *testing.T) {
	a := newAdapter()
	_ = a.Connect(context.Background(), domain.VenueCredential{})
	err := a.Disconnect(context.Background())
	if err != nil {
		t.Fatalf("Disconnect() error: %v", err)
	}
	if a.Status() != adapter.Disconnected {
		t.Errorf("expected Disconnected after Disconnect, got %v", a.Status())
	}
}

func TestSubmitOrderT0Settlement(t *testing.T) {
	a := newAdapter()
	_ = a.Connect(context.Background(), domain.VenueCredential{})
	defer func() { _ = a.Disconnect(context.Background()) }()

	order := &domain.Order{
		ID:              "test-order-1",
		InstrumentID:    "TSLA-T",
		Side:            domain.SideBuy,
		Type:            domain.OrderTypeMarket,
		Quantity:        decimal.NewFromInt(10),
		AssetClass:      domain.AssetClassTokenizedSecurity,
		SettlementCycle: domain.SettlementT0,
	}

	ack, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() error: %v", err)
	}
	if ack == nil {
		t.Fatal("SubmitOrder() returned nil ack")
	}
	if ack.VenueOrderID == "" {
		t.Error("SubmitOrder() returned empty VenueOrderID")
	}
	if order.SettlementCycle != domain.SettlementT0 {
		t.Errorf("expected SettlementT0, got %v", order.SettlementCycle)
	}
}

func TestSubmitOrderFailsWhenDisconnected(t *testing.T) {
	a := newAdapter()

	order := &domain.Order{
		ID:           "test-order-2",
		InstrumentID: "TSLA-T",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeMarket,
		Quantity:     decimal.NewFromInt(10),
	}

	_, err := a.SubmitOrder(context.Background(), order)
	if err == nil {
		t.Error("expected error when submitting order while disconnected")
	}
}

func TestCancelOrderWorksForPendingOrders(t *testing.T) {
	a := newAdapter()
	_ = a.Connect(context.Background(), domain.VenueCredential{})
	defer func() { _ = a.Disconnect(context.Background()) }()

	order := &domain.Order{
		ID:           "test-cancel-1",
		InstrumentID: "AAPL-T",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeLimit,
		Quantity:     decimal.NewFromInt(5),
		Price:        decimal.NewFromFloat(0.01), // very low price so it won't fill
		AssetClass:   domain.AssetClassTokenizedSecurity,
	}

	_, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() error: %v", err)
	}

	err = a.CancelOrder(context.Background(), order.ID, "TOK-test-cancel-1")
	if err != nil {
		t.Errorf("CancelOrder() error: %v", err)
	}
}

func TestQueryOrderFindsSubmittedOrders(t *testing.T) {
	a := newAdapter()
	_ = a.Connect(context.Background(), domain.VenueCredential{})
	defer func() { _ = a.Disconnect(context.Background()) }()

	order := &domain.Order{
		ID:           "test-query-1",
		InstrumentID: "SPY-T",
		Side:         domain.SideBuy,
		Type:         domain.OrderTypeLimit,
		Quantity:     decimal.NewFromInt(5),
		Price:        decimal.NewFromFloat(0.01), // very low price so it won't fill
		AssetClass:   domain.AssetClassTokenizedSecurity,
	}

	ack, err := a.SubmitOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("SubmitOrder() error: %v", err)
	}

	found, err := a.QueryOrder(context.Background(), ack.VenueOrderID)
	if err != nil {
		t.Fatalf("QueryOrder() error: %v", err)
	}
	if found.ID != order.ID {
		t.Errorf("QueryOrder() returned order ID %q, want %q", found.ID, order.ID)
	}
}

func TestSubscribeMarketDataReturnsChannelWhenConnected(t *testing.T) {
	a := newAdapter()
	_ = a.Connect(context.Background(), domain.VenueCredential{})
	defer func() { _ = a.Disconnect(context.Background()) }()

	ch, err := a.SubscribeMarketData(context.Background(), []string{"TSLA-T"})
	if err != nil {
		t.Fatalf("SubscribeMarketData() error: %v", err)
	}
	if ch == nil {
		t.Error("SubscribeMarketData() returned nil channel")
	}
}

func TestSubscribeMarketDataFailsWhenDisconnected(t *testing.T) {
	a := newAdapter()
	_, err := a.SubscribeMarketData(context.Background(), []string{"TSLA-T"})
	if err == nil {
		t.Error("expected error when subscribing while disconnected")
	}
}

func TestFillFeedReturnsNonNilChannel(t *testing.T) {
	a := newAdapter()
	ch := a.FillFeed()
	if ch == nil {
		t.Error("FillFeed() returned nil channel")
	}
}

func TestCapabilitiesIncludesTokenizedSecurity(t *testing.T) {
	a := newAdapter()
	caps := a.Capabilities()
	found := false
	for _, ac := range caps.SupportedAssetClasses {
		if ac == domain.AssetClassTokenizedSecurity {
			found = true
			break
		}
	}
	if !found {
		t.Error("Capabilities().SupportedAssetClasses does not include TokenizedSecurity")
	}
}

func TestSupportedInstrumentsAllT0(t *testing.T) {
	a := newAdapter()
	instruments, err := a.SupportedInstruments()
	if err != nil {
		t.Fatalf("SupportedInstruments() error: %v", err)
	}
	if len(instruments) == 0 {
		t.Fatal("expected at least one instrument")
	}
	for _, inst := range instruments {
		if inst.SettlementCycle != domain.SettlementT0 {
			t.Errorf("instrument %s has settlement %v, want T0", inst.Symbol, inst.SettlementCycle)
		}
		if inst.AssetClass != domain.AssetClassTokenizedSecurity {
			t.Errorf("instrument %s has asset class %v, want TokenizedSecurity", inst.Symbol, inst.AssetClass)
		}
	}
}
