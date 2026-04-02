package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// FeeModel represents how fees are calculated.
type FeeModel int

const (
	FeeModelPerShare FeeModel = iota
	FeeModelPercentage
)

// LiquidityType represents the liquidity classification of a fill.
type LiquidityType int

const (
	LiquidityMaker LiquidityType = iota
	LiquidityTaker
	LiquidityInternal
)

// Fill represents a single execution against an order.
type Fill struct {
	ID          string
	OrderID     OrderID
	VenueID     string
	Quantity    decimal.Decimal
	Price       decimal.Decimal
	Fee         decimal.Decimal
	FeeAsset    string
	FeeModel    FeeModel
	Liquidity   LiquidityType
	Timestamp   time.Time
	VenueExecID string
}
