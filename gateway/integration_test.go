//go:build integration

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// gatewayURL returns the base URL for the running gateway.
func gatewayURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("GATEWAY_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://localhost:8080"
}

// wsURL converts an HTTP base URL to a WebSocket URL for the given path.
func wsURL(base, path string) string {
	u := strings.Replace(base, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1)
	return u + path
}

// instrumentJSON is used to decode GET /api/v1/instruments responses.
type instrumentJSON struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

// orderJSON is used to decode order responses.
type orderJSON struct {
	ID             string `json:"id"`
	InstrumentID   string `json:"instrument_id"`
	Side           string `json:"side"`
	Type           string `json:"type"`
	Quantity       string `json:"quantity"`
	Status         string `json:"status"`
	FilledQuantity string `json:"filled_quantity"`
	Fills          []struct {
		ID       string `json:"id"`
		Quantity string `json:"quantity"`
		Price    string `json:"price"`
	} `json:"fills"`
}

// wsMessage is the envelope received from WebSocket streams.
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// wsOrderData represents an order update received via WebSocket.
type wsOrderData struct {
	ID             string `json:"id"`
	InstrumentID   string `json:"instrument_id"`
	Status         string `json:"status"`
	FilledQuantity string `json:"filled_quantity"`
}

// wsPositionData represents a position update received via WebSocket.
type wsPositionData struct {
	InstrumentID string `json:"instrument_id"`
	Quantity     string `json:"quantity"`
}

// positionJSON is used to decode GET /api/v1/positions/{instrumentID} responses.
type positionJSON struct {
	InstrumentID string `json:"instrument_id"`
	Quantity     string `json:"quantity"`
}

