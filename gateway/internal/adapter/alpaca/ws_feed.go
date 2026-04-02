package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
)

const (
	// reconnectDelay is the delay between WebSocket reconnection attempts.
	reconnectDelay = 3 * time.Second
	// pingInterval is how often we send a ping to keep the connection alive.
	// Alpaca disconnects idle connections after ~5 minutes.
	pingInterval = 30 * time.Second
	// writeTimeout is the timeout for WebSocket write operations.
	writeTimeout = 10 * time.Second
)

// ----- Market Data Feed (IEX stream) -----

// wsMessage represents a generic Alpaca WebSocket message.
type wsMessage struct {
	Action string   `json:"action,omitempty"`
	Trades []string `json:"trades,omitempty"`
	Quotes []string `json:"quotes,omitempty"`
	Bars   []string `json:"bars,omitempty"`
}

// wsAuthMessage is used to authenticate with the Alpaca WebSocket.
type wsAuthMessage struct {
	Action string `json:"action"`
	Key    string `json:"key"`
	Secret string `json:"secret"`
}

// wsQuoteEvent represents a real-time quote from the IEX stream.
type wsQuoteEvent struct {
	T  string  `json:"T"`  // message type: "q" for quote
	S  string  `json:"S"`  // symbol
	Bp float64 `json:"bp"` // bid price
	Bs int     `json:"bs"` // bid size
	Ap float64 `json:"ap"` // ask price
	As int     `json:"as"` // ask size
	Ts string  `json:"t"`  // timestamp
}

// wsTradeEvent represents a real-time trade from the IEX stream.
type wsTradeEvent struct {
	T  string  `json:"T"`  // message type: "t" for trade
	S  string  `json:"S"`  // symbol
	P  float64 `json:"p"`  // price
	Sz int     `json:"s"`  // size
	Ts string  `json:"t"`  // timestamp
}

// MarketDataFeed manages the WebSocket connection to the Alpaca IEX market data stream.
type MarketDataFeed struct {
	url       string
	apiKey    string
	apiSecret string
	outCh     chan adapter.MarketDataSnapshot
	logger    *slog.Logger

	conn   *websocket.Conn
	mu     sync.Mutex
	stopCh chan struct{}
	done   chan struct{}
}

