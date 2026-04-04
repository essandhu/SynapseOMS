// anomaly_consumer.go provides a Kafka consumer for the anomaly-alerts topic.
// It deserializes AnomalyAlert messages and forwards them to a callback
// (typically the WebSocket hub for real-time relay to connected clients).
package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/synapse-oms/gateway/internal/logging"
)

// AnomalyAlert represents a deserialized anomaly alert from Kafka.
type AnomalyAlert struct {
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

// AnomalyCallback is called when an anomaly alert is received.
type AnomalyCallback func(alert AnomalyAlert)

// AnomalyConsumer subscribes to the anomaly-alerts Kafka topic
// and forwards alerts to a callback (for WebSocket relay).
type AnomalyConsumer struct {
	brokers  string
	callback AnomalyCallback
	logger   *slog.Logger
	cancel   context.CancelFunc
}

// NewAnomalyConsumer creates a new consumer for the anomaly-alerts topic.
func NewAnomalyConsumer(brokers string, callback AnomalyCallback) *AnomalyConsumer {
	return &AnomalyConsumer{
		brokers:  brokers,
		callback: callback,
		logger:   logging.NewDefault("gateway", "anomaly-consumer"),
	}
}

// Start begins consuming in a goroutine. Call Stop to shut down.
func (ac *AnomalyConsumer) Start(ctx context.Context) {
	ctx, ac.cancel = context.WithCancel(ctx)
	go ac.consumeLoop(ctx)
	ac.logger.Info("anomaly consumer started")
}

// Stop signals the consumer to shut down.
func (ac *AnomalyConsumer) Stop() {
	if ac.cancel != nil {
		ac.cancel()
	}
	ac.logger.Info("anomaly consumer stopped")
}

func (ac *AnomalyConsumer) consumeLoop(ctx context.Context) {
	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  []string{ac.brokers},
		Topic:    TopicAnomalyAlerts,
		GroupID:  "gateway-anomaly-relay",
		MaxWait:  time.Second,
		MinBytes: 1,
		MaxBytes: 10e6, // 10MB
	})
	defer reader.Close()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // context cancelled — clean shutdown
			}
			// transient error — keep polling
			continue
		}
		ac.processMessage(msg.Value)
	}
}

// processMessage deserializes a raw Kafka message value into an AnomalyAlert
// and invokes the configured callback. Invalid JSON is logged and skipped.
func (ac *AnomalyConsumer) processMessage(data []byte) {
	var alert AnomalyAlert
	if err := json.Unmarshal(data, &alert); err != nil {
		ac.logger.Error("failed to unmarshal anomaly alert",
			slog.String("error", err.Error()),
		)
		return
	}
	ac.callback(alert)
}
