// Package alpaca implements the LiquidityProvider interface for Alpaca paper trading.
// It supports US equity paper trading via REST and WebSocket market data via the IEX stream.
// IMPORTANT: Only the paper trading base URL is used — the live API URL must never be used.
package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/adapter"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
)

const (
	venueID           = "alpaca"
	venueName         = "Alpaca Paper Trading"
	paperBaseURL      = "https://paper-api.alpaca.markets/v2"
	wsMarketDataURL   = "wss://stream.data.alpaca.markets/v2/iex"
	wsTradeUpdatesURL = "wss://paper-api.alpaca.markets/stream"
)

// Adapter implements adapter.LiquidityProvider for the Alpaca paper trading venue.
type Adapter struct {
	baseURL    string
	apiKey     string
	apiSecret  string
	httpClient *http.Client
	fillCh     chan domain.Fill
	marketCh   chan adapter.MarketDataSnapshot
	status     adapter.VenueStatus
	logger     *slog.Logger
	mu         sync.RWMutex

	// WebSocket connections
	marketFeed *MarketDataFeed
	tradeFeed  *TradeUpdateFeed
}

// NewAdapter creates a new Alpaca adapter. The config map may optionally contain
// "base_url" to override the default paper trading URL (useful for testing).
func NewAdapter(config map[string]string) adapter.LiquidityProvider {
	baseURL := paperBaseURL
	if u, ok := config["base_url"]; ok && u != "" {
		baseURL = u
	}

	return &Adapter{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		fillCh:   make(chan domain.Fill, 1000),
		marketCh: make(chan adapter.MarketDataSnapshot, 1000),
		status:   adapter.Disconnected,
		logger:   logging.NewDefault("gateway", "alpaca-adapter"),
	}
}

func (a *Adapter) VenueID() string   { return venueID }
func (a *Adapter) VenueName() string  { return venueName }
func (a *Adapter) VenueType() string  { return "exchange" }

func (a *Adapter) SupportedAssetClasses() []domain.AssetClass {
	return []domain.AssetClass{domain.AssetClassEquity}
}

