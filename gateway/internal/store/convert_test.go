package store

import (
	"testing"

	"github.com/synapse-oms/gateway/internal/domain"
)

func TestOrderSideRoundTrip(t *testing.T) {
	tests := []struct {
		side domain.OrderSide
		str  string
	}{
		{domain.SideBuy, "buy"},
		{domain.SideSell, "sell"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := orderSideToString(tt.side)
			if got != tt.str {
				t.Errorf("orderSideToString(%d) = %q, want %q", tt.side, got, tt.str)
			}
			back := stringToOrderSide(got)
			if back != tt.side {
				t.Errorf("stringToOrderSide(%q) = %d, want %d", got, back, tt.side)
			}
		})
	}
}

func TestOrderSideDefault(t *testing.T) {
	if got := stringToOrderSide("unknown"); got != domain.SideBuy {
		t.Errorf("expected SideBuy for unknown, got %d", got)
	}
	if got := orderSideToString(domain.OrderSide(99)); got != "buy" {
		t.Errorf("expected 'buy' for unknown side, got %q", got)
	}
}

func TestOrderTypeRoundTrip(t *testing.T) {
	tests := []struct {
		typ domain.OrderType
		str string
	}{
		{domain.OrderTypeMarket, "market"},
		{domain.OrderTypeLimit, "limit"},
		{domain.OrderTypeStopLimit, "stop_limit"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := orderTypeToString(tt.typ)
			if got != tt.str {
				t.Errorf("orderTypeToString(%d) = %q, want %q", tt.typ, got, tt.str)
			}
			back := stringToOrderType(got)
			if back != tt.typ {
				t.Errorf("stringToOrderType(%q) = %d, want %d", got, back, tt.typ)
			}
		})
	}
}

func TestOrderTypeDefault(t *testing.T) {
	if got := stringToOrderType("unknown"); got != domain.OrderTypeMarket {
		t.Errorf("expected OrderTypeMarket for unknown, got %d", got)
	}
}

func TestOrderStatusRoundTrip(t *testing.T) {
	tests := []struct {
		status domain.OrderStatus
		str    string
	}{
		{domain.OrderStatusNew, "new"},
		{domain.OrderStatusAcknowledged, "acknowledged"},
		{domain.OrderStatusPartiallyFilled, "partially_filled"},
		{domain.OrderStatusFilled, "filled"},
		{domain.OrderStatusCanceled, "canceled"},
		{domain.OrderStatusRejected, "rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := orderStatusToString(tt.status)
			if got != tt.str {
				t.Errorf("orderStatusToString(%d) = %q, want %q", tt.status, got, tt.str)
			}
			back := stringToOrderStatus(got)
			if back != tt.status {
				t.Errorf("stringToOrderStatus(%q) = %d, want %d", got, back, tt.status)
			}
		})
	}
}

func TestOrderStatusDefault(t *testing.T) {
	if got := stringToOrderStatus("unknown"); got != domain.OrderStatusNew {
		t.Errorf("expected OrderStatusNew for unknown, got %d", got)
	}
}

func TestAssetClassRoundTrip(t *testing.T) {
	tests := []struct {
		ac  domain.AssetClass
		str string
	}{
		{domain.AssetClassEquity, "equity"},
		{domain.AssetClassCrypto, "crypto"},
		{domain.AssetClassTokenizedSecurity, "tokenized_security"},
		{domain.AssetClassFuture, "future"},
		{domain.AssetClassOption, "option"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := assetClassToString(tt.ac)
			if got != tt.str {
				t.Errorf("assetClassToString(%d) = %q, want %q", tt.ac, got, tt.str)
			}
			back := stringToAssetClass(got)
			if back != tt.ac {
				t.Errorf("stringToAssetClass(%q) = %d, want %d", got, back, tt.ac)
			}
		})
	}
}

func TestAssetClassDefault(t *testing.T) {
	if got := stringToAssetClass("unknown"); got != domain.AssetClassEquity {
		t.Errorf("expected AssetClassEquity for unknown, got %d", got)
	}
}

func TestSettlementCycleRoundTrip(t *testing.T) {
	tests := []struct {
		cycle domain.SettlementCycle
		str   string
	}{
		{domain.SettlementT0, "T+0"},
		{domain.SettlementT1, "T+1"},
		{domain.SettlementT2, "T+2"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := settlementCycleToString(tt.cycle)
			if got != tt.str {
				t.Errorf("settlementCycleToString(%d) = %q, want %q", tt.cycle, got, tt.str)
			}
			back := stringToSettlementCycle(got)
			if back != tt.cycle {
				t.Errorf("stringToSettlementCycle(%q) = %d, want %d", got, back, tt.cycle)
			}
		})
	}
}

func TestSettlementCycleDefault(t *testing.T) {
	if got := stringToSettlementCycle("unknown"); got != domain.SettlementT0 {
		t.Errorf("expected SettlementT0 for unknown, got %d", got)
	}
}

func TestLiquidityTypeRoundTrip(t *testing.T) {
	tests := []struct {
		lt  domain.LiquidityType
		str string
	}{
		{domain.LiquidityMaker, "maker"},
		{domain.LiquidityTaker, "taker"},
		{domain.LiquidityInternal, "internal"},
	}
	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			got := liquidityToString(tt.lt)
			if got != tt.str {
				t.Errorf("liquidityToString(%d) = %q, want %q", tt.lt, got, tt.str)
			}
			back := stringToLiquidity(got)
			if back != tt.lt {
				t.Errorf("stringToLiquidity(%q) = %d, want %d", got, back, tt.lt)
			}
		})
	}
}

func TestLiquidityTypeDefault(t *testing.T) {
	if got := stringToLiquidity("unknown"); got != domain.LiquidityTaker {
		t.Errorf("expected LiquidityTaker for unknown, got %d", got)
	}
}
