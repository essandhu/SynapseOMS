package simulated

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
)

const (
	venueID   = "sim-exchange"
	venueName = "Simulated Exchange"
)

// defaultInstruments returns the six pre-loaded instruments.
func defaultInstruments() []instrumentDef {
	return []instrumentDef{
		{id: "AAPL", symbol: "AAPL", name: "Apple Inc.", assetClass: domain.AssetClassEquity, price: 185.0, volatility: 0.30, drift: 0.05, quoteCurrency: "USD", settlement: domain.SettlementT2},
		{id: "MSFT", symbol: "MSFT", name: "Microsoft Corp.", assetClass: domain.AssetClassEquity, price: 420.0, volatility: 0.30, drift: 0.05, quoteCurrency: "USD", settlement: domain.SettlementT2},
		{id: "GOOG", symbol: "GOOG", name: "Alphabet Inc.", assetClass: domain.AssetClassEquity, price: 175.0, volatility: 0.30, drift: 0.05, quoteCurrency: "USD", settlement: domain.SettlementT2},
		{id: "BTC-USD", symbol: "BTC-USD", name: "Bitcoin", assetClass: domain.AssetClassCrypto, price: 65000.0, volatility: 0.80, drift: 0.0, quoteCurrency: "USD", settlement: domain.SettlementT0},
		{id: "ETH-USD", symbol: "ETH-USD", name: "Ethereum", assetClass: domain.AssetClassCrypto, price: 3500.0, volatility: 0.80, drift: 0.0, quoteCurrency: "USD", settlement: domain.SettlementT0},
		{id: "SOL-USD", symbol: "SOL-USD", name: "Solana", assetClass: domain.AssetClassCrypto, price: 140.0, volatility: 0.80, drift: 0.0, quoteCurrency: "USD", settlement: domain.SettlementT0},
	}
}

type instrumentDef struct {
	id            string
	symbol        string
	name          string
	assetClass    domain.AssetClass
	price         float64
	volatility    float64
	drift         float64
	quoteCurrency string
	settlement    domain.SettlementCycle
}

// Adapter implements the adapter.LiquidityProvider interface for a simulated exchange.
type Adapter struct {
	engine      *MatchingEngine
	fillCh      chan domain.Fill
	instruments []domain.Instrument
	status      adapter.VenueStatus
	logger      *slog.Logger
	mdCh        chan adapter.MarketDataSnapshot
	mdStopCh    chan struct{}
}

