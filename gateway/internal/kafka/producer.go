// Package kafka provides a thin wrapper around confluent-kafka-go for publishing
// order-lifecycle, market-data, and venue-status events as serialized protobuf payloads.
package kafka

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/synapse-oms/gateway/internal/logging"
)

// Topic constants for the three event streams.
const (
	TopicOrderLifecycle = "order-lifecycle"
	TopicMarketData     = "market-data"
	TopicVenueStatus    = "venue-status"
)

// Header key used to propagate correlation IDs across Kafka messages.
const headerCorrelationID = "correlation-id"

// Producer wraps a confluent-kafka-go producer with typed publish helpers
// for each SynapseOMS event stream.
type Producer struct {
	producer *kafka.Producer
	logger   *slog.Logger
	doneCh   chan struct{}
}

// NewProducer creates a Kafka producer connected to the given broker list
// (comma-separated). It starts an internal goroutine for delivery reports.
func NewProducer(brokers string, logger *slog.Logger) (*Producer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": brokers,
		"acks":              "all",
		"retries":           3,
		"retry.backoff.ms":  100,
		"linger.ms":         5,
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}

	kp := &Producer{
		producer: p,
		logger:   logger,
		doneCh:   make(chan struct{}),
	}

	go kp.deliveryReportHandler()

	return kp, nil
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

// Close flushes any pending messages (up to 10 s) and shuts down the producer.
func (p *Producer) Close() {
	p.producer.Flush(10000)
	p.producer.Close()
	close(p.doneCh)
}

// publish is the shared implementation for all topic-specific publish methods.
func (p *Producer) publish(ctx context.Context, topic, key string, payload []byte) error {
	headers := buildHeaders(ctx)

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &topic,
			Partition: kafka.PartitionAny,
		},
		Key:     []byte(key),
		Value:   payload,
		Headers: headers,
	}

	if err := p.producer.Produce(msg, nil); err != nil {
		return fmt.Errorf("produce to %s: %w", topic, err)
	}

	return nil
}

// buildHeaders extracts known context values (correlation ID) and returns
// them as Kafka message headers.
func buildHeaders(ctx context.Context) []kafka.Header {
	var headers []kafka.Header

	if corrID := logging.CorrelationIDFromContext(ctx); corrID != "" {
		headers = append(headers, kafka.Header{
			Key:   headerCorrelationID,
			Value: []byte(corrID),
		})
	}

	return headers
}

// deliveryReportHandler consumes the producer's Events channel and logs
// delivery successes and failures. It runs until the channel is closed
// (which happens when the underlying producer is closed).
func (p *Producer) deliveryReportHandler() {
	for e := range p.producer.Events() {
		switch ev := e.(type) {
		case *kafka.Message:
			if ev.TopicPartition.Error != nil {
				p.logger.Error("delivery failed",
					slog.String("topic", *ev.TopicPartition.Topic),
					slog.String("key", string(ev.Key)),
					slog.String("error", ev.TopicPartition.Error.Error()),
				)
			} else {
				p.logger.Debug("message delivered",
					slog.String("topic", *ev.TopicPartition.Topic),
					slog.Int64("offset", int64(ev.TopicPartition.Offset)),
					slog.Int("partition", int(ev.TopicPartition.Partition)),
				)
			}
		}
	}
}
