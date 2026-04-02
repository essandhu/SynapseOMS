package crossing

import (
	"sync"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
)

func newOrder(id string, side domain.OrderSide, typ domain.OrderType, instrument string, qty, price float64) *domain.Order {
	return &domain.Order{
		ID:           domain.OrderID(id),
		InstrumentID: instrument,
		Side:         side,
		Type:         typ,
		Quantity:     decimal.NewFromFloat(qty),
		Price:        decimal.NewFromFloat(price),
	}
}

func TestFullCross_BuySell_MidpointPrice(t *testing.T) {
	eng := NewCrossingEngine()

	// Resting sell at 100.20
	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 100, 100.20)
	eng.AddOrder(sell)

	// Incoming buy at 100.40
	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.40)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Crossed {
		t.Fatal("expected cross")
	}
	if result.ResidualOrder != nil {
		t.Fatal("expected full cross, got residual")
	}
	if len(result.Fills) != 2 {
		t.Fatalf("expected 2 fills, got %d", len(result.Fills))
	}

	expectedPrice := decimal.NewFromFloat(100.30) // midpoint
	for i, fill := range result.Fills {
		if !fill.Price.Equal(expectedPrice) {
			t.Errorf("fill[%d] price = %s, want %s", i, fill.Price, expectedPrice)
		}
		if !fill.Quantity.Equal(decimal.NewFromFloat(100)) {
			t.Errorf("fill[%d] quantity = %s, want 100", i, fill.Quantity)
		}
		if !fill.Fee.IsZero() {
			t.Errorf("fill[%d] fee = %s, want 0", i, fill.Fee)
		}
		if fill.Liquidity != domain.LiquidityInternal {
			t.Errorf("fill[%d] liquidity = %d, want LiquidityInternal", i, fill.Liquidity)
		}
		if fill.VenueID != "INTERNAL" {
			t.Errorf("fill[%d] venueID = %s, want INTERNAL", i, fill.VenueID)
		}
	}
}

func TestNoOppositeSide_NoCross(t *testing.T) {
	eng := NewCrossingEngine()

	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Crossed {
		t.Fatal("expected no cross on empty book")
	}
}

func TestSameSideOrders_NoCross(t *testing.T) {
	eng := NewCrossingEngine()

	// Add a resting buy
	restingBuy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	eng.AddOrder(restingBuy)

	// Incoming buy should not cross
	incomingBuy := newOrder("buy-2", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 50, 100.00)
	result, err := eng.TryCross(incomingBuy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Crossed {
		t.Fatal("expected no cross for same-side orders")
	}
}

func TestPartialCross_ResidualReturned(t *testing.T) {
	eng := NewCrossingEngine()

	// Resting sell for 50 shares
	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 50, 100.00)
	eng.AddOrder(sell)

	// Incoming buy for 100 shares
	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.20)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Crossed {
		t.Fatal("expected cross")
	}
	if result.ResidualOrder == nil {
		t.Fatal("expected residual order")
	}

	residualRemaining := result.ResidualOrder.Quantity.Sub(result.ResidualOrder.FilledQuantity)
	if !residualRemaining.Equal(decimal.NewFromFloat(50)) {
		t.Errorf("residual remaining = %s, want 50", residualRemaining)
	}

	// Fills should be for 50 shares
	for i, fill := range result.Fills {
		if !fill.Quantity.Equal(decimal.NewFromFloat(50)) {
			t.Errorf("fill[%d] quantity = %s, want 50", i, fill.Quantity)
		}
	}
}

