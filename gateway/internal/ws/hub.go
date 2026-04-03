// Package ws provides WebSocket server functionality for real-time order
// and position update streaming.
package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/synapse-oms/gateway/internal/domain"
	"github.com/synapse-oms/gateway/internal/logging"
	"github.com/synapse-oms/gateway/internal/metrics"
)

// StreamType identifies the type of WebSocket stream.
type StreamType string

const (
	StreamOrders    StreamType = "orders"
	StreamPositions StreamType = "positions"
	StreamVenues    StreamType = "venues"
	StreamAnomalies StreamType = "anomalies"
)

// Message is the JSON envelope sent to WebSocket clients.
type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// orderData is the JSON representation of an order update.
type orderData struct {
	ID             string    `json:"id"`
	ClientOrderID  string    `json:"client_order_id,omitempty"`
	InstrumentID   string    `json:"instrument_id"`
	Side           string    `json:"side"`
	Type           string    `json:"type"`
	Quantity       string    `json:"quantity"`
	Price          string    `json:"price"`
	FilledQuantity string    `json:"filled_quantity"`
	AveragePrice   string    `json:"average_price"`
	Status         string    `json:"status"`
	VenueID        string    `json:"venue_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// positionData is the JSON representation of a position update.
type positionData struct {
	InstrumentID      string    `json:"instrument_id"`
	VenueID           string    `json:"venue_id"`
	Quantity          string    `json:"quantity"`
	AverageCost       string    `json:"average_cost"`
	MarketPrice       string    `json:"market_price"`
	UnrealizedPnL     string    `json:"unrealized_pnl"`
	RealizedPnL       string    `json:"realized_pnl"`
	UnsettledQuantity string    `json:"unsettled_quantity"`
	SettledQuantity   string    `json:"settled_quantity"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// VenueStatusEvent represents a venue status change for broadcasting.
type VenueStatusEvent struct {
	VenueID   string
	Name      string
	Status    string
	LatencyMs int64
	Timestamp time.Time
}

// venueData is the JSON representation of a venue status update.
type venueData struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	LatencyMs int64     `json:"latencyMs"`
	Timestamp time.Time `json:"timestamp"`
}

// AnomalyAlertEvent represents an anomaly alert for broadcasting.
type AnomalyAlertEvent struct {
	ID           string
	InstrumentID string
	VenueID      string
	AnomalyScore float64
	Severity     string
	Features     map[string]float64
	Description  string
	Timestamp    time.Time
	Acknowledged bool
}

// anomalyData is the JSON representation of an anomaly alert.
type anomalyData struct {
	ID           string             `json:"id"`
	InstrumentID string             `json:"instrument_id"`
	VenueID      string             `json:"venue_id"`
	AnomalyScore float64            `json:"anomaly_score"`
	Severity     string             `json:"severity"`
	Features     map[string]float64 `json:"features"`
	Description  string             `json:"description"`
	Timestamp    time.Time          `json:"timestamp"`
	Acknowledged bool               `json:"acknowledged"`
}

// client represents a connected WebSocket client.
type client struct {
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
	stream StreamType
}

// Hub manages WebSocket clients and broadcasts updates.
// It implements the pipeline.Notifier interface.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	logger  *slog.Logger

	// posThrottle tracks last position broadcast time for throttling.
	posThrottleMu    sync.Mutex
	posLastBroadcast time.Time
	posPending       *positionData
	posTimer         *time.Timer

	// VenueStatusChan receives venue status events from adapters.
	// The hub's Run loop monitors this channel and broadcasts to /ws/venues clients.
	VenueStatusChan chan VenueStatusEvent
}

const positionThrottleInterval = 100 * time.Millisecond

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:         make(map[*client]struct{}),
		logger:          logging.NewDefault("gateway", "ws-hub"),
		VenueStatusChan: make(chan VenueStatusEvent, 64),
	}
}

// Run starts the hub's event loop, monitoring the VenueStatusChan for
// venue status events and broadcasting them to subscribed clients.
// It blocks until the provided context is canceled.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-h.VenueStatusChan:
			h.NotifyVenueStatus(event)
		}
	}
}

// register adds a client to the hub.
func (h *Hub) register(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	metrics.ActiveWebSocketConnections.Inc()
	h.logger.Info("client connected",
		slog.String("stream", string(c.stream)),
		slog.String("remote", c.conn.RemoteAddr().String()),
	)
}

// unregister removes a client from the hub and closes its send channel.
func (h *Hub) unregister(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
		metrics.ActiveWebSocketConnections.Dec()
		h.logger.Info("client disconnected",
			slog.String("stream", string(c.stream)),
			slog.String("remote", c.conn.RemoteAddr().String()),
		)
	}
}

// ClientCount returns the number of connected clients (useful for testing).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// broadcast sends data to all clients subscribed to the given stream.
func (h *Hub) broadcast(stream StreamType, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		if c.stream != stream {
			continue
		}
		select {
		case c.send <- data:
		default:
			// Client is too slow; drop the message (will be cleaned up by pong timeout).
			h.logger.Warn("dropping message for slow client",
				slog.String("remote", c.conn.RemoteAddr().String()),
			)
		}
	}
}

