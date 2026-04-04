// Package kafka provides a thin wrapper around segmentio/kafka-go for publishing
// order-lifecycle, market-data, and venue-status events.
package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/synapse-oms/gateway/internal/logging"
)

// Topic constants for the event streams.
const (
	TopicOrderLifecycle = "order-lifecycle"
	TopicMarketData     = "market-data"
	TopicVenueStatus    = "venue-status"
	TopicAnomalyAlerts  = "anomaly-alerts"
)

// Header key used to propagate correlation IDs across Kafka messages.
const headerCorrelationID = "correlation-id"

// Producer wraps a segmentio/kafka-go writer with typed publish helpers
// for each SynapseOMS event stream.
type Producer struct {
	writers map[string]*kafkago.Writer
	logger  *slog.Logger
}

// NewProducer creates a Kafka producer connected to the given broker list
// (comma-separated). It creates one Writer per topic for optimal batching.
func NewProducer(brokers string, logger *slog.Logger) (*Producer, error) {
	if brokers == "" {
		return nil, fmt.Errorf("kafka brokers must not be empty")
	}

	topics := []string{TopicOrderLifecycle, TopicMarketData, TopicVenueStatus}
	writers := make(map[string]*kafkago.Writer, len(topics))

	// Ensure topics exist before writing. This is a best-effort operation;
	// if the broker is unreachable or doesn't allow auto-creation, writers
	// will still attempt to create topics via AllowAutoTopicCreation.
	ensureTopics(brokers, topics, logger)

	for _, topic := range topics {
		writers[topic] = &kafkago.Writer{
			Addr:                   kafkago.TCP(brokers),
			Topic:                  topic,
			Balancer:               &kafkago.Hash{},
			BatchTimeout:           5 * time.Millisecond,
			RequiredAcks:           kafkago.RequireAll,
			MaxAttempts:            3,
			AllowAutoTopicCreation: true,
			Logger:                 kafkago.LoggerFunc(func(msg string, args ...interface{}) { logger.Debug(msg) }),
			ErrorLogger:            kafkago.LoggerFunc(func(msg string, args ...interface{}) { logger.Error(msg) }),
		}
	}

	return &Producer{
		writers: writers,
		logger:  logger,
	}, nil
}

// PublishOrderLifecycle publishes a serialized OrderLifecycleEvent to the
// order-lifecycle topic. The message is partitioned by instrumentID.
// If a correlation ID is present in ctx it is forwarded as a Kafka header.
func (p *Producer) PublishOrderLifecycle(ctx context.Context, instrumentID string, payload []byte) error {
	return p.publish(ctx, TopicOrderLifecycle, instrumentID, payload)
}

// PublishMarketData publishes a serialized market-data message to the
// market-data topic. The message is partitioned by instrumentID.
func (p *Producer) PublishMarketData(ctx context.Context, instrumentID string, payload []byte) error {
	return p.publish(ctx, TopicMarketData, instrumentID, payload)
}

// PublishVenueStatus publishes a serialized venue-status message to the
// venue-status topic. The message is partitioned by venueID.
func (p *Producer) PublishVenueStatus(ctx context.Context, venueID string, payload []byte) error {
	return p.publish(ctx, TopicVenueStatus, venueID, payload)
}

// Close flushes any pending messages and shuts down all writers.
func (p *Producer) Close() {
	for topic, w := range p.writers {
		if err := w.Close(); err != nil {
			p.logger.Error("failed to close kafka writer",
				slog.String("topic", topic),
				slog.String("error", err.Error()),
			)
		}
	}
}

// writeTimeout limits how long a publish can block when Kafka is unreachable,
// preventing the fill pipeline from stalling.
const writeTimeout = 5 * time.Second

// publish is the shared implementation for all topic-specific publish methods.
// It uses a timeout context so that Kafka unavailability doesn't block the
// order processing pipeline.
func (p *Producer) publish(ctx context.Context, topic, key string, payload []byte) error {
	w, ok := p.writers[topic]
	if !ok {
		return fmt.Errorf("no writer for topic %s", topic)
	}

	headers := buildHeaders(ctx)

	msg := kafkago.Message{
		Key:     []byte(key),
		Value:   payload,
		Headers: headers,
	}

	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if err := w.WriteMessages(writeCtx, msg); err != nil {
		return fmt.Errorf("produce to %s: %w", topic, err)
	}

	p.logger.Info("kafka message delivered",
		slog.String("topic", topic),
		slog.String("key", key),
		slog.Int("bytes", len(payload)),
	)

	return nil
}

// ensureTopics creates Kafka topics if they don't already exist.
// This is best-effort — failures are logged but don't prevent startup.
func ensureTopics(brokers string, topics []string, logger *slog.Logger) {
	conn, err := kafkago.Dial("tcp", brokers)
	if err != nil {
		logger.Warn("failed to connect to Kafka for topic creation",
			slog.String("error", err.Error()),
		)
		return
	}
	defer conn.Close()

	controller, err := conn.Controller()
	if err != nil {
		logger.Warn("failed to get Kafka controller",
			slog.String("error", err.Error()),
		)
		return
	}
	controllerConn, err := kafkago.Dial("tcp", fmt.Sprintf("%s:%d", controller.Host, controller.Port))
	if err != nil {
		logger.Warn("failed to connect to Kafka controller",
			slog.String("error", err.Error()),
		)
		return
	}
	defer controllerConn.Close()

	topicConfigs := make([]kafkago.TopicConfig, len(topics))
	for i, t := range topics {
		topicConfigs[i] = kafkago.TopicConfig{
			Topic:             t,
			NumPartitions:     3,
			ReplicationFactor: 1,
		}
	}

	if err := controllerConn.CreateTopics(topicConfigs...); err != nil {
		logger.Warn("failed to create Kafka topics (may already exist)",
			slog.String("error", err.Error()),
		)
		return
	}

	for _, t := range topics {
		logger.Info("Kafka topic ensured", slog.String("topic", t))
	}
}

// buildHeaders extracts known context values (correlation ID) and returns
// them as Kafka message headers.
func buildHeaders(ctx context.Context) []kafkago.Header {
	var headers []kafkago.Header

	if corrID := logging.CorrelationIDFromContext(ctx); corrID != "" {
		headers = append(headers, kafkago.Header{
			Key:   headerCorrelationID,
			Value: []byte(corrID),
		})
	}

	return headers
}
