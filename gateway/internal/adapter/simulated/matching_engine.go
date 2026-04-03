package simulated

import (
	"crypto/rand"
	"fmt"
	"math"
	mrand "math/rand"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// OrderBook holds pending limit orders for a single instrument.
type OrderBook struct {
	mu     sync.Mutex
	orders []*domain.Order
}

// MatchingEngine simulates an exchange with synthetic prices and order matching.
type MatchingEngine struct {
	mu         sync.RWMutex
	books      map[string]*OrderBook
	priceWalks map[string]*PriceWalk
	fillCh     chan domain.Fill
	rng        *mrand.Rand
}

// NewMatchingEngine creates a new matching engine with the given fill channel.
func NewMatchingEngine(fillCh chan domain.Fill) *MatchingEngine {
	return &MatchingEngine{
		books:      make(map[string]*OrderBook),
		priceWalks: make(map[string]*PriceWalk),
		fillCh:     fillCh,
		rng:        mrand.New(mrand.NewSource(time.Now().UnixNano())),
	}
}

// RegisterInstrument sets up a price walk and order book for an instrument.
func (me *MatchingEngine) RegisterInstrument(instrumentID string, initialPrice decimal.Decimal, volatility, drift float64, interval time.Duration) {
	me.mu.Lock()
	defer me.mu.Unlock()
	me.priceWalks[instrumentID] = NewPriceWalk(initialPrice, volatility, drift, interval)
	me.books[instrumentID] = &OrderBook{}
}

// Start begins all price walks and the limit order sweep loop.
func (me *MatchingEngine) Start() {
	me.mu.RLock()
	defer me.mu.RUnlock()
	for _, pw := range me.priceWalks {
		pw.Start()
	}
	go me.sweepLoop()
}

// Stop halts all price walks.
func (me *MatchingEngine) Stop() {
	me.mu.RLock()
	defer me.mu.RUnlock()
	for _, pw := range me.priceWalks {
		pw.Stop()
	}
}

// InstrumentIDs returns all registered instrument IDs.
func (me *MatchingEngine) InstrumentIDs() []string {
	me.mu.RLock()
	defer me.mu.RUnlock()
	ids := make([]string, 0, len(me.priceWalks))
	for id := range me.priceWalks {
		ids = append(ids, id)
	}
	return ids
}

// GetPrice returns the current synthetic price for an instrument.
func (me *MatchingEngine) GetPrice(instrumentID string) (decimal.Decimal, bool) {
	me.mu.RLock()
	defer me.mu.RUnlock()
	pw, ok := me.priceWalks[instrumentID]
	if !ok {
		return decimal.Zero, false
	}
	return pw.CurrentPrice(), true
}

// ProcessOrder handles an incoming order: market orders fill immediately,
// limit orders are queued for sweep matching.
func (me *MatchingEngine) ProcessOrder(order *domain.Order) {
	switch order.Type {
	case domain.OrderTypeMarket:
		me.fillMarketOrder(order)
	case domain.OrderTypeLimit:
		me.queueLimitOrder(order)
	}
}

func (me *MatchingEngine) fillMarketOrder(order *domain.Order) {
	price, ok := me.GetPrice(order.InstrumentID)
	if !ok {
		return
	}

	// Apply random slippage 0-5bps
	slippageBps := me.rng.Float64() * 5.0
	slippageMul := 1.0
	if order.Side == domain.SideBuy {
		slippageMul = 1.0 + slippageBps/10000.0
	} else {
		slippageMul = 1.0 - slippageBps/10000.0
	}
	fillPrice := decimal.NewFromFloat(price.InexactFloat64() * slippageMul)

	fee := calculateFee(order.AssetClass, order.Quantity, fillPrice)

	fill := domain.Fill{
		ID:          newFillID(),
		OrderID:     order.ID,
		VenueID:     "simulated",
		Quantity:    order.Quantity,
		Price:       fillPrice,
		Fee:         fee,
		FeeAsset:    feeAsset(order.AssetClass),
		FeeModel:    feeModel(order.AssetClass),
		Liquidity:   domain.LiquidityTaker,
		Timestamp:   time.Now(),
		VenueExecID: newFillID(),
	}

	me.fillCh <- fill
}

func (me *MatchingEngine) queueLimitOrder(order *domain.Order) {
	me.mu.RLock()
	book, ok := me.books[order.InstrumentID]
	me.mu.RUnlock()
	if !ok {
		return
	}
	book.mu.Lock()
	defer book.mu.Unlock()
	book.orders = append(book.orders, order)
}

// sweepLoop checks limit orders against current prices every 50ms.
func (me *MatchingEngine) sweepLoop() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for range ticker.C {
		me.sweepAll()
	}
}

