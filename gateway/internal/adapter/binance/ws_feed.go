package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

// ------------------------------------------------------------------
// MarketDataFeed subscribes to Binance bookTicker streams via WebSocket.
// ------------------------------------------------------------------

// MarketDataFeed manages a WebSocket connection to Binance for real-time
// book ticker (best bid/ask) updates.
type MarketDataFeed struct {
	wsURL   string
	symbols []string // lowercase binance symbols, e.g. "btcusdt"
	outCh   chan adapter.MarketDataSnapshot
	logger  *slog.Logger

	conn   *websocket.Conn
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewMarketDataFeed creates a new MarketDataFeed.
// symbols should be lowercase Binance symbols (e.g. "btcusdt").
func NewMarketDataFeed(wsURL string, symbols []string, outCh chan adapter.MarketDataSnapshot, logger *slog.Logger) *MarketDataFeed {
	return &MarketDataFeed{
		wsURL:   wsURL,
		symbols: symbols,
		outCh:   outCh,
		logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// Start connects to the Binance WebSocket and begins reading messages.
func (f *MarketDataFeed) Start(_ context.Context) error {
	// Build combined stream URL: wss://host/ws/btcusdt@bookTicker/ethusdt@bookTicker
	var streams []string
	for _, sym := range f.symbols {
		streams = append(streams, sym+"@bookTicker")
	}
	wsAddr := f.wsURL + "/" + strings.Join(streams, "/")

	conn, _, err := websocket.DefaultDialer.Dial(wsAddr, nil)
	if err != nil {
		return fmt.Errorf("market data ws dial: %w", err)
	}

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	go f.readLoop()

	f.logger.Info("market data feed started", slog.String("url", wsAddr))
	return nil
}

// Stop closes the WebSocket connection.
func (f *MarketDataFeed) Stop() {
	close(f.stopCh)
	f.mu.Lock()
	if f.conn != nil {
		_ = f.conn.Close()
	}
	f.mu.Unlock()
}

// bookTickerMsg represents a Binance bookTicker WebSocket message.
type bookTickerMsg struct {
	Symbol   string `json:"s"`
	BidPrice string `json:"b"`
	BidQty   string `json:"B"`
	AskPrice string `json:"a"`
	AskQty   string `json:"A"`
}

func (f *MarketDataFeed) readLoop() {
	for {
		select {
		case <-f.stopCh:
			return
		default:
		}

		f.mu.Lock()
		conn := f.conn
		f.mu.Unlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-f.stopCh:
				return
			default:
				f.logger.Error("market data ws read error", slog.String("error", err.Error()))
				return
			}
		}

		var tick bookTickerMsg
		if err := json.Unmarshal(msg, &tick); err != nil {
			f.logger.Warn("market data ws unmarshal error", slog.String("error", err.Error()))
			continue
		}

		instrumentID, err := FromSymbol(tick.Symbol)
		if err != nil {
			// Could be a symbol we don't track internally.
			continue
		}

		bidPrice, _ := decimal.NewFromString(tick.BidPrice)
		askPrice, _ := decimal.NewFromString(tick.AskPrice)

		snapshot := adapter.MarketDataSnapshot{
			InstrumentID: instrumentID,
			VenueID:      venueID,
			BidPrice:     bidPrice,
			AskPrice:     askPrice,
			Timestamp:    time.Now(),
		}

		select {
		case f.outCh <- snapshot:
		default:
			// Drop if channel is full to avoid blocking.
		}
	}
}

// ------------------------------------------------------------------
// UserDataFeed connects to the Binance user data stream for fill events.
// ------------------------------------------------------------------

// UserDataFeed manages a WebSocket connection to the Binance user data stream
// which delivers execution reports (fills, order updates).
type UserDataFeed struct {
	wsURL     string
	listenKey string
	fillCh    chan domain.Fill
	logger    *slog.Logger

	conn   *websocket.Conn
	mu     sync.Mutex
	stopCh chan struct{}
}

// NewUserDataFeed creates a new UserDataFeed.
func NewUserDataFeed(wsURL string, listenKey string, fillCh chan domain.Fill, logger *slog.Logger) *UserDataFeed {
	return &UserDataFeed{
		wsURL:     wsURL,
		listenKey: listenKey,
		fillCh:    fillCh,
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// Start connects to the Binance user data WebSocket stream.
func (f *UserDataFeed) Start(_ context.Context) error {
	wsAddr := f.wsURL + "/" + f.listenKey
	conn, _, err := websocket.DefaultDialer.Dial(wsAddr, nil)
	if err != nil {
		return fmt.Errorf("user data ws dial: %w", err)
	}

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	go f.readLoop()

	f.logger.Info("user data feed started")
	return nil
}

// Stop closes the user data WebSocket connection.
func (f *UserDataFeed) Stop() {
	close(f.stopCh)
	f.mu.Lock()
	if f.conn != nil {
		_ = f.conn.Close()
	}
	f.mu.Unlock()
}

// executionReportMsg represents a Binance execution report WebSocket message.
type executionReportMsg struct {
	EventType     string `json:"e"`
	Symbol        string `json:"s"`
	ClientOrderID string `json:"c"`
	Side          string `json:"S"`
	OrderType     string `json:"o"`
	ExecType      string `json:"x"` // TRADE = fill
	OrderStatus   string `json:"X"`
	OrderID       int64  `json:"i"`
	LastQty       string `json:"l"` // last executed quantity
	LastPrice     string `json:"L"` // last executed price
	Commission    string `json:"n"` // commission amount
	CommAsset     string `json:"N"` // commission asset
	TradeID       int64  `json:"t"` // trade ID
}

func (f *UserDataFeed) readLoop() {
	for {
		select {
		case <-f.stopCh:
			return
		default:
		}

		f.mu.Lock()
		conn := f.conn
		f.mu.Unlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-f.stopCh:
				return
			default:
				f.logger.Error("user data ws read error", slog.String("error", err.Error()))
				return
			}
		}

		// Parse the event type first.
		var event struct {
			EventType string `json:"e"`
		}
		if err := json.Unmarshal(msg, &event); err != nil {
			f.logger.Warn("user data ws unmarshal error", slog.String("error", err.Error()))
			continue
		}

		if event.EventType != "executionReport" {
			continue
		}

		var report executionReportMsg
		if err := json.Unmarshal(msg, &report); err != nil {
			f.logger.Warn("execution report unmarshal error", slog.String("error", err.Error()))
			continue
		}

		// Only process actual trade fills.
		if report.ExecType != "TRADE" {
			continue
		}

		lastQty, _ := decimal.NewFromString(report.LastQty)
		lastPrice, _ := decimal.NewFromString(report.LastPrice)
		commission, _ := decimal.NewFromString(report.Commission)

		liquidity := domain.LiquidityTaker
		// Binance doesn't expose maker/taker in user data stream directly for spot,
		// but we default to taker for market orders.
		if report.OrderType == "LIMIT" {
			liquidity = domain.LiquidityMaker
		}

		fill := domain.Fill{
			ID:          fmt.Sprintf("%d", report.TradeID),
			OrderID:     domain.OrderID(report.ClientOrderID),
			VenueID:     venueID,
			Quantity:    lastQty,
			Price:       lastPrice,
			Fee:         commission,
			FeeAsset:    report.CommAsset,
			FeeModel:    domain.FeeModelPercentage,
			Liquidity:   liquidity,
			Timestamp:   time.Now(),
			VenueExecID: fmt.Sprintf("%d", report.TradeID),
		}

		select {
		case f.fillCh <- fill:
		default:
			f.logger.Warn("fill channel full, dropping fill",
				slog.String("trade_id", fmt.Sprintf("%d", report.TradeID)),
			)
		}
	}
}