// NewAdapter creates a new simulated exchange adapter.
func NewAdapter(_ map[string]string) adapter.LiquidityProvider {
	fillCh := make(chan domain.Fill, 1000)
	engine := NewMatchingEngine(fillCh)
	logger := logging.NewDefault("gateway", "simulated-adapter")

	defs := defaultInstruments()
	instruments := make([]domain.Instrument, 0, len(defs))

	for _, d := range defs {
		engine.RegisterInstrument(d.id, decimal.NewFromFloat(d.price), d.volatility, d.drift, 100*time.Millisecond)

		tickSize := decimal.NewFromFloat(0.01)
		lotSize := decimal.NewFromInt(1)
		schedule := domain.TradingSchedule{
			MarketOpen:  "09:30",
			MarketClose: "16:00",
			PreMarket:   "04:00",
			AfterHours:  "20:00",
			Timezone:    "America/New_York",
		}
		if d.assetClass == domain.AssetClassCrypto {
			tickSize = decimal.NewFromFloat(0.01)
			lotSize = decimal.NewFromFloat(0.0001)
			schedule = domain.TradingSchedule{Is24x7: true}
		}

		instruments = append(instruments, domain.Instrument{
			ID:              d.id,
			Symbol:          d.symbol,
			Name:            d.name,
			AssetClass:      d.assetClass,
			QuoteCurrency:   d.quoteCurrency,
			TickSize:        tickSize,
			LotSize:         lotSize,
			SettlementCycle: d.settlement,
			TradingHours:    schedule,
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

func (a *Adapter) VenueID() string   { return venueID }
func (a *Adapter) VenueName() string  { return venueName }
func (a *Adapter) VenueType() string  { return "simulated" }

func (a *Adapter) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassEquity, domain.AssetClassCrypto}
}

func (a *Adapter) SupportedInstruments() ([]domain.Instrument, error) {
	return a.instruments, nil
}

func (a *Adapter) Connect(_ context.Context, _ domain.VenueCredential) error {
	a.logger.Info("connecting to simulated exchange")
	a.engine.Start()
	a.status = adapter.Connected
	a.logger.Info("connected to simulated exchange")
	return nil
}

func (a *Adapter) Disconnect(_ context.Context) error {
	a.logger.Info("disconnecting from simulated exchange")
	a.engine.Stop()
	a.status = adapter.Disconnected
	a.logger.Info("disconnected from simulated exchange")
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
		return nil, fmt.Errorf("simulated exchange not connected")
	}

	a.logger.Info("submitting order",
		slog.String("order_id", string(order.ID)),
		slog.String("instrument", order.InstrumentID),
		slog.String("side", fmt.Sprintf("%d", order.Side)),
		slog.String("type", fmt.Sprintf("%d", order.Type)),
	)

	venueOrderID := fmt.Sprintf("SIM-%s", order.ID)
	a.engine.ProcessOrder(order)

	return &adapter.VenueAck{
		VenueOrderID: venueOrderID,
		ReceivedAt:   time.Now(),
	}, nil
}

func (a *Adapter) CancelOrder(_ context.Context, orderID domain.OrderID, venueOrderID string) error {
	if a.status != adapter.Connected {
		return fmt.Errorf("simulated exchange not connected")
	}

	a.logger.Info("canceling order",
		slog.String("order_id", string(orderID)),
		slog.String("venue_order_id", venueOrderID),
	)

	// Remove from order books
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()
	for _, book := range a.engine.books {
		book.mu.Lock()
		remaining := make([]*domain.Order, 0, len(book.orders))
		for _, o := range book.orders {
			if o.ID != orderID {
				remaining = append(remaining, o)
			}
		}
		book.orders = remaining
		book.mu.Unlock()
	}

	return nil
}

func (a *Adapter) QueryOrder(_ context.Context, venueOrderID string) (*domain.Order, error) {
	if a.status != adapter.Connected {
		return nil, fmt.Errorf("simulated exchange not connected")
	}

	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()
	for _, book := range a.engine.books {
		book.mu.Lock()
		for _, o := range book.orders {
			if fmt.Sprintf("SIM-%s", o.ID) == venueOrderID {
				book.mu.Unlock()
				return o, nil
			}
		}
		book.mu.Unlock()
	}

	return nil, fmt.Errorf("order %s not found", venueOrderID)
}

func (a *Adapter) SubscribeMarketData(_ context.Context, _ []string) (<-chan adapter.MarketDataSnapshot, error) {
	if a.status != adapter.Connected {
		return nil, fmt.Errorf("simulated exchange not connected")
	}

	ch := make(chan adapter.MarketDataSnapshot, 100)
	a.mdCh = ch
	a.mdStopCh = make(chan struct{})

	// Start a goroutine that pushes price snapshots for all instruments
	// every 500ms, matching the price walk tick rate.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		spread := decimal.NewFromFloat(0.02)

		for {
			select {
			case <-a.mdStopCh:
				return
			case <-ticker.C:
				for _, id := range a.engine.InstrumentIDs() {
					price, ok := a.engine.GetPrice(id)
					if !ok {
						continue
					}
					halfSpread := price.Mul(spread).Div(decimal.NewFromInt(2))
					snap := adapter.MarketDataSnapshot{
						InstrumentID: id,
						VenueID:      venueID,
						LastPrice:    price,
						BidPrice:     price.Sub(halfSpread),
						AskPrice:     price.Add(halfSpread),
						TickVolume:   decimal.NewFromFloat(rand.Float64()*999 + 1),
						Timestamp:    time.Now(),
					}
					select {
					case ch <- snap:
					default:
						// Drop if channel full
					}
				}
			}
		}
	}()

	return ch, nil
}

func (a *Adapter) UnsubscribeMarketData(_ context.Context, _ []string) error {
	if a.mdStopCh != nil {
		close(a.mdStopCh)
		a.mdStopCh = nil
	}
	return nil
}

func (a *Adapter) FillFeed() <-chan domain.Fill {
	return a.fillCh
}

func (a *Adapter) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{
		SupportedOrderTypes:   []domain.OrderType{domain.OrderTypeMarket, domain.OrderTypeLimit},
		SupportedAssetClasses: []domain.AssetClass{domain.AssetClassEquity, domain.AssetClassCrypto},
		SupportsStreaming:      true,
		MaxOrdersPerSecond:    1000,
	}
}

func init() {
	adapter.Register("simulated", NewAdapter)
}