func TestPartialCross_LargerRestingOrder(t *testing.T) {
	eng := NewCrossingEngine()

	// Resting sell for 200 shares
	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 200, 50.00)
	eng.AddOrder(sell)

	// Incoming buy for 75 shares — fully fills, resting order stays with residual
	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 75, 50.10)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Crossed {
		t.Fatal("expected cross")
	}
	if result.ResidualOrder != nil {
		t.Fatal("expected no residual for incoming order (fully filled)")
	}

	// Resting order should still be in the book with reduced quantity
	eng.mu.RLock()
	defer eng.mu.RUnlock()
	orders := eng.book["AAPL"]
	if len(orders) != 1 {
		t.Fatalf("expected 1 resting order in book, got %d", len(orders))
	}
	restRemaining := orders[0].Quantity.Sub(orders[0].FilledQuantity)
	if !restRemaining.Equal(decimal.NewFromFloat(125)) {
		t.Errorf("resting remaining = %s, want 125", restRemaining)
	}
}

func TestMarketOrderCross(t *testing.T) {
	eng := NewCrossingEngine()

	// Resting limit sell at 50.00
	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 100, 50.00)
	eng.AddOrder(sell)

	// Incoming market buy — should cross at the limit order's price
	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeMarket, "AAPL", 100, 0)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Crossed {
		t.Fatal("expected cross")
	}
	expectedPrice := decimal.NewFromFloat(50.00)
	if !result.Fills[0].Price.Equal(expectedPrice) {
		t.Errorf("price = %s, want %s", result.Fills[0].Price, expectedPrice)
	}
}

func TestBothMarketOrders_CrossAtZero(t *testing.T) {
	eng := NewCrossingEngine()

	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeMarket, "AAPL", 100, 0)
	eng.AddOrder(sell)

	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeMarket, "AAPL", 100, 0)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Crossed {
		t.Fatal("expected cross")
	}
	if !result.Fills[0].Price.IsZero() {
		t.Errorf("both market orders should cross at zero, got %s", result.Fills[0].Price)
	}
}

func TestNoCross_PriceDoesNotOverlap(t *testing.T) {
	eng := NewCrossingEngine()

	// Resting sell at 105.00
	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 100, 105.00)
	eng.AddOrder(sell)

	// Incoming buy at 100.00 — buy price < sell price, no cross
	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Crossed {
		t.Fatal("expected no cross when buy price < sell price")
	}
}

func TestDifferentInstruments_NoCross(t *testing.T) {
	eng := NewCrossingEngine()

	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "GOOG", 100, 100.00)
	eng.AddOrder(sell)

	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Crossed {
		t.Fatal("expected no cross for different instruments")
	}
}

func TestThreadSafety(t *testing.T) {
	eng := NewCrossingEngine()

	var wg sync.WaitGroup
	// Concurrently add orders and try crosses.
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			sell := newOrder(
				"sell-"+string(rune('A'+n%26)),
				domain.SideSell, domain.OrderTypeLimit, "AAPL",
				10, 100.00,
			)
			eng.AddOrder(sell)
		}(i)
		go func(n int) {
			defer wg.Done()
			buy := newOrder(
				"buy-"+string(rune('A'+n%26)),
				domain.SideBuy, domain.OrderTypeLimit, "AAPL",
				10, 100.00,
			)
			_, _ = eng.TryCross(buy)
		}(i)
	}
	wg.Wait()
	// If we got here without a race panic/deadlock, concurrency is safe.
}

func TestFillIDsAreUnique(t *testing.T) {
	eng := NewCrossingEngine()

	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	eng.AddOrder(sell)

	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	result, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Fills[0].ID == result.Fills[1].ID {
		t.Error("fill IDs should be unique")
	}
	if result.Fills[0].ID == "" || result.Fills[1].ID == "" {
		t.Error("fill IDs should not be empty")
	}
}

func TestOrderRemovedFromBookAfterFullCross(t *testing.T) {
	eng := NewCrossingEngine()

	sell := newOrder("sell-1", domain.SideSell, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	eng.AddOrder(sell)

	buy := newOrder("buy-1", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	_, err := eng.TryCross(buy)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second attempt should not cross
	buy2 := newOrder("buy-2", domain.SideBuy, domain.OrderTypeLimit, "AAPL", 100, 100.00)
	result, err := eng.TryCross(buy2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Crossed {
		t.Fatal("expected no cross after resting order was fully filled and removed")
	}
}
