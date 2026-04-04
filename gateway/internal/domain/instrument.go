package domain

import "github.com/shopspring/decimal"

// AssetClass represents the class of a financial instrument.
type AssetClass int

const (
	AssetClassEquity AssetClass = iota
	AssetClassCrypto
	AssetClassTokenizedSecurity
	AssetClassFuture
	AssetClassOption
)

// String returns the lowercase string representation of an AssetClass.
func (a AssetClass) String() string {
	switch a {
	case AssetClassEquity:
		return "equity"
	case AssetClassCrypto:
		return "crypto"
	case AssetClassTokenizedSecurity:
		return "tokenized_security"
	case AssetClassFuture:
		return "future"
	case AssetClassOption:
		return "option"
	default:
		return "equity"
	}
}

// SettlementCycle represents the settlement period.
type SettlementCycle int

const (
	SettlementT0 SettlementCycle = iota
	SettlementT1
	SettlementT2
)

// TradingSchedule defines trading hours for an instrument.
type TradingSchedule struct {
	Is24x7      bool
	MarketOpen  string // "09:30" ET
	MarketClose string // "16:00" ET
	PreMarket   string // "04:00" ET
	AfterHours  string // "20:00" ET
	Timezone    string // "America/New_York"
}

// Instrument represents a tradeable financial instrument.
type Instrument struct {
	ID              string
	Symbol          string
	Name            string
	AssetClass      AssetClass
	QuoteCurrency   string
	BaseCurrency    string
	TickSize        decimal.Decimal
	LotSize         decimal.Decimal
	SettlementCycle SettlementCycle
	TradingHours    TradingSchedule
	Venues          []string
	MarginRequired  decimal.Decimal
}