// alpacaAsset represents the JSON response from the Alpaca /v2/assets endpoint.
type alpacaAsset struct {
	ID           string `json:"id"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	Class        string `json:"class"`       // "us_equity"
	Exchange     string `json:"exchange"`
	Status       string `json:"status"`       // "active"
	Tradable     bool   `json:"tradable"`
	Fractionable bool   `json:"fractionable"`
}

// SupportedInstruments fetches active US equity assets from Alpaca.
func (a *Adapter) SupportedInstruments() ([]domain.Instrument, error) {
	a.mu.RLock()
	key, secret, base := a.apiKey, a.apiSecret, a.baseURL
	a.mu.RUnlock()

	req, err := http.NewRequest(http.MethodGet, base+"/assets?status=active&class=us_equity", nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build request: %w", err)
	}
	a.setAuthHeaders(req, key, secret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: list assets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca: list assets status %d: %s", resp.StatusCode, string(body))
	}

	var assets []alpacaAsset
	if err := json.NewDecoder(resp.Body).Decode(&assets); err != nil {
		return nil, fmt.Errorf("alpaca: decode assets: %w", err)
	}

	instruments := make([]domain.Instrument, 0, len(assets))
	for _, asset := range assets {
		if !asset.Tradable {
			continue
		}
		instruments = append(instruments, domain.Instrument{
			ID:            asset.ID,
			Symbol:        asset.Symbol,
			Name:          asset.Name,
			AssetClass:    domain.AssetClassEquity,
			QuoteCurrency: "USD",
			TickSize:      decimal.NewFromFloat(0.01),
			LotSize:       decimal.NewFromInt(1),
			SettlementCycle: domain.SettlementT2,
			TradingHours: domain.TradingSchedule{
				MarketOpen:  "09:30",
				MarketClose: "16:00",
				PreMarket:   "04:00",
				AfterHours:  "20:00",
				Timezone:    "America/New_York",
			},
			Venues: []string{venueID},
		})
	}

	return instruments, nil
}

// Connect authenticates with the Alpaca API and starts WebSocket feeds.
func (a *Adapter) Connect(ctx context.Context, cred domain.VenueCredential) error {
	a.mu.Lock()
	a.apiKey = cred.APIKey
	a.apiSecret = cred.APISecret
	a.mu.Unlock()

	a.logger.Info("connecting to Alpaca paper trading")

	// Verify credentials by calling the account endpoint.
	if _, err := a.Ping(ctx); err != nil {
		return fmt.Errorf("alpaca: connect verification failed: %w", err)
	}

	// Start the market data WebSocket feed.
	a.mu.Lock()
	a.marketFeed = NewMarketDataFeed(wsMarketDataURL, a.apiKey, a.apiSecret, a.marketCh, a.logger)
	a.tradeFeed = NewTradeUpdateFeed(wsTradeUpdatesURL, a.apiKey, a.apiSecret, a.fillCh, a.logger)
	a.mu.Unlock()

	a.marketFeed.Start(ctx)
	a.tradeFeed.Start(ctx)

	a.mu.Lock()
	a.status = adapter.Connected
	a.mu.Unlock()

	a.logger.Info("connected to Alpaca paper trading")
	return nil
}

// Disconnect closes WebSocket connections and marks the adapter as disconnected.
func (a *Adapter) Disconnect(_ context.Context) error {
	a.logger.Info("disconnecting from Alpaca paper trading")

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.marketFeed != nil {
		a.marketFeed.Stop()
		a.marketFeed = nil
	}
	if a.tradeFeed != nil {
		a.tradeFeed.Stop()
		a.tradeFeed = nil
	}

	a.status = adapter.Disconnected
	a.logger.Info("disconnected from Alpaca paper trading")
	return nil
}

// Status returns the current connection status.
func (a *Adapter) Status() adapter.VenueStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.status
}

// Ping checks connectivity by calling the Alpaca account endpoint and returns latency.
func (a *Adapter) Ping(ctx context.Context) (time.Duration, error) {
	a.mu.RLock()
	key, secret, base := a.apiKey, a.apiSecret, a.baseURL
	a.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/account", nil)
	if err != nil {
		return 0, fmt.Errorf("alpaca: build ping request: %w", err)
	}
	a.setAuthHeaders(req, key, secret)

	start := time.Now()
	resp, err := a.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		return 0, fmt.Errorf("alpaca: ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("alpaca: ping status %d: %s", resp.StatusCode, string(body))
	}

	return latency, nil
}

// alpacaOrderRequest represents the JSON body sent to POST /v2/orders.
type alpacaOrderRequest struct {
	Symbol      string `json:"symbol"`
	Qty         string `json:"qty"`
	Side        string `json:"side"`
	Type        string `json:"type"`
	TimeInForce string `json:"time_in_force"`
	LimitPrice  string `json:"limit_price,omitempty"`
	StopPrice   string `json:"stop_price,omitempty"`
	ClientOrdID string `json:"client_order_id,omitempty"`
}

// alpacaOrderResponse represents the JSON response from the Alpaca orders endpoint.
type alpacaOrderResponse struct {
	ID            string `json:"id"`
	ClientOrderID string `json:"client_order_id"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Type          string `json:"type"`
	Qty           string `json:"qty"`
	FilledQty     string `json:"filled_qty"`
	FilledAvgPrice string `json:"filled_avg_price"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	LimitPrice    string `json:"limit_price"`
	StopPrice     string `json:"stop_price"`
}

// SubmitOrder sends an order to the Alpaca paper trading API.
func (a *Adapter) SubmitOrder(ctx context.Context, order *domain.Order) (*adapter.VenueAck, error) {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return nil, fmt.Errorf("alpaca: not connected")
	}
	key, secret, base := a.apiKey, a.apiSecret, a.baseURL
	a.mu.RUnlock()

	a.logger.Info("submitting order",
		slog.String("order_id", string(order.ID)),
		slog.String("instrument", order.InstrumentID),
		slog.String("side", orderSideStr(order.Side)),
		slog.String("type", orderTypeStr(order.Type)),
	)

	reqBody := alpacaOrderRequest{
		Symbol:      order.InstrumentID,
		Qty:         order.Quantity.String(),
		Side:        orderSideStr(order.Side),
		Type:        orderTypeStr(order.Type),
		TimeInForce: "day",
		ClientOrdID: order.ClientOrderID,
	}

	if order.Type == domain.OrderTypeLimit || order.Type == domain.OrderTypeStopLimit {
		reqBody.LimitPrice = order.Price.String()
	}
	if order.Type == domain.OrderTypeStopLimit {
		reqBody.StopPrice = order.Price.String()
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("alpaca: marshal order: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/orders", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("alpaca: build submit request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.setAuthHeaders(req, key, secret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: submit order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca: submit order status %d: %s", resp.StatusCode, string(body))
	}

	var alpacaResp alpacaOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&alpacaResp); err != nil {
		return nil, fmt.Errorf("alpaca: decode order response: %w", err)
	}

	return &adapter.VenueAck{
		VenueOrderID: alpacaResp.ID,
		ReceivedAt:   time.Now(),
	}, nil
}

// CancelOrder sends a DELETE request to cancel an order by its venue order ID.
func (a *Adapter) CancelOrder(ctx context.Context, orderID domain.OrderID, venueOrderID string) error {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return fmt.Errorf("alpaca: not connected")
	}
	key, secret, base := a.apiKey, a.apiSecret, a.baseURL
	a.mu.RUnlock()

	a.logger.Info("canceling order",
		slog.String("order_id", string(orderID)),
		slog.String("venue_order_id", venueOrderID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, base+"/orders/"+venueOrderID, nil)
	if err != nil {
		return fmt.Errorf("alpaca: build cancel request: %w", err)
	}
	a.setAuthHeaders(req, key, secret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("alpaca: cancel order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("alpaca: cancel order status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// QueryOrder retrieves order details from Alpaca by venue order ID.
func (a *Adapter) QueryOrder(ctx context.Context, venueOrderID string) (*domain.Order, error) {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return nil, fmt.Errorf("alpaca: not connected")
	}
	key, secret, base := a.apiKey, a.apiSecret, a.baseURL
	a.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/orders/"+venueOrderID, nil)
	if err != nil {
		return nil, fmt.Errorf("alpaca: build query request: %w", err)
	}
	a.setAuthHeaders(req, key, secret)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("alpaca: query order: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca: query order status %d: %s", resp.StatusCode, string(body))
	}

	var alpacaResp alpacaOrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&alpacaResp); err != nil {
		return nil, fmt.Errorf("alpaca: decode order response: %w", err)
	}

	return mapAlpacaOrderToOrder(alpacaResp), nil
}

// SubscribeMarketData subscribes to real-time market data for the given instruments
// via the Alpaca IEX WebSocket stream.
func (a *Adapter) SubscribeMarketData(_ context.Context, instruments []string) (<-chan adapter.MarketDataSnapshot, error) {
	a.mu.RLock()
	if a.status != adapter.Connected {
		a.mu.RUnlock()
		return nil, fmt.Errorf("alpaca: not connected")
	}
	mf := a.marketFeed
	a.mu.RUnlock()

	if mf != nil {
		mf.Subscribe(instruments)
	}

	return a.marketCh, nil
}

// UnsubscribeMarketData unsubscribes from market data for the given instruments.
func (a *Adapter) UnsubscribeMarketData(_ context.Context, instruments []string) error {
	a.mu.RLock()
	mf := a.marketFeed
	a.mu.RUnlock()

	if mf != nil {
		mf.Unsubscribe(instruments)
	}
	return nil
}

// FillFeed returns the channel on which fill events are delivered.
func (a *Adapter) FillFeed() <-chan domain.Fill {
	return a.fillCh
}

// Capabilities returns the capabilities of the Alpaca adapter.
func (a *Adapter) Capabilities() adapter.VenueCapabilities {
	return adapter.VenueCapabilities{
		SupportedOrderTypes:   []domain.OrderType{domain.OrderTypeMarket, domain.OrderTypeLimit, domain.OrderTypeStopLimit},
		SupportedAssetClasses: []domain.AssetClass{domain.AssetClassEquity},
		SupportsStreaming:     true,
		MaxOrdersPerSecond:    200,
	}
}

// setAuthHeaders adds the Alpaca API key headers to a request.
func (a *Adapter) setAuthHeaders(req *http.Request, key, secret string) {
	req.Header.Set("APCA-API-KEY-ID", key)
	req.Header.Set("APCA-API-SECRET-KEY", secret)
}

// orderSideStr converts a domain.OrderSide to the Alpaca API string representation.
func orderSideStr(side domain.OrderSide) string {
	switch side {
	case domain.SideBuy:
		return "buy"
	case domain.SideSell:
		return "sell"
	default:
		return "buy"
	}
}

// orderTypeStr converts a domain.OrderType to the Alpaca API string representation.
func orderTypeStr(ot domain.OrderType) string {
	switch ot {
	case domain.OrderTypeMarket:
		return "market"
	case domain.OrderTypeLimit:
		return "limit"
	case domain.OrderTypeStopLimit:
		return "stop_limit"
	default:
		return "market"
	}
}

// mapAlpacaOrderToOrder maps an Alpaca order response to a domain.Order.
func mapAlpacaOrderToOrder(r alpacaOrderResponse) *domain.Order {
	qty, _ := decimal.NewFromString(r.Qty)
	filledQty, _ := decimal.NewFromString(r.FilledQty)
	avgPrice, _ := decimal.NewFromString(r.FilledAvgPrice)
	price, _ := decimal.NewFromString(r.LimitPrice)

	var side domain.OrderSide
	if r.Side == "sell" {
		side = domain.SideSell
	}

	var orderType domain.OrderType
	switch r.Type {
	case "limit":
		orderType = domain.OrderTypeLimit
	case "stop_limit":
		orderType = domain.OrderTypeStopLimit
	default:
		orderType = domain.OrderTypeMarket
	}

	status := mapAlpacaStatus(r.Status)

	return &domain.Order{
		ID:              domain.OrderID(r.ID),
		ClientOrderID:   r.ClientOrderID,
		InstrumentID:    r.Symbol,
		Side:            side,
		Type:            orderType,
		Quantity:        qty,
		Price:           price,
		FilledQuantity:  filledQty,
		AveragePrice:    avgPrice,
		Status:          status,
		VenueID:         venueID,
		AssetClass:      domain.AssetClassEquity,
		SettlementCycle: domain.SettlementT2,
	}
}

// mapAlpacaStatus maps an Alpaca order status string to domain.OrderStatus.
func mapAlpacaStatus(s string) domain.OrderStatus {
	switch s {
	case "new", "accepted", "pending_new":
		return domain.OrderStatusNew
	case "partially_filled":
		return domain.OrderStatusPartiallyFilled
	case "filled":
		return domain.OrderStatusFilled
	case "canceled", "expired", "pending_cancel":
		return domain.OrderStatusCanceled
	case "rejected":
		return domain.OrderStatusRejected
	default:
		return domain.OrderStatusNew
	}
}

func init() {
	adapter.Register("alpaca", NewAdapter)
}
