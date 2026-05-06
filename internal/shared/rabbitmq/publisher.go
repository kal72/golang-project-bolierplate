package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	ch *amqp.Channel
}

func NewPublisher(ch *amqp.Channel) *Publisher {
	return &Publisher{ch: ch}
}

func (p *Publisher) publish(ctx context.Context, exchange, routingKey, contentType string, body []byte) error {
	if p.ch == nil {
		return fmt.Errorf("rabbitmq channel is nil")
	}

	return p.ch.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  contentType,
			MessageId:    uuid.NewString(),
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
		},
	)
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
