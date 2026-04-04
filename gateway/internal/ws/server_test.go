package ws_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/ws"
)

func dialWS(t *testing.T, server *httptest.Server, path string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + path
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", path, err)
	}
	return conn
}

func TestWebSocketConnect(t *testing.T) {
	hub := ws.NewHub()
	srv := ws.NewServer(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/orders", srv.HandleOrders)
	mux.HandleFunc("/ws/positions", srv.HandlePositions)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect to orders stream
	conn := dialWS(t, server, "/ws/orders")
	defer conn.Close()

	// Give the hub a moment to register the client.
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}
}

func TestWebSocketOrderBroadcast(t *testing.T) {
	hub := ws.NewHub()
	srv := ws.NewServer(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/orders", srv.HandleOrders)
	server := httptest.NewServer(mux)
	defer server.Close()

	conn := dialWS(t, server, "/ws/orders")
	defer conn.Close()

	// Wait for registration
	time.Sleep(50 * time.Millisecond)

	// Broadcast an order update
	order := &domain.Order{
		ID:             "test-order-1",
		InstrumentID:   "AAPL",
		Side:           domain.SideBuy,
		Type:           domain.OrderTypeMarket,
		Quantity:       decimal.NewFromInt(10),
		Price:          decimal.Zero,
		FilledQuantity: decimal.NewFromInt(10),
		AveragePrice:   decimal.NewFromFloat(150.50),
		Status:         domain.OrderStatusFilled,
		AssetClass:     domain.AssetClassEquity,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	hub.NotifyOrderUpdate(order)

	// Read the message
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	var msg ws.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	if msg.Type != "order_update" {
		t.Errorf("expected type order_update, got %s", msg.Type)
	}

	// Check that data contains expected fields
	dataMap, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", msg.Data)
	}
	if dataMap["id"] != "test-order-1" {
		t.Errorf("expected id test-order-1, got %v", dataMap["id"])
	}
	if dataMap["instrument_id"] != "AAPL" {
		t.Errorf("expected instrument_id AAPL, got %v", dataMap["instrument_id"])
	}
	// Decimal values must be strings
	if dataMap["quantity"] != "10" {
		t.Errorf("expected quantity '10', got %v (type %T)", dataMap["quantity"], dataMap["quantity"])
	}
	if dataMap["status"] != "filled" {
		t.Errorf("expected status filled, got %v", dataMap["status"])
	}
	if dataMap["asset_class"] != "equity" {
		t.Errorf("expected asset_class equity, got %v", dataMap["asset_class"])
	}
}

func TestWebSocketPositionBroadcast(t *testing.T) {
	hub := ws.NewHub()
	srv := ws.NewServer(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/positions", srv.HandlePositions)
	server := httptest.NewServer(mux)
	defer server.Close()

	conn := dialWS(t, server, "/ws/positions")
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	position := &domain.Position{
		InstrumentID:  "AAPL",
		VenueID:       "sim-exchange",
		Quantity:      decimal.NewFromInt(100),
		AverageCost:   decimal.NewFromFloat(150.25),
		MarketPrice:   decimal.NewFromFloat(155.00),
		UnrealizedPnL: decimal.NewFromFloat(475.00),
		RealizedPnL:   decimal.Zero,
		UpdatedAt:     time.Now(),
	}

	hub.NotifyPositionUpdate(position)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}

	var msg ws.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal message: %v", err)
	}

	if msg.Type != "position_update" {
		t.Errorf("expected type position_update, got %s", msg.Type)
	}

	dataMap, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected data to be a map, got %T", msg.Data)
	}
	if dataMap["instrument_id"] != "AAPL" {
		t.Errorf("expected instrument_id AAPL, got %v", dataMap["instrument_id"])
	}
	if dataMap["quantity"] != "100" {
		t.Errorf("expected quantity '100', got %v", dataMap["quantity"])
	}
}

func TestWebSocketDisconnectCleanup(t *testing.T) {
	hub := ws.NewHub()
	srv := ws.NewServer(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/orders", srv.HandleOrders)
	server := httptest.NewServer(mux)
	defer server.Close()

	conn := dialWS(t, server, "/ws/orders")

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	// Close the connection
	conn.Close()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}

func TestWebSocketStreamIsolation(t *testing.T) {
	hub := ws.NewHub()
	srv := ws.NewServer(hub)

	mux := http.NewServeMux()
	mux.HandleFunc("/ws/orders", srv.HandleOrders)
	mux.HandleFunc("/ws/positions", srv.HandlePositions)
	server := httptest.NewServer(mux)
	defer server.Close()

	// Connect one client to orders, another to positions
	orderConn := dialWS(t, server, "/ws/orders")
	defer orderConn.Close()
	posConn := dialWS(t, server, "/ws/positions")
	defer posConn.Close()

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}

	// Send an order update — only the orders client should receive it
	order := &domain.Order{
		ID:           "test-order-2",
		InstrumentID: "MSFT",
		Side:         domain.SideSell,
		Type:         domain.OrderTypeLimit,
		Quantity:     decimal.NewFromInt(5),
		Price:        decimal.NewFromInt(350),
		Status:       domain.OrderStatusAcknowledged,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	hub.NotifyOrderUpdate(order)

	// Orders client should receive the message
	_ = orderConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := orderConn.ReadMessage()
	if err != nil {
		t.Fatalf("orders client read: %v", err)
	}
	var msg ws.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "order_update" {
		t.Errorf("expected order_update, got %s", msg.Type)
	}

	// Positions client should NOT receive the order update — set a short deadline
	_ = posConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = posConn.ReadMessage()
	if err == nil {
		t.Error("positions client should not have received an order update")
	}
}