func TestPhase1AcceptanceFlow(t *testing.T) {
	base := gatewayURL(t)
	t.Logf("Gateway URL: %s", base)

	// -------------------------------------------------------
	// Step 1: GET /api/v1/instruments — verify 6 instruments
	// -------------------------------------------------------
	t.Log("Step 1: GET /api/v1/instruments — verify 6 instruments")
	resp, err := http.Get(base + "/api/v1/instruments")
	if err != nil {
		t.Fatalf("Step 1: failed to GET instruments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Step 1: expected 200, got %d", resp.StatusCode)
	}

	var instruments []instrumentJSON
	if err := json.NewDecoder(resp.Body).Decode(&instruments); err != nil {
		t.Fatalf("Step 1: failed to decode instruments: %v", err)
	}

	if len(instruments) != 6 {
		t.Fatalf("Step 1: expected 6 instruments, got %d", len(instruments))
	}
	t.Logf("Step 1: PASS — %d instruments found", len(instruments))

	// -------------------------------------------------------
	// Step 2: Connect WebSocket to /ws/orders
	// -------------------------------------------------------
	t.Log("Step 2: Connect WebSocket to /ws/orders")
	orderWSURL := wsURL(base, "/ws/orders")
	orderConn, _, err := websocket.DefaultDialer.Dial(orderWSURL, nil)
	if err != nil {
		t.Fatalf("Step 2: failed to connect to /ws/orders: %v", err)
	}
	defer orderConn.Close()
	t.Log("Step 2: PASS — WebSocket /ws/orders connected")

	// Start collecting order WS messages in background
	orderMsgs := make(chan wsOrderData, 50)
	go func() {
		for {
			_, raw, err := orderConn.ReadMessage()
			if err != nil {
				close(orderMsgs)
				return
			}
			// The server may batch messages with newlines
			for _, line := range strings.Split(string(raw), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var msg wsMessage
				if err := json.Unmarshal([]byte(line), &msg); err != nil {
					continue
				}
				if msg.Type == "order_update" {
					var od wsOrderData
					if err := json.Unmarshal(msg.Data, &od); err == nil {
						orderMsgs <- od
					}
				}
			}
		}
	}()

	// -------------------------------------------------------
	// Step 3: POST /api/v1/orders — market buy 10 AAPL
	// -------------------------------------------------------
	t.Log("Step 3: POST /api/v1/orders — market buy 10 AAPL")
	orderBody := `{
		"instrument_id": "AAPL",
		"side": "buy",
		"type": "market",
		"quantity": "10",
		"price": "0"
	}`
	resp, err = http.Post(base+"/api/v1/orders", "application/json", strings.NewReader(orderBody))
	if err != nil {
		t.Fatalf("Step 3: failed to POST order: %v", err)
	}
	defer resp.Body.Close()

	// -------------------------------------------------------
	// Step 4: Verify 201, order status "new"
	// -------------------------------------------------------
	t.Log("Step 4: Verify 201 Created, status 'new'")
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Step 4: expected 201, got %d", resp.StatusCode)
	}

	var createdOrder orderJSON
	if err := json.NewDecoder(resp.Body).Decode(&createdOrder); err != nil {
		t.Fatalf("Step 4: failed to decode order response: %v", err)
	}

	if createdOrder.Status != "new" {
		t.Fatalf("Step 4: expected status 'new', got '%s'", createdOrder.Status)
	}
	if createdOrder.ID == "" {
		t.Fatal("Step 4: order ID is empty")
	}
	orderID := createdOrder.ID
	t.Logf("Step 4: PASS — order %s created with status 'new'", orderID)

	// -------------------------------------------------------
	// Step 5: Wait for WS: New → Acknowledged → Filled (within 10s)
	// -------------------------------------------------------
	t.Log("Step 5: Wait for WebSocket transitions: new → acknowledged → filled")
	seenStatuses := map[string]bool{}
	requiredStatuses := []string{"new", "acknowledged", "filled"}
	timeout := time.After(10 * time.Second)

	func() {
		for {
			select {
			case od, ok := <-orderMsgs:
				if !ok {
					t.Fatal("Step 5: WebSocket closed before receiving all statuses")
					return
				}
				if od.ID != orderID {
					continue
				}
				seenStatuses[od.Status] = true
				t.Logf("Step 5:   WS order_update: id=%s status=%s", od.ID, od.Status)

				// Check if we've seen all required statuses
				allSeen := true
				for _, s := range requiredStatuses {
					if !seenStatuses[s] {
						allSeen = false
						break
					}
				}
				if allSeen {
					return
				}

			case <-timeout:
				missing := []string{}
				for _, s := range requiredStatuses {
					if !seenStatuses[s] {
						missing = append(missing, s)
					}
				}
				t.Fatalf("Step 5: timed out waiting for statuses; missing: %v; seen: %v",
					missing, seenStatuses)
				return
			}
		}
	}()
	t.Log("Step 5: PASS — all status transitions observed via WebSocket")

	// -------------------------------------------------------
	// Step 6: GET /api/v1/orders/{id} — status "filled", filledQuantity "10", fills non-empty
	// -------------------------------------------------------
	t.Logf("Step 6: GET /api/v1/orders/%s — verify filled", orderID)
	resp, err = http.Get(fmt.Sprintf("%s/api/v1/orders/%s", base, orderID))
	if err != nil {
		t.Fatalf("Step 6: failed to GET order: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Step 6: expected 200, got %d", resp.StatusCode)
	}

	var filledOrder orderJSON
	if err := json.NewDecoder(resp.Body).Decode(&filledOrder); err != nil {
		t.Fatalf("Step 6: failed to decode order: %v", err)
	}

	if filledOrder.Status != "filled" {
		t.Fatalf("Step 6: expected status 'filled', got '%s'", filledOrder.Status)
	}
	if filledOrder.FilledQuantity != "10" {
		t.Fatalf("Step 6: expected filledQuantity '10', got '%s'", filledOrder.FilledQuantity)
	}
	if len(filledOrder.Fills) == 0 {
		t.Fatal("Step 6: expected non-empty fills array")
	}
	t.Logf("Step 6: PASS — order status='filled', filledQuantity='%s', fills=%d",
		filledOrder.FilledQuantity, len(filledOrder.Fills))

	// -------------------------------------------------------
	// Step 7: GET /api/v1/positions/AAPL — quantity "10"
	// -------------------------------------------------------
	t.Log("Step 7: GET /api/v1/positions/AAPL — verify quantity 10")
	resp, err = http.Get(base + "/api/v1/positions/AAPL")
	if err != nil {
		t.Fatalf("Step 7: failed to GET position: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Step 7: expected 200, got %d", resp.StatusCode)
	}

	var position positionJSON
	if err := json.NewDecoder(resp.Body).Decode(&position); err != nil {
		t.Fatalf("Step 7: failed to decode position: %v", err)
	}

	if position.Quantity != "10" {
		t.Fatalf("Step 7: expected quantity '10', got '%s'", position.Quantity)
	}
	t.Logf("Step 7: PASS — AAPL position quantity='%s'", position.Quantity)

	// -------------------------------------------------------
	// Step 8: Connect /ws/positions — verify position update received
	// -------------------------------------------------------
	t.Log("Step 8: Connect /ws/positions — verify position update received")
	posWSURL := wsURL(base, "/ws/positions")
	posConn, _, err := websocket.DefaultDialer.Dial(posWSURL, nil)
	if err != nil {
		t.Fatalf("Step 8: failed to connect to /ws/positions: %v", err)
	}
	defer posConn.Close()

	// Submit another small order to trigger a position update on the WS
	t.Log("Step 8:   submitting a second order to trigger position WS update")
	order2Body := `{
		"instrument_id": "AAPL",
		"side": "buy",
		"type": "market",
		"quantity": "1",
		"price": "0"
	}`
	resp, err = http.Post(base+"/api/v1/orders", "application/json", strings.NewReader(order2Body))
	if err != nil {
		t.Fatalf("Step 8: failed to POST second order: %v", err)
	}
	resp.Body.Close()

	// Wait for position update via WebSocket
	posReceived := make(chan wsPositionData, 10)
	go func() {
		for {
			_, raw, err := posConn.ReadMessage()
			if err != nil {
				close(posReceived)
				return
			}
			for _, line := range strings.Split(string(raw), "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				var msg wsMessage
				if err := json.Unmarshal([]byte(line), &msg); err != nil {
					continue
				}
				if msg.Type == "position_update" {
					var pd wsPositionData
					if err := json.Unmarshal(msg.Data, &pd); err == nil {
						posReceived <- pd
					}
				}
			}
		}
	}()

	posTimeout := time.After(10 * time.Second)
	select {
	case pd, ok := <-posReceived:
		if !ok {
			t.Fatal("Step 8: WebSocket closed before receiving position update")
		}
		t.Logf("Step 8: PASS — position update received: instrument=%s quantity=%s",
			pd.InstrumentID, pd.Quantity)
	case <-posTimeout:
		t.Fatal("Step 8: timed out waiting for position update via WebSocket")
	}

	t.Log("=== Phase 1 Acceptance Test: ALL STEPS PASSED ===")
}