func (me *MatchingEngine) sweepAll() {
	me.mu.RLock()
	instrumentIDs := make([]string, 0, len(me.books))
	for id := range me.books {
		instrumentIDs = append(instrumentIDs, id)
	}
	me.mu.RUnlock()

	for _, id := range instrumentIDs {
		me.sweepInstrument(id)
	}
}

func (me *MatchingEngine) sweepInstrument(instrumentID string) {
	price, ok := me.GetPrice(instrumentID)
	if !ok {
		return
	}

	me.mu.RLock()
	book, ok := me.books[instrumentID]
	me.mu.RUnlock()
	if !ok {
		return
	}

	book.mu.Lock()
	defer book.mu.Unlock()

	remaining := make([]*domain.Order, 0, len(book.orders))
	for _, order := range book.orders {
		if shouldFillLimit(order, price) {
			fee := calculateFee(order.AssetClass, order.Quantity, order.Price)
			fill := domain.Fill{
				ID:          newFillID(),
				OrderID:     order.ID,
				VenueID:     "simulated",
				Quantity:    order.Quantity,
				Price:       order.Price,
				Fee:         fee,
				FeeAsset:    feeAsset(order.AssetClass),
				FeeModel:    feeModel(order.AssetClass),
				Liquidity:   domain.LiquidityMaker,
				Timestamp:   time.Now(),
				VenueExecID: newFillID(),
			}
			me.fillCh <- fill
		} else {
			remaining = append(remaining, order)
		}
	}
	book.orders = remaining
}

// shouldFillLimit checks if a limit order should be filled at the current price.
func shouldFillLimit(order *domain.Order, currentPrice decimal.Decimal) bool {
	switch order.Side {
	case domain.SideBuy:
		// Buy limit fills when price <= limit price
		return currentPrice.LessThanOrEqual(order.Price)
	case domain.SideSell:
		// Sell limit fills when price >= limit price
		return currentPrice.GreaterThanOrEqual(order.Price)
	}
	return false
}

// calculateFee computes the fee based on asset class.
// Equity: $0.005/share, Crypto: 0.1% of notional.
func calculateFee(assetClass domain.AssetClass, quantity, price decimal.Decimal) decimal.Decimal {
	switch assetClass {
	case domain.AssetClassEquity:
		// $0.005 per share
		return quantity.Mul(decimal.NewFromFloat(0.005))
	case domain.AssetClassCrypto:
		// 0.1% of notional (price * quantity)
		notional := price.Mul(quantity)
		return notional.Mul(decimal.NewFromFloat(0.001))
	default:
		return decimal.Zero
	}
}

func feeAsset(assetClass domain.AssetClass) string {
	switch assetClass {
	case domain.AssetClassEquity:
		return "USD"
	case domain.AssetClassCrypto:
		return "USD"
	default:
		return "USD"
	}
}

func feeModel(assetClass domain.AssetClass) domain.FeeModel {
	switch assetClass {
	case domain.AssetClassEquity:
		return domain.FeeModelPerShare
	case domain.AssetClassCrypto:
		return domain.FeeModelPercentage
	default:
		return domain.FeeModelPerShare
	}
}

func newFillID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("fill-%x", b)
}

// CancelOrder removes an order from all order books by its ID.
func (me *MatchingEngine) CancelOrder(orderID domain.OrderID) {
	me.mu.RLock()
	defer me.mu.RUnlock()
	for _, book := range me.books {
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
}

// FindOrder searches all order books for an order matching the venue order ID.
// The prefix is stripped from venueOrderID to match against the order's ID.
func (me *MatchingEngine) FindOrder(venueOrderID, prefix string) *domain.Order {
	me.mu.RLock()
	defer me.mu.RUnlock()
	for _, book := range me.books {
		book.mu.Lock()
		for _, o := range book.orders {
			if fmt.Sprintf("%s%s", prefix, o.ID) == venueOrderID {
				book.mu.Unlock()
				return o
			}
		}
		book.mu.Unlock()
	}
	return nil
}

// slippageBps returns the slippage in basis points between two prices.
func slippageBps(original, filled decimal.Decimal) float64 {
	if original.IsZero() {
		return 0
	}
	diff := filled.Sub(original).Abs()
	return diff.Div(original).InexactFloat64() * 10000
}

// Abs returns the absolute value of a float64.
func absFloat(f float64) float64 {
	return math.Abs(f)
}
