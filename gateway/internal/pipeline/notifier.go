// Package pipeline implements the order processing pipeline for the gateway.
package pipeline

import (
	"log/slog"

	"github.com/synapse-oms/gateway/internal/domain"
)

// Notifier is the interface for broadcasting order and position updates
// to downstream consumers (e.g., WebSocket clients, event bus).
type Notifier interface {
	NotifyOrderUpdate(order *domain.Order)
	NotifyPositionUpdate(position *domain.Position)
}

// LogNotifier is a Notifier that logs updates via structured logging.
// It serves as the default notifier when no WebSocket or event bus is configured.
type LogNotifier struct {
	logger *slog.Logger
}

// NewLogNotifier creates a LogNotifier with the given logger.
func NewLogNotifier(logger *slog.Logger) *LogNotifier {
	return &LogNotifier{logger: logger}
}

// NotifyOrderUpdate logs the order update.
func (n *LogNotifier) NotifyOrderUpdate(order *domain.Order) {
	n.logger.Info("order update",
		slog.String("order_id", string(order.ID)),
		slog.Int("status", int(order.Status)),
		slog.String("instrument", order.InstrumentID),
		slog.String("filled_qty", order.FilledQuantity.String()),
	)
}

// NotifyPositionUpdate logs the position update.
func (n *LogNotifier) NotifyPositionUpdate(position *domain.Position) {
	n.logger.Info("position update",
		slog.String("instrument", position.InstrumentID),
		slog.String("venue", position.VenueID),
		slog.String("quantity", position.Quantity.String()),
		slog.String("avg_cost", position.AverageCost.String()),
	)
}
