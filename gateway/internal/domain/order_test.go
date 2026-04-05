package domain

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestApplyTransition(t *testing.T) {
	tests := []struct {
		name      string
		from      OrderStatus
		to        OrderStatus
		wantErr   bool
	}{
		// Valid transitions
		{"New to Acknowledged", OrderStatusNew, OrderStatusAcknowledged, false},
		{"New to Rejected", OrderStatusNew, OrderStatusRejected, false},
		{"Acknowledged to PartiallyFilled", OrderStatusAcknowledged, OrderStatusPartiallyFilled, false},
		{"Acknowledged to Canceled", OrderStatusAcknowledged, OrderStatusCanceled, false},
		{"Acknowledged to Rejected", OrderStatusAcknowledged, OrderStatusRejected, false},
		{"PartiallyFilled to Filled", OrderStatusPartiallyFilled, OrderStatusFilled, false},
		{"PartiallyFilled to Canceled", OrderStatusPartiallyFilled, OrderStatusCanceled, false},

		// Invalid transitions
		{"New to Filled", OrderStatusNew, OrderStatusFilled, true},
		{"New to PartiallyFilled", OrderStatusNew, OrderStatusPartiallyFilled, true},
		{"New to Canceled", OrderStatusNew, OrderStatusCanceled, true},
		{"Acknowledged to Filled", OrderStatusAcknowledged, OrderStatusFilled, true},
		{"Filled to anything", OrderStatusFilled, OrderStatusCanceled, true},
		{"Canceled to anything", OrderStatusCanceled, OrderStatusNew, true},
		{"Rejected to anything", OrderStatusRejected, OrderStatusNew, true},
		{"PartiallyFilled to Acknowledged", OrderStatusPartiallyFilled, OrderStatusAcknowledged, true},
		{"Same status New to New", OrderStatusNew, OrderStatusNew, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := &Order{
				ID:       "test-order",
				Status:   tt.from,
				Quantity: decimal.NewFromInt(100),
			}
			err := order.ApplyTransition(tt.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyTransition(%d -> %d) error = %v, wantErr %v", tt.from, tt.to, err, tt.wantErr)
			}
			if err == nil && order.Status != tt.to {
				t.Errorf("ApplyTransition(%d -> %d) status = %d, want %d", tt.from, tt.to, order.Status, tt.to)
			}
		})
	}
}

