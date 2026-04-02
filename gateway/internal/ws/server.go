package ws

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/synapse-oms/gateway/internal/logging"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer (clients don't send large messages).
	maxMessageSize = 512

	// Send channel buffer size per client.
	sendBufSize = 256
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		return origin == "" || origin == "http://localhost:3000"
	},
}

// Server provides HTTP handlers for WebSocket upgrade endpoints.
type Server struct {
	Hub    *Hub
	logger *slog.Logger
}

// NewServer creates a new WebSocket server backed by the given hub.
func NewServer(hub *Hub) *Server {
	return &Server{
		Hub:    hub,
		logger: logging.NewDefault("gateway", "ws-server"),
	}
}

// HandleOrders upgrades the connection for /ws/orders streaming.
func (s *Server) HandleOrders(w http.ResponseWriter, r *http.Request) {
	s.serveWS(w, r, StreamOrders)
}

// HandlePositions upgrades the connection for /ws/positions streaming.
func (s *Server) HandlePositions(w http.ResponseWriter, r *http.Request) {
	s.serveWS(w, r, StreamPositions)
}

// HandleVenues upgrades the connection for /ws/venues streaming.
func (s *Server) HandleVenues(w http.ResponseWriter, r *http.Request) {
	s.serveWS(w, r, StreamVenues)
}

// HandleAnomalies upgrades the connection for /ws/anomalies streaming.
func (s *Server) HandleAnomalies(w http.ResponseWriter, r *http.Request) {
	s.serveWS(w, r, StreamAnomalies)
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request, stream StreamType) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed",
			slog.String("error", err.Error()),
			slog.String("stream", string(stream)),
		)
		return
	}

	c := &client{
		conn:   conn,
		send:   make(chan []byte, sendBufSize),
		hub:    s.Hub,
		stream: stream,
	}

	s.Hub.register(c)

	// Start read and write pumps in separate goroutines.
	go c.writePump()
	go c.readPump()
}

// readPump reads messages from the WebSocket connection.
// It handles pong messages and detects disconnection.
func (c *client) readPump() {
	defer func() {
		c.hub.unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.hub.logger.Warn("unexpected close",
					slog.String("remote", c.conn.RemoteAddr().String()),
					slog.String("error", err.Error()),
				)
			}
			break
		}
		// Clients don't send meaningful messages; discard.
	}
}

// writePump writes messages from the send channel to the WebSocket connection
// and sends periodic pings.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Drain queued messages into the same write batch.
			n := len(c.send)
			for range n {
				_, _ = w.Write([]byte("\n"))
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
