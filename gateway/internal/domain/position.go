package domain

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// Position represents a holding in a specific instrument at a specific venue.
type Position struct {
	InstrumentID      string
	VenueID           string
	Quantity          decimal.Decimal
	AverageCost       decimal.Decimal
	MarketPrice       decimal.Decimal
	UnrealizedPnL     decimal.Decimal
	RealizedPnL       decimal.Decimal
	UnsettledQuantity decimal.Decimal
	SettledQuantity   decimal.Decimal
	AssetClass        AssetClass
	QuoteCurrency     string
	UpdatedAt         time.Time
}

var ErrInvalidFillQuantity = errors.New("fill quantity must be positive")

// ApplyFill updates the position based on a fill and order side.
func (p *Position) ApplyFill(fill Fill, side OrderSide) error {
	if !fill.Quantity.IsPositive() {
		return ErrInvalidFillQuantity
	}

	signedQty := fill.Quantity
	if side == SideSell {
		signedQty = signedQty.Neg()
	}

	// Determine if this fill increases or reduces the position
	isIncreasing := p.Quantity.IsZero() ||
		(p.Quantity.IsPositive() && side == SideBuy) ||
		(p.Quantity.IsNegative() && side == SideSell)

	if isIncreasing {
		// Increasing position: update average cost via VWAP
		absOld := p.Quantity.Abs()
		totalCost := p.AverageCost.Mul(absOld).Add(fill.Price.Mul(fill.Quantity))
		newAbsQty := absOld.Add(fill.Quantity)
		p.AverageCost = totalCost.Div(newAbsQty)
		p.Quantity = p.Quantity.Add(signedQty)
	} else {
		// Reducing position: compute realized P&L
		if p.Quantity.IsPositive() {
			// Long position being sold: realized = (fillPrice - avgCost) * fillQty
			pnl := fill.Price.Sub(p.AverageCost).Mul(fill.Quantity)
			p.RealizedPnL = p.RealizedPnL.Add(pnl)
		} else {
			// Short position being bought: realized = (avgCost - fillPrice) * fillQty
			pnl := p.AverageCost.Sub(fill.Price).Mul(fill.Quantity)
			p.RealizedPnL = p.RealizedPnL.Add(pnl)
		}
		p.Quantity = p.Quantity.Add(signedQty)
		// Average cost stays the same when reducing
	}

	p.UnsettledQuantity = p.UnsettledQuantity.Add(fill.Quantity)
	p.UpdatedAt = time.Now()
	return nil
}
