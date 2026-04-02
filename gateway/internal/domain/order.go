package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// OrderID is a unique identifier for an order.
type OrderID string

// OrderSide represents buy or sell.
type OrderSide int

const (
	SideBuy OrderSide = iota
	SideSell
)

// OrderType represents the type of order.
type OrderType int

const (
	OrderTypeMarket OrderType = iota
	OrderTypeLimit
	OrderTypeStopLimit
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus int

const (
	OrderStatusNew OrderStatus = iota
	OrderStatusAcknowledged
	OrderStatusPartiallyFilled
	OrderStatusFilled
	OrderStatusCanceled
	OrderStatusRejected
)

// Order represents a trading order with state machine semantics.
type Order struct {
	ID              OrderID
	ClientOrderID   string
	InstrumentID    string
	Side            OrderSide
	Type            OrderType
	Quantity        decimal.Decimal
	Price           decimal.Decimal
	FilledQuantity  decimal.Decimal
	AveragePrice    decimal.Decimal
	Status          OrderStatus
	VenueID         string
	AssetClass      AssetClass
	SettlementCycle SettlementCycle
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Fills           []Fill
}

// validTransitions defines the allowed state machine transitions.
var validTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusNew:             {OrderStatusAcknowledged, OrderStatusRejected},
	OrderStatusAcknowledged:    {OrderStatusPartiallyFilled, OrderStatusCanceled, OrderStatusRejected},
	OrderStatusPartiallyFilled: {OrderStatusFilled, OrderStatusCanceled},
}

var (
	ErrInvalidTransition = errors.New("invalid order status transition")
	ErrOverfill          = errors.New("fill quantity exceeds remaining order quantity")
	ErrOrderNotFillable  = errors.New("order is not in a fillable state")
)

// ApplyTransition transitions the order to a new status if the transition is valid.
func (o *Order) ApplyTransition(newStatus OrderStatus) error {
	allowed, ok := validTransitions[o.Status]
	if !ok {
		return fmt.Errorf("%w: %d -> %d", ErrInvalidTransition, o.Status, newStatus)
	}

	for _, s := range allowed {
		if s == newStatus {
			o.Status = newStatus
			o.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("%w: %d -> %d", ErrInvalidTransition, o.Status, newStatus)
}

// ApplyFill applies a fill to the order, updating filled quantity, VWAP, and status.
func (o *Order) ApplyFill(fill Fill) error {
	if o.Status != OrderStatusAcknowledged && o.Status != OrderStatusPartiallyFilled {
		return fmt.Errorf("%w: status=%d", ErrOrderNotFillable, o.Status)
	}

	remaining := o.Quantity.Sub(o.FilledQuantity)
	if fill.Quantity.GreaterThan(remaining) {
		return fmt.Errorf("%w: fill=%s remaining=%s", ErrOverfill, fill.Quantity, remaining)
	}

	// VWAP: newAvg = (oldAvg * oldQty + fillPrice * fillQty) / (oldQty + fillQty)
	newFilledQty := o.FilledQuantity.Add(fill.Quantity)
	if o.FilledQuantity.IsZero() {
		o.AveragePrice = fill.Price
	} else {
		totalCost := o.AveragePrice.Mul(o.FilledQuantity).Add(fill.Price.Mul(fill.Quantity))
		o.AveragePrice = totalCost.Div(newFilledQty)
	}
	o.FilledQuantity = newFilledQty
	o.Fills = append(o.Fills, fill)

	// Transition status
	if o.FilledQuantity.Equal(o.Quantity) {
		o.Status = OrderStatusFilled
	} else {
		o.Status = OrderStatusPartiallyFilled
	}

	o.UpdatedAt = time.Now()
	return nil
}
