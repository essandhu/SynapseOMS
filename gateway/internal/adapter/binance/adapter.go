// Package binance implements a LiquidityProvider adapter for the Binance testnet.
package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
)

const (
	venueID        = "binance_testnet"
	venueName      = "Binance Testnet"
	testnetBaseURL = "https://testnet.binance.vision/api/v3"
	testnetWSURL   = "wss://testnet.binance.vision/ws"

	keepAliveInterval = 30 * time.Minute
)

// symbolMap maps internal instrument IDs to Binance symbols.
var symbolMap = map[string]string{
	"BTC-USD": "BTCUSDT",
	"ETH-USD": "ETHUSDT",
	"SOL-USD": "SOLUSDT",
}

// reverseSymbolMap maps Binance symbols back to internal instrument IDs.
var reverseSymbolMap = map[string]string{
	"BTCUSDT": "BTC-USD",
	"ETHUSDT": "ETH-USD",
	"SOLUSDT": "SOL-USD",
}

// ToSymbol converts an internal instrument ID to a Binance symbol.
func ToSymbol(instrumentID string) (string, error) {
	s, ok := symbolMap[instrumentID]
	if !ok {
		return "", fmt.Errorf("unsupported instrument: %s", instrumentID)
	}
	return s, nil
}

// FromSymbol converts a Binance symbol to an internal instrument ID.
func FromSymbol(symbol string) (string, error) {
	id, ok := reverseSymbolMap[symbol]
	if !ok {
		return "", fmt.Errorf("unknown binance symbol: %s", symbol)
	}
	return id, nil
}

// Adapter implements adapter.LiquidityProvider for Binance testnet.
type Adapter struct {
	baseURL    string
	wsURL      string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	fillCh     chan domain.Fill
	marketCh   chan adapter.MarketDataSnapshot
	status     adapter.VenueStatus
	logger     *slog.Logger
	mu         sync.RWMutex
	listenKey  string

	marketFeed *MarketDataFeed
	userFeed   *UserDataFeed

	cancelKeepAlive context.CancelFunc
}

// NewAdapter creates a new Binance testnet adapter.
func NewAdapter(_ map[string]string) adapter.LiquidityProvider {
	return &Adapter{
		baseURL:    testnetBaseURL,
		wsURL:      testnetWSURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		fillCh:     make(chan domain.Fill, 1000),
		marketCh:   make(chan adapter.MarketDataSnapshot, 100),
		status:     adapter.Disconnected,
		logger:     logging.NewDefault("gateway", "binance-adapter"),
	}
}

func (a *Adapter) VenueID() string   { return venueID }
func (a *Adapter) VenueName() string  { return venueName }
func (a *Adapter) VenueType() string  { return "exchange" }

func (a *Adapter) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassCrypto}
}

func (a *Adapter) SupportedInstruments() ([]domain.Instrument, error) {
	schedule := domain.TradingSchedule{Is24x7: true}
	instruments := []domain.Instrument{
		{
			ID: "BTC-USD", Symbol: "BTC-USD", Name: "Bitcoin",
			AssetClass: domain.AssetClassCrypto, QuoteCurrency: "USD", BaseCurrency: "BTC",
			TickSize: decimal.NewFromFloat(0.01), LotSize: decimal.NewFromFloat(0.00001),
			SettlementCycle: domain.SettlementT0, TradingHours: schedule,
			Venues: []string{venueID},
		},
		{
			ID: "ETH-USD", Symbol: "ETH-USD", Name: "Ethereum",
			AssetClass: domain.AssetClassCrypto, QuoteCurrency: "USD", BaseCurrency: "ETH",
			TickSize: decimal.NewFromFloat(0.01), LotSize: decimal.NewFromFloat(0.0001),
			SettlementCycle: domain.SettlementT0, TradingHours: schedule,
			Venues: []string{venueID},
		},
		{
			ID: "SOL-USD", Symbol: "SOL-USD", Name: "Solana",
			AssetClass: domain.AssetClassCrypto, QuoteCurrency: "USD", BaseCurrency: "SOL",
			TickSize: decimal.NewFromFloat(0.01), LotSize: decimal.NewFromFloat(0.001),
			SettlementCycle: domain.SettlementT0, TradingHours: schedule,
			Venues: []string{venueID},
		},
	}
	return instruments, nil
}

// Connect authenticates with Binance testnet and starts WebSocket feeds.
func (a *Adapter) Connect(ctx context.Context, cred domain.VenueCredential) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.apiKey = cred.APIKey
	a.apiSecret = cred.APISecret
	a.logger.Info("connecting to binance testnet")

	// Create a listen key for the user data stream.
	listenKey, err := a.createListenKey(ctx)
	if err != nil {
		return fmt.Errorf("create listen key: %w", err)
	}
	a.listenKey = listenKey

	// Start the user data WebSocket feed.
	a.userFeed = NewUserDataFeed(a.wsURL, a.listenKey, a.fillCh, a.logger)
	if err := a.userFeed.Start(ctx); err != nil {
		return fmt.Errorf("start user data feed: %w", err)
	}

	// Start keep-alive goroutine for the listen key.
	kaCtx, kaCancel := context.WithCancel(context.Background())
	a.cancelKeepAlive = kaCancel
	go a.keepAliveLoop(kaCtx)

	a.status = adapter.Connected
	a.logger.Info("connected to binance testnet", slog.String("listen_key", a.listenKey))
	return nil
}

