package crossing

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

// CrossResult contains the outcome of a crossing attempt.
type CrossResult struct {
	Crossed       bool
	Fills         []domain.Fill
	ResidualOrder *domain.Order
}

// CrossingEngine maintains an internal book of resting orders and attempts
// to cross incoming orders against opposite-side resting orders for the same
// instrument. Crossing happens at the midpoint price, with zero fees.
type CrossingEngine struct {
	mu   sync.RWMutex
	book map[string][]*domain.Order // keyed by instrumentID
}

// NewCrossingEngine creates a new crossing engine with an empty book.
func NewCrossingEngine() *CrossingEngine {
	return &CrossingEngine{
		book: make(map[string][]*domain.Order),
	}
}

// AddOrder adds a resting order to the internal crossing book.
func (e *CrossingEngine) AddOrder(order *domain.Order) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.book[order.InstrumentID] = append(e.book[order.InstrumentID], order)
}

// TryCross checks if an opposite-side order exists for the same instrument at
// a crossable price. If found, it generates internal fills at the midpoint price.
func (e *CrossingEngine) TryCross(order *domain.Order) (*CrossResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	resting := e.book[order.InstrumentID]
	if len(resting) == 0 {
		return &CrossResult{Crossed: false}, nil
	}

	// Find the first opposite-side order that is crossable.
	for i, candidate := range resting {
		if candidate.Side == order.Side {
			continue
		}

		if !canCross(order, candidate) {
			continue
		}

		crossPrice := midpointPrice(order, candidate)

		incomingRemaining := order.Quantity.Sub(order.FilledQuantity)
		candidateRemaining := candidate.Quantity.Sub(candidate.FilledQuantity)

		fillQty := decimal.Min(incomingRemaining, candidateRemaining)

		now := time.Now()

		// Fill for the incoming order.
		incomingFill := domain.Fill{
			ID:        generateID(),
			OrderID:   order.ID,
			VenueID:   "INTERNAL",
			Quantity:  fillQty,
			Price:     crossPrice,
			Fee:       decimal.Zero,
			FeeModel:  domain.FeeModelPerShare, // irrelevant at zero fee
			Liquidity: domain.LiquidityInternal,
			Timestamp: now,
		}

		// Fill for the resting order.
		candidateFill := domain.Fill{
			ID:        generateID(),
			OrderID:   candidate.ID,
			VenueID:   "INTERNAL",
			Quantity:  fillQty,
			Price:     crossPrice,
			Fee:       decimal.Zero,
			FeeModel:  domain.FeeModelPerShare,
			Liquidity: domain.LiquidityInternal,
			Timestamp: now,
		}

		fills := []domain.Fill{incomingFill, candidateFill}

		// Update candidate remaining; remove from book if fully filled.
		candidate.FilledQuantity = candidate.FilledQuantity.Add(fillQty)
		if candidate.FilledQuantity.Equal(candidate.Quantity) {
			e.book[order.InstrumentID] = append(resting[:i], resting[i+1:]...)
		}

		// Determine if incoming order has residual.
		order.FilledQuantity = order.FilledQuantity.Add(fillQty)
		if order.FilledQuantity.LessThan(order.Quantity) {
			residual := *order // shallow copy
			return &CrossResult{
				Crossed:       true,
				Fills:         fills,
				ResidualOrder: &residual,
			}, nil
		}

		return &CrossResult{
			Crossed: true,
			Fills:   fills,
		}, nil
	}

	return &CrossResult{Crossed: false}, nil
}

// canCross checks whether two orders can cross based on price.
// Market orders always cross. Limit buy must be >= limit sell price.
func canCross(incoming, resting *domain.Order) bool {
	// If either is a market order, they can always cross.
	if incoming.Type == domain.OrderTypeMarket || resting.Type == domain.OrderTypeMarket {
		return true
	}

	// Both are limit orders: buy price must be >= sell price.
	var buyPrice, sellPrice decimal.Decimal
	if incoming.Side == domain.SideBuy {
		buyPrice = incoming.Price
		sellPrice = resting.Price
	} else {
		buyPrice = resting.Price
		sellPrice = incoming.Price
	}

	return buyPrice.GreaterThanOrEqual(sellPrice)
}

// midpointPrice calculates the crossing price as the midpoint of the two
// orders' limit prices. If both are market orders, returns zero (caller
// should use last traded price in production).
func midpointPrice(a, b *domain.Order) decimal.Decimal {
	if a.Type == domain.OrderTypeMarket && b.Type == domain.OrderTypeMarket {
		return decimal.Zero
	}
	if a.Type == domain.OrderTypeMarket {
		return b.Price
	}
	if b.Type == domain.OrderTypeMarket {
		return a.Price
	}
	return a.Price.Add(b.Price).Div(decimal.NewFromInt(2))
}

// generateID creates a random UUID v4 string.
func generateID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
