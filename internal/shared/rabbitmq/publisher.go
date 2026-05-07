package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"golang-project-boilerplate/internal/shared/breaker"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type Publisher struct {
	conn    *Connection
	breaker *breaker.CircuitBreaker
}

func NewPublisher(conn *Connection) *Publisher {
	return &Publisher{conn: conn}
}

// NewPublisherWithBreaker membungkus publish dengan circuit breaker.
// Saat broker down/lambat, breaker akan Open dan publish gagal cepat tanpa
// menunggu timeout konfirmasi 10s — mencegah back-pressure ke caller.
func NewPublisherWithBreaker(conn *Connection, cb *breaker.CircuitBreaker) *Publisher {
	return &Publisher{conn: conn, breaker: cb}
}

func (p *Publisher) publish(ctx context.Context, exchange, routingKey, contentType string, body []byte) error {
	messageID := uuid.NewString()
	headers := injectHeaders(ctx, nil)
	traceID, _ := headers[HeaderTraceID].(string)

	pub := amqp.Publishing{
		MessageId:    messageID,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Headers:      headers,
		Body:         body,
	}

	start := time.Now()
	var err error
	if p.breaker != nil {
		_, err = p.breaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, p.conn.Publish(ctx, exchange, routingKey, contentType, pub)
		})
	} else {
		err = p.conn.Publish(ctx, exchange, routingKey, contentType, pub)
	}

	if log := p.conn.Logger(); log != nil {
		fields := logrus.Fields{
			"component":   "rabbitmq.publisher",
			"exchange":    exchange,
			"routing_key": routingKey,
			"message_id":  messageID,
			"trace_id":    traceID,
			"duration_ms": time.Since(start).Milliseconds(),
			"size_bytes":  len(body),
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

func (p *Publisher) PublishJSON(ctx context.Context, exchange, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	return p.publish(ctx, exchange, routingKey, "application/json", body)
}

func (p *Publisher) PublishToQueue(ctx context.Context, queue string, payload any) error {
	return p.PublishJSON(ctx, "", queue, payload)
}
