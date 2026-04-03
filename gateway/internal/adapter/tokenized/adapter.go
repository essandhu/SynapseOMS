package tokenized

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/adapter/simulated"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
)

const (
	venueID   = "tokenized_sim"
	venueName = "Tokenized Securities (Simulated)"
)

type instrumentDef struct {
	id         string
	symbol     string
	name       string
	price      float64
	volatility float64
	drift      float64
}

func defaultInstruments() []instrumentDef {
	return []instrumentDef{
		{id: "TSLA-T", symbol: "TSLA-T", name: "Tokenized Tesla Inc.", price: 250.0, volatility: 0.40, drift: 0.05},
		{id: "AAPL-T", symbol: "AAPL-T", name: "Tokenized Apple Inc.", price: 185.0, volatility: 0.30, drift: 0.05},
		{id: "SPY-T", symbol: "SPY-T", name: "Tokenized SPY ETF", price: 520.0, volatility: 0.20, drift: 0.03},
	}
}

// Adapter implements adapter.LiquidityProvider for tokenized securities.
type Adapter struct {
	engine      *simulated.MatchingEngine
	fillCh      chan domain.Fill
	instruments []domain.Instrument
	status      adapter.VenueStatus
	logger      *slog.Logger
	mdCh        chan adapter.MarketDataSnapshot
}

// NewAdapter creates a new tokenized securities adapter.
func NewAdapter(_ map[string]string) adapter.LiquidityProvider {
	fillCh := make(chan domain.Fill, 1000)
	engine := simulated.NewMatchingEngine(fillCh)
	logger := logging.NewDefault("gateway", "tokenized-adapter")

	defs := defaultInstruments()
	instruments := make([]domain.Instrument, 0, len(defs))

	for _, d := range defs {
		engine.RegisterInstrument(d.id, decimal.NewFromFloat(d.price), d.volatility, d.drift, 100*time.Millisecond)

		instruments = append(instruments, domain.Instrument{
			ID:              d.id,
			Symbol:          d.symbol,
			Name:            d.name,
			AssetClass:      domain.AssetClassTokenizedSecurity,
			QuoteCurrency:   "USD",
			TickSize:        decimal.NewFromFloat(0.01),
			LotSize:         decimal.NewFromFloat(0.001),
			SettlementCycle: domain.SettlementT0,
			TradingHours:    domain.TradingSchedule{Is24x7: true},
			Venues:          []string{venueID},
		})
	}

	return &Adapter{
		engine:      engine,
		fillCh:      fillCh,
		instruments: instruments,
		status:      adapter.Disconnected,
		logger:      logger,
	}
}

func (a *Adapter) VenueID() string  { return venueID }
func (a *Adapter) VenueName() string { return venueName }

func (a *Adapter) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassTokenizedSecurity}
}

func (a *Adapter) SupportedInstruments() ([]domain.Instrument, error) {
	return a.instruments, nil
}

func (a *Adapter) Connect(_ context.Context, _ domain.VenueCredential) error {
	a.logger.Info("connecting to tokenized securities exchange")
	a.engine.Start()
	a.status = adapter.Connected
	a.logger.Info("connected to tokenized securities exchange")
	return nil
}

func (a *Adapter) Disconnect(_ context.Context) error {
	a.logger.Info("disconnecting from tokenized securities exchange")
	a.engine.Stop()
	a.status = adapter.Disconnected
	a.logger.Info("disconnected from tokenized securities exchange")
	return nil
}

func (a *Adapter) Status() adapter.VenueStatus {
	return a.status
}

func (a *Adapter) Ping(_ context.Context) (time.Duration, error) {
	return 0, nil
}

func (a *Adapter) SubmitOrder(_ context.Context, order *domain.Order) (*adapter.VenueAck, error) {
	if a.status != adapter.Connected {
		return nil, fmt.Errorf("tokenized securities exchange not connected")
	}

	order.SettlementCycle = domain.SettlementT0

	a.logger.Info("submitting order",
		slog.String("order_id", string(order.ID)),
		slog.String("instrument", order.InstrumentID),
		slog.String("side", fmt.Sprintf("%d", order.Side)),
		slog.String("type", fmt.Sprintf("%d", order.Type)),
	)

	venueOrderID := fmt.Sprintf("TOK-%s", order.ID)
	a.engine.ProcessOrder(order)

	return &adapter.VenueAck{
		VenueOrderID: venueOrderID,
		ReceivedAt:   time.Now(),
	}, nil
}

func (a *Adapter) CancelOrder(_ context.Context, orderID domain.OrderID, venueOrderID string) error {
	if a.status != adapter.Connected {
		return fmt.Errorf("tokenized securities exchange not connected")
	}

	a.logger.Info("canceling order",
		slog.String("order_id", string(orderID)),
		slog.String("venue_order_id", venueOrderID),
	)

	a.engine.CancelOrder(orderID)
	return nil
}

func (a *Adapter) QueryOrder(_ context.Context, venueOrderID string) (*domain.Order, error) {
	if a.status != adapter.Connected {
		return nil, fmt.Errorf("tokenized securities exchange not connected")
	}

	order := a.engine.FindOrder(venueOrderID, "TOK-")
	if order == nil {
		return nil, fmt.Errorf("order %s not found", venueOrderID)
	}
	return order, nil
}

func (a *Adapter) SubscribeMarketData(_ context.Context, _ []string) (<-chan adapter.MarketDataSnapshot, error) {
	if a.status != adapter.Connected {
		return nil, fmt.Errorf("tokenized securities exchange not connected")
	}

	ch := make(chan adapter.MarketDataSnapshot, 100)
	a.mdCh = ch
	return ch, nil
}

func (a *Adapter) UnsubscribeMarketData(_ context.Context, _ []string) error {
	return nil
}

func (a *Adapter) FillFeed() <-chan domain.Fill {
	return a.fillCh
}

func (a *Adapter) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{
		SupportedOrderTypes:   []domain.OrderType{domain.OrderTypeMarket, domain.OrderTypeLimit},
		SupportedAssetClasses: []domain.AssetClass{domain.AssetClassTokenizedSecurity},
		SupportsStreaming:      true,
		MaxOrdersPerSecond:    1000,
	}
}

func init() {
	adapter.Register("tokenized", NewAdapter)
}
