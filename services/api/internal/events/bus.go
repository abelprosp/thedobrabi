package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/thedobra/thedobra/services/api/internal/platform"
)

var Topics = []string{
	"data.ingestion",
	"data.validation",
	"data.transformation",
	"data.completed",
	"dataset.created",
	"dataset.updated",
	"query.requested",
	"query.completed",
	"dashboard.created",
	"dashboard.updated",
	"insight.detected",
	"anomaly.detected",
	"forecast.completed",
	"alert.created",
	"alert.triggered",
	"ai.requested",
	"ai.completed",
}

type KafkaBus struct {
	writers map[string]*kafkago.Writer
	log     *slog.Logger
}

func Connect(brokers []string, log *slog.Logger) platform.EventBus {
	if len(brokers) == 0 {
		return NoopBus{log: log}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	conn, err := kafkago.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		log.Warn("kafka unavailable, using noop bus", "err", err)
		return NoopBus{log: log}
	}
	defer conn.Close()
	for _, t := range Topics {
		_ = conn.CreateTopics(kafkago.TopicConfig{Topic: t, NumPartitions: 3, ReplicationFactor: 1})
	}
	writers := make(map[string]*kafkago.Writer, len(Topics))
	for _, t := range Topics {
		writers[t] = &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        t,
			Balancer:     &kafkago.Hash{},
			RequiredAcks: kafkago.RequireOne,
			Async:        true,
			BatchTimeout: 50 * time.Millisecond,
		}
	}
	return &KafkaBus{writers: writers, log: log}
}

func (b *KafkaBus) Publish(ctx context.Context, topic string, event platform.Event) error {
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Version == 0 {
		event.Version = 1
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if event.IdempotencyKey == "" {
		event.IdempotencyKey = event.ID
	}
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	w, ok := b.writers[topic]
	if !ok {
		return nil
	}
	return w.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(event.IdempotencyKey),
		Value: body,
		Headers: []kafkago.Header{
			{Key: "event-type", Value: []byte(event.Type)},
			{Key: "org-id", Value: []byte(event.OrgID)},
		},
	})
}

func (b *KafkaBus) Close() error {
	for _, w := range b.writers {
		_ = w.Close()
	}
	return nil
}

type NoopBus struct{ log *slog.Logger }

func (NoopBus) Publish(context.Context, string, platform.Event) error { return nil }
func (NoopBus) Close() error                                          { return nil }
