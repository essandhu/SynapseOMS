package router

import (
	"testing"

	"github.com/shopspring/decimal"
)

func d(v string) decimal.Decimal {
	return decimal.RequireFromString(v)
}

func TestSplitOrder_BelowThreshold_NoSplit(t *testing.T) {
	// Order qty = 20, best venue depth = 80 → 20/80 = 25% < 50% → no split
	orderQty := d("20")
	ranked := []VenueAllocation{
		{VenueID: "binance", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "simulated", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"binance":   d("80"),
		"simulated": d("60"),
	}
	lotSize := d("1")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if result[0].VenueID != "binance" {
		t.Errorf("expected venue binance, got %s", result[0].VenueID)
	}
	if !result[0].Quantity.Equal(orderQty) {
		t.Errorf("expected quantity %s, got %s", orderQty, result[0].Quantity)
	}
}

func TestSplitOrder_ExactlyAtThreshold_NoSplit(t *testing.T) {
	// Order qty = 40, best venue depth = 80 → 40/80 = 50% → not > 50%, no split
	orderQty := d("40")
	ranked := []VenueAllocation{
		{VenueID: "binance", Quantity: decimal.Zero, Reason: "best-price"},
	}
	depthMap := map[string]decimal.Decimal{
		"binance": d("80"),
	}
	lotSize := d("1")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if !result[0].Quantity.Equal(orderQty) {
		t.Errorf("expected quantity %s, got %s", orderQty, result[0].Quantity)
	}
}

func TestSplitOrder_AboveThreshold_SplitsProportionally(t *testing.T) {
	// Architecture example: 100 ETH, Binance=40, Simulated=80
	// 100 > 50% of 40 (=20) → split
	// Binance: 40/120 * 100 = 33.33 → floor to 33
	// Simulated: 80/120 * 100 = 66.66 → floor to 66
	// Residual: 100 - 33 - 66 = 1 → best venue (binance)
	// Final: binance=34, simulated=66
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "binance", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "simulated", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"binance":   d("40"),
		"simulated": d("80"),
	}
	lotSize := d("1")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	if len(result) != 2 {
		t.Fatalf("expected 2 allocations, got %d", len(result))
	}

	allocs := make(map[string]decimal.Decimal)
	for _, a := range result {
		allocs[a.VenueID] = a.Quantity
	}

	// Binance: floor(33.33) = 33, + 1 residual = 34
	if !allocs["binance"].Equal(d("34")) {
		t.Errorf("expected binance=34, got %s", allocs["binance"])
	}
	// Simulated: floor(66.66) = 66
	if !allocs["simulated"].Equal(d("66")) {
		t.Errorf("expected simulated=66, got %s", allocs["simulated"])
	}

	// Total must equal order qty
	total := decimal.Zero
	for _, a := range result {
		total = total.Add(a.Quantity)
	}
	if !total.Equal(orderQty) {
		t.Errorf("total allocations %s != order qty %s", total, orderQty)
	}
}

func TestSplitOrder_LotSizeRounding(t *testing.T) {
	// 100 qty, venue A depth=30, venue B depth=70, lotSize=10
	// A: 30/100 * 100 = 30 → floor to lot=30
	// B: 70/100 * 100 = 70 → floor to lot=70
	// Residual: 0 → no residual
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "A", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "B", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"A": d("30"),
		"B": d("70"),
	}
	lotSize := d("10")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	allocs := make(map[string]decimal.Decimal)
	for _, a := range result {
		allocs[a.VenueID] = a.Quantity
	}

	if !allocs["A"].Equal(d("30")) {
		t.Errorf("expected A=30, got %s", allocs["A"])
	}
	if !allocs["B"].Equal(d("70")) {
		t.Errorf("expected B=70, got %s", allocs["B"])
	}
}

func TestSplitOrder_LotSizeRounding_WithResidual(t *testing.T) {
	// 100 qty, venue A depth=40, venue B depth=80, lotSize=10
	// A: 40/120 * 100 = 33.33 → floor to lot 10 = 30
	// B: 80/120 * 100 = 66.66 → floor to lot 10 = 60
	// Residual: 100 - 30 - 60 = 10 → best venue (A)
	// Final: A=40, B=60
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "A", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "B", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"A": d("40"),
		"B": d("80"),
	}
	lotSize := d("10")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	allocs := make(map[string]decimal.Decimal)
	for _, a := range result {
		allocs[a.VenueID] = a.Quantity
	}

	if !allocs["A"].Equal(d("40")) {
		t.Errorf("expected A=40, got %s", allocs["A"])
	}
	if !allocs["B"].Equal(d("60")) {
		t.Errorf("expected B=60, got %s", allocs["B"])
	}

	total := decimal.Zero
	for _, a := range result {
		total = total.Add(a.Quantity)
	}
	if !total.Equal(orderQty) {
		t.Errorf("total %s != order qty %s", total, orderQty)
	}
}

func TestSplitOrder_SubLotAllocationsDropped(t *testing.T) {
	// 100 qty, venue A depth=99, venue B depth=1, lotSize=10
	// A: 99/100 * 100 = 99 → floor to lot 10 = 90
	// B: 1/100 * 100 = 1 → floor to lot 10 = 0 (dropped, < 1 lot)
	// Residual: 100 - 90 = 10 → best venue (A)
	// Final: A=100, only 1 allocation returned
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "A", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "B", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"A": d("99"),
		"B": d("1"),
	}
	lotSize := d("10")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation (sub-lot B dropped), got %d", len(result))
	}
	if result[0].VenueID != "A" {
		t.Errorf("expected venue A, got %s", result[0].VenueID)
	}
	if !result[0].Quantity.Equal(d("100")) {
		t.Errorf("expected A=100, got %s", result[0].Quantity)
	}
}

func TestSplitOrder_SingleVenue(t *testing.T) {
	// Only one venue in ranked list → always single allocation
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "A", Quantity: decimal.Zero, Reason: "best-price"},
	}
	depthMap := map[string]decimal.Decimal{
		"A": d("50"),
	}
	lotSize := d("1")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	if len(result) != 1 {
		t.Fatalf("expected 1 allocation, got %d", len(result))
	}
	if !result[0].Quantity.Equal(d("100")) {
		t.Errorf("expected 100, got %s", result[0].Quantity)
	}
}

func TestSplitOrder_EmptyRankedVenues(t *testing.T) {
	result := SplitOrder(d("100"), nil, nil, d("1"))
	if result != nil {
		t.Errorf("expected nil for empty venues, got %v", result)
	}
}

func TestSplitOrder_ReasonField(t *testing.T) {
	orderQty := d("100")
	ranked := []VenueAllocation{
		{VenueID: "A", Quantity: decimal.Zero, Reason: "best-price"},
		{VenueID: "B", Quantity: decimal.Zero, Reason: ""},
	}
	depthMap := map[string]decimal.Decimal{
		"A": d("40"),
		"B": d("80"),
	}
	lotSize := d("1")

	result := SplitOrder(orderQty, ranked, depthMap, lotSize)

	for _, a := range result {
		if a.Reason == "" {
			t.Errorf("expected non-empty reason for venue %s", a.VenueID)
		}
	}
}