// NotifyOrderUpdate broadcasts an order update to all /ws/orders clients.
// This satisfies the pipeline.Notifier interface.
func (h *Hub) NotifyOrderUpdate(order *domain.Order) {
	msg := Message{
		Type: "order_update",
		Data: orderData{
			ID:             string(order.ID),
			ClientOrderID:  order.ClientOrderID,
			InstrumentID:   order.InstrumentID,
			Side:           sideStr(order.Side),
			Type:           typeStr(order.Type),
			Quantity:       order.Quantity.String(),
			Price:          order.Price.String(),
			FilledQuantity: order.FilledQuantity.String(),
			AveragePrice:   order.AveragePrice.String(),
			Status:         statusStr(order.Status),
			VenueID:        order.VenueID,
			CreatedAt:      order.CreatedAt,
			UpdatedAt:      order.UpdatedAt,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal order update", slog.String("error", err.Error()))
		return
	}

	h.broadcast(StreamOrders, data)
}

// NotifyPositionUpdate broadcasts a position update to all /ws/positions clients
// with 100ms throttling.
// This satisfies the pipeline.Notifier interface.
func (h *Hub) NotifyPositionUpdate(position *domain.Position) {
	pd := positionData{
		InstrumentID:      position.InstrumentID,
		VenueID:           position.VenueID,
		Quantity:          position.Quantity.String(),
		AverageCost:       position.AverageCost.String(),
		MarketPrice:       position.MarketPrice.String(),
		UnrealizedPnL:     position.UnrealizedPnL.String(),
		RealizedPnL:       position.RealizedPnL.String(),
		UnsettledQuantity: position.UnsettledQuantity.String(),
		SettledQuantity:   position.SettledQuantity.String(),
		UpdatedAt:         position.UpdatedAt,
	}

	h.posThrottleMu.Lock()
	defer h.posThrottleMu.Unlock()

	now := time.Now()
	if now.Sub(h.posLastBroadcast) >= positionThrottleInterval {
		// Enough time has passed — send immediately.
		h.sendPositionUpdate(pd)
		h.posLastBroadcast = now
		return
	}

	// Store as pending and schedule a delayed send.
	h.posPending = &pd
	if h.posTimer == nil {
		remaining := positionThrottleInterval - now.Sub(h.posLastBroadcast)
		h.posTimer = time.AfterFunc(remaining, func() {
			h.posThrottleMu.Lock()
			defer h.posThrottleMu.Unlock()
			if h.posPending != nil {
				h.sendPositionUpdate(*h.posPending)
				h.posPending = nil
				h.posLastBroadcast = time.Now()
			}
			h.posTimer = nil
		})
	}
}

func (h *Hub) sendPositionUpdate(pd positionData) {
	msg := Message{
		Type: "position_update",
		Data: pd,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal position update", slog.String("error", err.Error()))
		return
	}
	h.broadcast(StreamPositions, data)
}

// venueMessage is the JSON envelope for venue status updates.
// It uses "venue" as the payload key per the API spec (differs from the generic "data" key).
type venueMessage struct {
	Type  string    `json:"type"`
	Venue venueData `json:"venue"`
}

// NotifyVenueStatus broadcasts a venue status event to all /ws/venues clients.
func (h *Hub) NotifyVenueStatus(event VenueStatusEvent) {
	msgType := "venue_" + event.Status // e.g. "venue_connected", "venue_disconnected", "venue_degraded"

	msg := venueMessage{
		Type: msgType,
		Venue: venueData{
			ID:        event.VenueID,
			Name:      event.Name,
			Status:    event.Status,
			LatencyMs: event.LatencyMs,
			Timestamp: event.Timestamp,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal venue status", slog.String("error", err.Error()))
		return
	}

	h.broadcast(StreamVenues, data)
}

// NotifyAnomalyAlert broadcasts an anomaly alert to all /ws/anomalies clients.
func (h *Hub) NotifyAnomalyAlert(event AnomalyAlertEvent) {
	msg := Message{
		Type: "anomaly_alert",
		Data: anomalyData{
			ID:           event.ID,
			InstrumentID: event.InstrumentID,
			VenueID:      event.VenueID,
			AnomalyScore: event.AnomalyScore,
			Severity:     event.Severity,
			Features:     event.Features,
			Description:  event.Description,
			Timestamp:    event.Timestamp,
			Acknowledged: event.Acknowledged,
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("failed to marshal anomaly alert", slog.String("error", err.Error()))
		return
	}

	h.broadcast(StreamAnomalies, data)
}

// --- string helpers ---

func sideStr(s domain.OrderSide) string {
	if s == domain.SideSell {
		return "sell"
	}
	return "buy"
}

func typeStr(t domain.OrderType) string {
	switch t {
	case domain.OrderTypeLimit:
		return "limit"
	case domain.OrderTypeStopLimit:
		return "stop_limit"
	default:
		return "market"
	}
}

func statusStr(s domain.OrderStatus) string {
	switch s {
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