// NewMarketDataFeed creates a new MarketDataFeed.
func NewMarketDataFeed(url, apiKey, apiSecret string, outCh chan adapter.MarketDataSnapshot, logger *slog.Logger) *MarketDataFeed {
	return &MarketDataFeed{
		url:       url,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		outCh:     outCh,
		logger:    logger,
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start begins the market data feed connection in a background goroutine.
func (f *MarketDataFeed) Start(ctx context.Context) {
	go f.run(ctx)
}

// Stop closes the market data feed.
func (f *MarketDataFeed) Stop() {
	select {
	case <-f.stopCh:
		// Already stopped.
	default:
		close(f.stopCh)
	}
	<-f.done
}

// Subscribe sends a subscribe message for the given symbols' quotes.
func (f *MarketDataFeed) Subscribe(symbols []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return
	}

	msg := wsMessage{
		Action: "subscribe",
		Quotes: symbols,
		Trades: symbols,
	}
	_ = f.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := f.conn.WriteJSON(msg); err != nil {
		f.logger.Error("alpaca market data: subscribe failed", slog.String("error", err.Error()))
	}
}

// Unsubscribe sends an unsubscribe message for the given symbols.
func (f *MarketDataFeed) Unsubscribe(symbols []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.conn == nil {
		return
	}

	msg := wsMessage{
		Action: "unsubscribe",
		Quotes: symbols,
		Trades: symbols,
	}
	_ = f.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := f.conn.WriteJSON(msg); err != nil {
		f.logger.Error("alpaca market data: unsubscribe failed", slog.String("error", err.Error()))
	}
}

func (f *MarketDataFeed) run(ctx context.Context) {
	defer close(f.done)

	for {
		select {
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		if err := f.connect(ctx); err != nil {
			f.logger.Warn("alpaca market data: connection failed, retrying",
				slog.String("error", err.Error()),
				slog.Duration("delay", reconnectDelay),
			)
			select {
			case <-time.After(reconnectDelay):
			case <-f.stopCh:
				return
			case <-ctx.Done():
				return
			}
			continue
		}

		f.readLoop(ctx)

		f.mu.Lock()
		if f.conn != nil {
			_ = f.conn.Close()
			f.conn = nil
		}
		f.mu.Unlock()
	}
}

func (f *MarketDataFeed) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, f.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Authenticate.
	authMsg := wsAuthMessage{
		Action: "auth",
		Key:    f.apiKey,
		Secret: f.apiSecret,
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteJSON(authMsg); err != nil {
		_ = conn.Close()
		return fmt.Errorf("auth write: %w", err)
	}

	// Read auth response.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("auth response: %w", err)
	}

	// Clear the read deadline for the read loop.
	_ = conn.SetReadDeadline(time.Time{})

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	f.logger.Info("alpaca market data: connected")
	return nil
}

func (f *MarketDataFeed) readLoop(ctx context.Context) {
	// Start a ping ticker to keep the connection alive.
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				f.mu.Lock()
				if f.conn != nil {
					_ = f.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
					_ = f.conn.WriteMessage(websocket.PingMessage, nil)
				}
				f.mu.Unlock()
			case <-f.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		f.mu.Lock()
		conn := f.conn
		f.mu.Unlock()
		if conn == nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				f.logger.Warn("alpaca market data: read error", slog.String("error", err.Error()))
			}
			return
		}

		// Alpaca sends arrays of events.
		var events []json.RawMessage
		if err := json.Unmarshal(raw, &events); err != nil {
			// Try as single message.
			events = []json.RawMessage{raw}
		}

		for _, evt := range events {
			var peek struct {
				T string `json:"T"`
			}
			if err := json.Unmarshal(evt, &peek); err != nil {
				continue
			}

			switch peek.T {
			case "q":
				var q wsQuoteEvent
				if err := json.Unmarshal(evt, &q); err != nil {
					continue
				}
				snapshot := adapter.MarketDataSnapshot{
					InstrumentID: q.S,
					VenueID:      venueID,
					BidPrice:     decimal.NewFromFloat(q.Bp),
					AskPrice:     decimal.NewFromFloat(q.Ap),
					Timestamp:    time.Now(),
				}
				select {
				case f.outCh <- snapshot:
				default:
					// Drop if channel full.
				}
			case "t":
				var t wsTradeEvent
				if err := json.Unmarshal(evt, &t); err != nil {
					continue
				}
				snapshot := adapter.MarketDataSnapshot{
					InstrumentID: t.S,
					VenueID:      venueID,
					LastPrice:    decimal.NewFromFloat(t.P),
					Timestamp:    time.Now(),
				}
				select {
				case f.outCh <- snapshot:
				default:
				}
			}
		}
	}
}

// ----- Trade Update Feed (fill events) -----

// wsTradeUpdate represents a trade update event from the Alpaca streaming API.
type wsTradeUpdate struct {
	Stream string              `json:"stream"`
	Data   wsTradeUpdateDetail `json:"data"`
}

// wsTradeUpdateDetail contains the detail of a trade update.
type wsTradeUpdateDetail struct {
	Event     string                 `json:"event"`
	Order     wsTradeUpdateOrder     `json:"order"`
	Execution *wsTradeUpdateExecution `json:"execution,omitempty"`
}

