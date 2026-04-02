package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestPositionApplyFill(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		position      Position
		fill          Fill
		side          OrderSide
		wantErr       bool
		wantQty       decimal.Decimal
		wantAvgCost   decimal.Decimal
		wantRealized  decimal.Decimal
	}{
		{
			name: "open long position with buy",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.Zero,
				AverageCost:  decimal.Zero,
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-1",
				Quantity:  decimal.NewFromInt(10),
				Price:     decimal.NewFromFloat(100.0),
				Timestamp: now,
			},
			side:         SideBuy,
			wantErr:      false,
			wantQty:      decimal.NewFromInt(10),
			wantAvgCost:  decimal.NewFromFloat(100.0),
			wantRealized: decimal.Zero,
		},
		{
			name: "add to long position with buy",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.NewFromInt(10),
				AverageCost:  decimal.NewFromFloat(100.0),
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-2",
				Quantity:  decimal.NewFromInt(5),
				Price:     decimal.NewFromFloat(110.0),
				Timestamp: now,
			},
			side:    SideBuy,
			wantErr: false,
			wantQty: decimal.NewFromInt(15),
			// VWAP: (100*10 + 110*5) / 15 = 1550/15 = 103.333...
			wantAvgCost:  decimal.RequireFromString("103.3333333333333333"),
			wantRealized: decimal.Zero,
		},
		{
			name: "partial close long with sell — realized PnL",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.NewFromInt(10),
				AverageCost:  decimal.NewFromFloat(100.0),
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-3",
				Quantity:  decimal.NewFromInt(5),
				Price:     decimal.NewFromFloat(120.0),
				Timestamp: now,
			},
			side:    SideSell,
			wantErr: false,
			wantQty: decimal.NewFromInt(5),
			// avg cost unchanged when reducing position
			wantAvgCost: decimal.NewFromFloat(100.0),
			// realized: (120 - 100) * 5 = 100
			wantRealized: decimal.NewFromInt(100),
		},
		{
			name: "full close long with sell",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.NewFromInt(10),
				AverageCost:  decimal.NewFromFloat(100.0),
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-4",
				Quantity:  decimal.NewFromInt(10),
				Price:     decimal.NewFromFloat(90.0),
				Timestamp: now,
			},
			side:    SideSell,
			wantErr: false,
			wantQty: decimal.Zero,
			// avg cost stays
			wantAvgCost: decimal.NewFromFloat(100.0),
			// realized: (90 - 100) * 10 = -100
			wantRealized: decimal.NewFromInt(-100),
		},
		{
			name: "open short position with sell",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.Zero,
				AverageCost:  decimal.Zero,
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-5",
				Quantity:  decimal.NewFromInt(10),
				Price:     decimal.NewFromFloat(200.0),
				Timestamp: now,
			},
			side:         SideSell,
			wantErr:      false,
			wantQty:      decimal.NewFromInt(-10),
			wantAvgCost:  decimal.NewFromFloat(200.0),
			wantRealized: decimal.Zero,
		},
		{
			name: "close short position with buy — realized PnL",
			position: Position{
				InstrumentID: "BTC-USD",
				Quantity:     decimal.NewFromInt(-10),
				AverageCost:  decimal.NewFromFloat(200.0),
				RealizedPnL:  decimal.Zero,
			},
			fill: Fill{
				ID:        "fill-6",
				Quantity:  decimal.NewFromInt(10),
				Price:     decimal.NewFromFloat(180.0),
				Timestamp: now,
			},
			side:    SideBuy,
			wantErr: false,
			wantQty: decimal.Zero,
			// avg cost stays
			wantAvgCost: decimal.NewFromFloat(200.0),
			// short realized: (avgCost - fillPrice) * qty = (200 - 180) * 10 = 200
			wantRealized: decimal.NewFromInt(200),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := tt.position
			err := pos.ApplyFill(tt.fill, tt.side)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyFill() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if !pos.Quantity.Equal(tt.wantQty) {
				t.Errorf("Quantity = %s, want %s", pos.Quantity, tt.wantQty)
			}
			if !pos.AverageCost.Equal(tt.wantAvgCost) {
				t.Errorf("AverageCost = %s, want %s", pos.AverageCost, tt.wantAvgCost)
			}
			if !pos.RealizedPnL.Equal(tt.wantRealized) {
				t.Errorf("RealizedPnL = %s, want %s", pos.RealizedPnL, tt.wantRealized)
			}
		})
	}
}