// Disconnect closes all WebSocket feeds and cleans up resources.
func (a *Adapter) Disconnect(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.logger.Info("disconnecting from binance testnet")

	if a.cancelKeepAlive != nil {
		a.cancelKeepAlive()
	}
	if a.userFeed != nil {
		a.userFeed.Stop()
	}
	if a.marketFeed != nil {
		a.marketFeed.Stop()
	}

	a.status = adapter.Disconnected
	a.logger.Info("disconnected from binance testnet")
	return nil
}

func (a *Adapter) Status() adapter.VenueStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// Ping tests connectivity to the Binance testnet REST API.
func (a *Adapter) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/ping", nil)
	if err != nil {
		return 0, fmt.Errorf("create ping request: %w", err)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("ping binance: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ping returned status %d", resp.StatusCode)
	}

	return time.Since(start), nil
}

// SubmitOrder sends a new order to the Binance testnet.
func (a *Adapter) SubmitOrder(ctx context.Context, order *domain.Order) (*adapter.VenueAck, error) {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return nil, fmt.Errorf("binance testnet not connected")
	}
	a.mu.RUnlock()

	symbol, err := ToSymbol(order.InstrumentID)
	if err != nil {
		return nil, err
	}

	side := "BUY"
	if order.Side == domain.SideSell {
		side = "SELL"
	}

	orderType := "MARKET"
	if order.Type == domain.OrderTypeLimit {
		orderType = "LIMIT"
	}

	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("type", orderType)
	params.Set("quantity", order.Quantity.String())
	params.Set("newClientOrderId", string(order.ID))

	if order.Type == domain.OrderTypeLimit {
		params.Set("price", order.Price.String())
		params.Set("timeInForce", "GTC")
	}

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	params.Set("timestamp", ts)
	params.Set("signature", a.sign(params.Encode()))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/order?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("create order request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit order: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read order response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("submit order returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode order response: %w", err)
	}

	a.logger.Info("order submitted",
		slog.String("order_id", string(order.ID)),
		slog.String("venue_order_id", strconv.FormatInt(result.OrderID, 10)),
		slog.String("symbol", symbol),
		slog.String("side", side),
	)

	return &adapter.VenueAck{
		VenueOrderID: strconv.FormatInt(result.OrderID, 10),
		ReceivedAt:   time.Now(),
	}, nil
}

// CancelOrder cancels an open order on Binance testnet.
func (a *Adapter) CancelOrder(ctx context.Context, orderID domain.OrderID, venueOrderID string) error {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return fmt.Errorf("binance testnet not connected")
	}
	a.mu.RUnlock()

	// We need the symbol to cancel. Try all supported symbols since the cancel
	// request requires a symbol parameter.
	// In production, we would store the symbol mapping with the order.
	// For now, we attempt with the venueOrderID and try each symbol.
	var lastErr error
	for _, symbol := range symbolMap {
		params := url.Values{}
		params.Set("symbol", symbol)
		params.Set("orderId", venueOrderID)
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		params.Set("timestamp", ts)
		params.Set("signature", a.sign(params.Encode()))

		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, a.baseURL+"/order?"+params.Encode(), nil)
		if err != nil {
			return fmt.Errorf("create cancel request: %w", err)
		}
		req.Header.Set("X-MBX-APIKEY", a.apiKey)

		resp, err := a.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)

		if resp.StatusCode == http.StatusOK {
			a.logger.Info("order canceled",
				slog.String("order_id", string(orderID)),
				slog.String("venue_order_id", venueOrderID),
			)
			return nil
		}
		lastErr = fmt.Errorf("cancel returned status %d for symbol %s", resp.StatusCode, symbol)
	}

	return fmt.Errorf("cancel order %s failed: %w", venueOrderID, lastErr)
}

