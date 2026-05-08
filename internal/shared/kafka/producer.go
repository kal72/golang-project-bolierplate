package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/compress"
	"github.com/sirupsen/logrus"
)

// Producer wraps kafka.Writer with sane defaults: acks=all, idempotent batching,
// trace header injection, and circuit breaker inherited from Client.
type Producer struct {
	client *Client
	writer *kafka.Writer
}

func NewProducer(client *Client) *Producer {
	cfg := client.cfg

	w := &kafka.Writer{
		Addr:                   kafka.TCP(cfg.Brokers...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequiredAcks(intOrDefault(cfg.RequiredAcks, -1)),
		BatchSize:              intOrDefault(cfg.BatchSize, 100),
		BatchTimeout:           durationMs(cfg.BatchTimeout, 50*time.Millisecond),
		WriteTimeout:           durationFrom(cfg.WriteTimeout, 10*time.Second),
		ReadTimeout:            durationFrom(cfg.ReadTimeout, 10*time.Second),
		MaxAttempts:            intOrDefault(cfg.MaxAttempts, 5),
		Compression:            compressionFromName(cfg.Compression),
		AllowAutoTopicCreation: false,
		Transport:              client.transport,
		Async:                  false, // sync write for delivery confirmation
	}

	return &Producer{client: client, writer: w}
}

// Publish sends raw bytes to a topic. Trace headers are injected automatically.
func (p *Producer) Publish(ctx context.Context, topic, key string, value []byte, extra ...kafka.Header) error {
	messageID := uuid.NewString()
	headers := injectHeaders(ctx, messageID, extra)
	traceID := string(findHeader(headers, HeaderTraceID))

	msg := kafka.Message{
		Topic:   topic,
		Key:     []byte(key),
		Value:   value,
		Headers: headers,
		Time:    time.Now(),
	}

	start := time.Now()
	err := p.write(ctx, msg)

	if log := p.client.log; log != nil {
		fields := logrus.Fields{
			"component":   "kafka.producer",
			"topic":       topic,
			"key":         key,
			"message_id":  messageID,
			"trace_id":    traceID,
			"duration_ms": time.Since(start).Milliseconds(),
			"size_bytes":  len(value),
		}
		if err != nil {
			fields["error"] = err.Error()
			log.Error("publish failed", fields)
		} else {
			log.Info("publish ok", fields)
		}
	}
	return err
}

// PublishJSON marshals payload and publishes it. Returns wrapped error on
// marshal failure so callers can distinguish encoding from broker errors.
func (p *Producer) PublishJSON(ctx context.Context, topic, key string, payload any, extra ...kafka.Header) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("kafka: marshal payload: %w", err)
	}
	return p.Publish(ctx, topic, key, body, extra...)
}

func (p *Producer) write(ctx context.Context, msg kafka.Message) error {
	if p.client.breaker == nil {
		return p.writer.WriteMessages(ctx, msg)
	}
	_, err := p.client.breaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, p.writer.WriteMessages(ctx, msg)
	})
	return err
}

// Close flushes pending writes and releases writer resources.
func (p *Producer) Close() error {
	if p.writer == nil {
		return nil
	}
	return p.writer.Close()
}

func compressionFromName(name string) compress.Compression {
	switch name {
	case "gzip":
		return compress.Gzip
	case "snappy":
		return compress.Snappy
	case "lz4":
		return compress.Lz4
	case "zstd":
		return compress.Zstd
	default:
		return 0 // no compression
	}
}

func intOrDefault(v, fallback int) int {
	if v == 0 {
		return fallback
	}
	return v
}

func durationMs(ms int, fallback time.Duration) time.Duration {
	if ms <= 0 {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func findHeader(hs []kafka.Header, key string) []byte {
	for _, h := range hs {
		if h.Key == key {
			return h.Value
		}
	}
	return nil
}