type wsTradeUpdateOrder struct {
	ID            string `json:"id"`
	ClientOrderID string `json:"client_order_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Qty           string `json:"qty"`
	FilledQty     string `json:"filled_qty"`
	FilledAvgPrice string `json:"filled_avg_price"`
	Status        string `json:"status"`
}

type wsTradeUpdateExecution struct {
	ID       string `json:"id"`
	Qty      string `json:"qty"`
	Price    string `json:"price"`
	Timestamp string `json:"timestamp"`
}

// TradeUpdateFeed manages the WebSocket connection for Alpaca trade updates (fills).
type TradeUpdateFeed struct {
	url       string
	apiKey    string
	apiSecret string
	fillCh    chan domain.Fill
	logger    *slog.Logger

	conn   *websocket.Conn
	mu     sync.Mutex
	stopCh chan struct{}
	done   chan struct{}
}

// NewTradeUpdateFeed creates a new TradeUpdateFeed.
func NewTradeUpdateFeed(url, apiKey, apiSecret string, fillCh chan domain.Fill, logger *slog.Logger) *TradeUpdateFeed {
	return &TradeUpdateFeed{
		url:       url,
		apiKey:    apiKey,
		apiSecret: apiSecret,
		fillCh:    fillCh,
		logger:    logger,
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start begins the trade update feed connection in a background goroutine.
func (f *TradeUpdateFeed) Start(ctx context.Context) {
	go f.run(ctx)
}

// Stop closes the trade update feed.
func (f *TradeUpdateFeed) Stop() {
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
	<-f.done
}

func (f *TradeUpdateFeed) run(ctx context.Context) {
	defer close(f.done)

	for {
		select {
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		if err := f.connect(ctx); err != nil {
			f.logger.Warn("alpaca trade updates: connection failed, retrying",
				slog.String("error", err.Error()),
				slog.Duration("delay", reconnectDelay),
			)
			select {
			case <-time.After(reconnectDelay):
			case <-f.stopCh:
				return
			case <-ctx.Done():
				return
			}
			continue
		}

		f.readLoop(ctx)

		f.mu.Lock()
		if f.conn != nil {
			_ = f.conn.Close()
			f.conn = nil
		}
		f.mu.Unlock()
	}
}

func (f *TradeUpdateFeed) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, f.url, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Authenticate via the streaming API auth format.
	authMsg := map[string]interface{}{
		"action": "authenticate",
		"data": map[string]string{
			"key_id":     f.apiKey,
			"secret_key": f.apiSecret,
		},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteJSON(authMsg); err != nil {
		_ = conn.Close()
		return fmt.Errorf("auth write: %w", err)
	}

	// Read auth response.
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, _, err = conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("auth response: %w", err)
	}

	// Subscribe to trade_updates stream.
	subMsg := map[string]interface{}{
		"action": "listen",
		"data": map[string]interface{}{
			"streams": []string{"trade_updates"},
		},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	if err := conn.WriteJSON(subMsg); err != nil {
		_ = conn.Close()
		return fmt.Errorf("subscribe write: %w", err)
	}

	// Clear deadlines.
	_ = conn.SetReadDeadline(time.Time{})

	f.mu.Lock()
	f.conn = conn
	f.mu.Unlock()

	f.logger.Info("alpaca trade updates: connected")
	return nil
}

func (f *TradeUpdateFeed) readLoop(ctx context.Context) {
	pingTicker := time.NewTicker(pingInterval)
	defer pingTicker.Stop()

	go func() {
		for {
			select {
			case <-pingTicker.C:
				f.mu.Lock()
				if f.conn != nil {
					_ = f.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
					_ = f.conn.WriteMessage(websocket.PingMessage, nil)
				}
				f.mu.Unlock()
			case <-f.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-f.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		f.mu.Lock()
		conn := f.conn
		f.mu.Unlock()
		if conn == nil {
			return
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				f.logger.Warn("alpaca trade updates: read error", slog.String("error", err.Error()))
			}
			return
		}

		var update wsTradeUpdate
		if err := json.Unmarshal(raw, &update); err != nil {
			f.logger.Debug("alpaca trade updates: unmarshal error", slog.String("error", err.Error()))
			continue
		}

		// Only process fill and partial_fill events.
		if update.Data.Event != "fill" && update.Data.Event != "partial_fill" {
			continue
		}

		fill := f.mapToFill(update.Data)
		select {
		case f.fillCh <- fill:
		default:
			f.logger.Warn("alpaca trade updates: fill channel full, dropping fill")
		}
	}
}

func (f *TradeUpdateFeed) mapToFill(detail wsTradeUpdateDetail) domain.Fill {
	var qty, price decimal.Decimal
	if detail.Execution != nil {
		qty, _ = decimal.NewFromString(detail.Execution.Qty)
		price, _ = decimal.NewFromString(detail.Execution.Price)
	} else {
		qty, _ = decimal.NewFromString(detail.Order.FilledQty)
		price, _ = decimal.NewFromString(detail.Order.FilledAvgPrice)
	}

	execID := ""
	if detail.Execution != nil {
		execID = detail.Execution.ID
	}

	return domain.Fill{
		ID:          fmt.Sprintf("alpaca-%s-%s", detail.Order.ID, execID),
		OrderID:     domain.OrderID(detail.Order.ID),
		VenueID:     venueID,
		Quantity:    qty,
		Price:       price,
		Fee:         decimal.Zero,
		FeeAsset:    "USD",
		FeeModel:    domain.FeeModelPerShare,
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: execID,
	}
}
