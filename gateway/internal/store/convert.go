package store

import (
	"github.com/synapse-oms/gateway/internal/domain"
)

// --- OrderSide ---

func orderSideToString(s domain.OrderSide) string {
	switch s {
	case domain.SideBuy:
		return "buy"
	case domain.SideSell:
		return "sell"
	default:
		return "buy"
	}
}

func stringToOrderSide(s string) domain.OrderSide {
	switch s {
	case "buy":
		return domain.SideBuy
	case "sell":
		return domain.SideSell
	default:
		return domain.SideBuy
	}
}

// --- OrderType ---

func orderTypeToString(t domain.OrderType) string {
	switch t {
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

func stringToOrderType(s string) domain.OrderType {
	switch s {
	case "market":
		return domain.OrderTypeMarket
	case "limit":
		return domain.OrderTypeLimit
	case "stop_limit":
		return domain.OrderTypeStopLimit
	default:
		return domain.OrderTypeMarket
	}
}

// --- OrderStatus ---

func orderStatusToString(s domain.OrderStatus) string {
	switch s {
	case domain.OrderStatusNew:
		return "new"
	case domain.OrderStatusAcknowledged:
		return "acknowledged"
	case domain.OrderStatusPartiallyFilled:
		return "partially_filled"
	case domain.OrderStatusFilled:
		return "filled"
	case domain.OrderStatusCanceled:
		return "canceled"
	case domain.OrderStatusRejected:
		return "rejected"
	default:
		return "new"
	}
}

func stringToOrderStatus(s string) domain.OrderStatus {
	switch s {
	case "new":
		return domain.OrderStatusNew
	case "acknowledged":
		return domain.OrderStatusAcknowledged
	case "partially_filled":
		return domain.OrderStatusPartiallyFilled
	case "filled":
		return domain.OrderStatusFilled
	case "canceled":
		return domain.OrderStatusCanceled
	case "rejected":
		return domain.OrderStatusRejected
	default:
		return domain.OrderStatusNew
	}
}

// --- AssetClass ---

func assetClassToString(a domain.AssetClass) string {
	switch a {
	case domain.AssetClassEquity:
		return "equity"
	case domain.AssetClassCrypto:
		return "crypto"
	case domain.AssetClassTokenizedSecurity:
		return "tokenized_security"
	case domain.AssetClassFuture:
		return "future"
	case domain.AssetClassOption:
		return "option"
	default:
		return "equity"
	}
}

func stringToAssetClass(s string) domain.AssetClass {
	switch s {
	case "equity":
		return domain.AssetClassEquity
	case "crypto":
		return domain.AssetClassCrypto
	case "tokenized_security":
		return domain.AssetClassTokenizedSecurity
	case "future":
		return domain.AssetClassFuture
	case "option":
		return domain.AssetClassOption
	default:
		return domain.AssetClassEquity
	}
}

// --- SettlementCycle ---

func settlementCycleToString(c domain.SettlementCycle) string {
	switch c {
	case domain.SettlementT0:
		return "T+0"
	case domain.SettlementT1:
		return "T+1"
	case domain.SettlementT2:
		return "T+2"
	default:
		return "T+0"
	}
}

func stringToSettlementCycle(s string) domain.SettlementCycle {
	switch s {
	case "T+0":
		return domain.SettlementT0
	case "T+1":
		return domain.SettlementT1
	case "T+2":
		return domain.SettlementT2
	default:
		return domain.SettlementT0
	}
}

// --- LiquidityType ---

func liquidityToString(l domain.LiquidityType) string {
	switch l {
	case domain.LiquidityMaker:
		return "maker"
	case domain.LiquidityTaker:
		return "taker"
	case domain.LiquidityInternal:
		return "internal"
	default:
		return "taker"
	}
}

func stringToLiquidity(s string) domain.LiquidityType {
	switch s {
	case "maker":
		return domain.LiquidityMaker
	case "taker":
		return domain.LiquidityTaker
	case "internal":
		return domain.LiquidityInternal
	default:
		return domain.LiquidityTaker
	}
}