func TestApplyFill(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name           string
		order          *Order
		fill           Fill
		wantErr        bool
		wantStatus     OrderStatus
		wantFilledQty  decimal.Decimal
		wantAvgPrice   decimal.Decimal
	}{
		{
			name: "partial fill on acknowledged order",
			order: &Order{
				ID:             "ord-1",
				Status:         OrderStatusAcknowledged,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.Zero,
				AveragePrice:   decimal.Zero,
			},
			fill: Fill{
				ID:       "fill-1",
				OrderID:  "ord-1",
				Quantity: decimal.NewFromInt(30),
				Price:    decimal.NewFromFloat(50.0),
				Timestamp: now,
			},
			wantErr:       false,
			wantStatus:    OrderStatusPartiallyFilled,
			wantFilledQty: decimal.NewFromInt(30),
			wantAvgPrice:  decimal.NewFromFloat(50.0),
		},
		{
			name: "second partial fill with VWAP",
			order: &Order{
				ID:             "ord-2",
				Status:         OrderStatusPartiallyFilled,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.NewFromInt(30),
				AveragePrice:   decimal.NewFromFloat(50.0),
			},
			fill: Fill{
				ID:       "fill-2",
				OrderID:  "ord-2",
				Quantity: decimal.NewFromInt(20),
				Price:    decimal.NewFromFloat(55.0),
				Timestamp: now,
			},
			wantErr:       false,
			wantStatus:    OrderStatusPartiallyFilled,
			wantFilledQty: decimal.NewFromInt(50),
			// VWAP: (50*30 + 55*20) / 50 = (1500 + 1100) / 50 = 52
			wantAvgPrice: decimal.NewFromInt(52),
		},
		{
			name: "full fill completes order",
			order: &Order{
				ID:             "ord-3",
				Status:         OrderStatusPartiallyFilled,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.NewFromInt(70),
				AveragePrice:   decimal.NewFromFloat(50.0),
			},
			fill: Fill{
				ID:       "fill-3",
				OrderID:  "ord-3",
				Quantity: decimal.NewFromInt(30),
				Price:    decimal.NewFromFloat(52.0),
				Timestamp: now,
			},
			wantErr:       false,
			wantStatus:    OrderStatusFilled,
			wantFilledQty: decimal.NewFromInt(100),
			// VWAP: (50*70 + 52*30) / 100 = (3500 + 1560) / 100 = 50.6
			wantAvgPrice: decimal.RequireFromString("50.6"),
		},
		{
			name: "overfill rejected",
			order: &Order{
				ID:             "ord-4",
				Status:         OrderStatusPartiallyFilled,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.NewFromInt(90),
				AveragePrice:   decimal.NewFromFloat(50.0),
			},
			fill: Fill{
				ID:       "fill-4",
				OrderID:  "ord-4",
				Quantity: decimal.NewFromInt(20),
				Price:    decimal.NewFromFloat(50.0),
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "fill on new order rejected",
			order: &Order{
				ID:             "ord-5",
				Status:         OrderStatusNew,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.Zero,
				AveragePrice:   decimal.Zero,
			},
			fill: Fill{
				ID:       "fill-5",
				OrderID:  "ord-5",
				Quantity: decimal.NewFromInt(10),
				Price:    decimal.NewFromFloat(50.0),
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "fill on filled order rejected",
			order: &Order{
				ID:             "ord-6",
				Status:         OrderStatusFilled,
				Quantity:       decimal.NewFromInt(100),
				FilledQuantity: decimal.NewFromInt(100),
				AveragePrice:   decimal.NewFromFloat(50.0),
			},
			fill: Fill{
				ID:       "fill-6",
				OrderID:  "ord-6",
				Quantity: decimal.NewFromInt(10),
				Price:    decimal.NewFromFloat(50.0),
				Timestamp: now,
			},
			wantErr: true,
		},
		{
			name: "full fill from acknowledged",
			order: &Order{
				ID:             "ord-7",
				Status:         OrderStatusAcknowledged,
				Quantity:       decimal.NewFromInt(50),
				FilledQuantity: decimal.Zero,
				AveragePrice:   decimal.Zero,
			},
			fill: Fill{
				ID:       "fill-7",
				OrderID:  "ord-7",
				Quantity: decimal.NewFromInt(50),
				Price:    decimal.NewFromFloat(100.0),
				Timestamp: now,
			},
			wantErr:       false,
			wantStatus:    OrderStatusFilled,
			wantFilledQty: decimal.NewFromInt(50),
			wantAvgPrice:  decimal.NewFromFloat(100.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := tt.order
			err := order.ApplyFill(tt.fill)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyFill() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if order.Status != tt.wantStatus {
				t.Errorf("ApplyFill() status = %d, want %d", order.Status, tt.wantStatus)
			}
			if !order.FilledQuantity.Equal(tt.wantFilledQty) {
				t.Errorf("ApplyFill() filledQty = %s, want %s", order.FilledQuantity, tt.wantFilledQty)
			}
			if !order.AveragePrice.Equal(tt.wantAvgPrice) {
				t.Errorf("ApplyFill() avgPrice = %s, want %s", order.AveragePrice, tt.wantAvgPrice)
			}
			// Verify fill was appended
			found := false
			for _, f := range order.Fills {
				if f.ID == tt.fill.ID {
					found = true
					break
				}
			}
			if !found {
				t.Error("ApplyFill() fill not appended to order.Fills")
			}
		})
	}
}