// QueryOrder retrieves the status of an order from Binance testnet.
func (a *Adapter) QueryOrder(ctx context.Context, venueOrderID string) (*domain.Order, error) {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return nil, fmt.Errorf("binance testnet not connected")
	}
	a.mu.RUnlock()

	// Like CancelOrder, we try each symbol.
	var lastErr error
	for internalID, symbol := range symbolMap {
		params := url.Values{}
		params.Set("symbol", symbol)
		params.Set("orderId", venueOrderID)
		ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
		params.Set("timestamp", ts)
		params.Set("signature", a.sign(params.Encode()))

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/order?"+params.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("create query request: %w", err)
		}
		req.Header.Set("X-MBX-APIKEY", a.apiKey)

		resp, err := a.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("query returned status %d for symbol %s", resp.StatusCode, symbol)
			continue
		}

		var result struct {
			OrderID       int64  `json:"orderId"`
			ClientOrderID string `json:"clientOrderId"`
			Symbol        string `json:"symbol"`
			Side          string `json:"side"`
			Type          string `json:"type"`
			Status        string `json:"status"`
			Price         string `json:"price"`
			OrigQty       string `json:"origQty"`
			ExecutedQty   string `json:"executedQty"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = err
			continue
		}

		side := domain.SideBuy
		if result.Side == "SELL" {
			side = domain.SideSell
		}

		orderType := domain.OrderTypeMarket
		if result.Type == "LIMIT" {
			orderType = domain.OrderTypeLimit
		}

		price, _ := decimal.NewFromString(result.Price)
		origQty, _ := decimal.NewFromString(result.OrigQty)
		executedQty, _ := decimal.NewFromString(result.ExecutedQty)

		status := mapBinanceStatus(result.Status)

		return &domain.Order{
			ID:             domain.OrderID(result.ClientOrderID),
			InstrumentID:   internalID,
			Side:           side,
			Type:           orderType,
			Quantity:       origQty,
			Price:          price,
			FilledQuantity: executedQty,
			Status:         status,
			VenueID:        venueID,
			AssetClass:     domain.AssetClassCrypto,
		}, nil
	}

	return nil, fmt.Errorf("query order %s failed: %w", venueOrderID, lastErr)
}

// SubscribeMarketData subscribes to real-time book ticker streams for the given instruments.
func (a *Adapter) SubscribeMarketData(ctx context.Context, instruments []string) (<-chan adapter.MarketDataSnapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.status != adapter.Connected {
		return nil, fmt.Errorf("binance testnet not connected")
	}

	// Map internal IDs to Binance symbols.
	var symbols []string
	for _, inst := range instruments {
		sym, err := ToSymbol(inst)
		if err != nil {
			return nil, err
		}
		symbols = append(symbols, strings.ToLower(sym))
	}

	a.marketFeed = NewMarketDataFeed(a.wsURL, symbols, a.marketCh, a.logger)
	if err := a.marketFeed.Start(ctx); err != nil {
		return nil, fmt.Errorf("start market data feed: %w", err)
	}

	return a.marketCh, nil
}

// UnsubscribeMarketData stops the market data WebSocket feed.
func (a *Adapter) UnsubscribeMarketData(_ context.Context, _ []string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.marketFeed != nil {
		a.marketFeed.Stop()
		a.marketFeed = nil
	}
	return nil
}

func (a *Adapter) FillFeed() <-chan domain.Fill {
	return a.fillCh
}

func (a *Adapter) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{
		SupportedOrderTypes:   []domain.OrderType{domain.OrderTypeMarket, domain.OrderTypeLimit},
		SupportedAssetClasses: []domain.AssetClass{domain.AssetClassCrypto},
		SupportsStreaming:      true,
		MaxOrdersPerSecond:    10,
	}
}

// sign computes HMAC-SHA256 of the query string using the API secret.
func (a *Adapter) sign(queryString string) string {
	mac := hmac.New(sha256.New, []byte(a.apiSecret))
	mac.Write([]byte(queryString))
	return hex.EncodeToString(mac.Sum(nil))
}

// createListenKey creates a new listen key for the user data stream.
func (a *Adapter) createListenKey(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/userDataStream", nil)
	if err != nil {
		return "", fmt.Errorf("create listen key request: %w", err)
	}
	req.Header.Set("X-MBX-APIKEY", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create listen key: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read listen key response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create listen key returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode listen key response: %w", err)
	}

	return result.ListenKey, nil
}

// keepAliveLoop periodically sends a keep-alive request for the listen key.
func (a *Adapter) keepAliveLoop(ctx context.Context) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.mu.RLock()
			key := a.listenKey
			a.mu.RUnlock()

			if err := a.keepAliveListenKey(ctx, key); err != nil {
				a.logger.Error("listen key keep-alive failed", slog.String("error", err.Error()))
			} else {
				a.logger.Debug("listen key keep-alive sent")
			}
		}
	}
}

// keepAliveListenKey sends a PUT request to keep the listen key alive.
func (a *Adapter) keepAliveListenKey(ctx context.Context, listenKey string) error {
	params := url.Values{}
	params.Set("listenKey", listenKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, a.baseURL+"/userDataStream?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-MBX-APIKEY", a.apiKey)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("keep-alive returned status %d", resp.StatusCode)
	}
	return nil
}

// mapBinanceStatus maps a Binance order status string to a domain.OrderStatus.
func mapBinanceStatus(status string) domain.OrderStatus {
	switch status {
	case "NEW":
		return domain.OrderStatusNew
	case "PARTIALLY_FILLED":
		return domain.OrderStatusPartiallyFilled
	case "FILLED":
		return domain.OrderStatusFilled
	case "CANCELED", "EXPIRED", "REJECTED":
		return domain.OrderStatusCanceled
	default:
		return domain.OrderStatusNew
	}
}

func init() {
	adapter.Register("binance_testnet", NewAdapter)
}
