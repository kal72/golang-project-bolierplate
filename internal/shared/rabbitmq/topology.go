package rabbitmq

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type QueueOptions struct {
	Name       string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool
	Args       amqp.Table
}

type ExchangeOptions struct {
	Name       string
	Kind       string // "direct", "fanout", "topic", "headers"
	Durable    bool
	AutoDelete bool
	Internal   bool
	NoWait     bool
	Args       amqp.Table
}

func DeclareQueue(conn *Connection, opts QueueOptions) (amqp.Queue, error) {
	ch := conn.RawChannel()
	if ch == nil {
		return amqp.Queue{}, fmt.Errorf("rabbitmq channel unavailable")
	}
	return ch.QueueDeclare(
		opts.Name,
		opts.Durable,
		opts.AutoDelete,
		opts.Exclusive,
		opts.NoWait,
		opts.Args,
	)
}

func DeclareExchange(conn *Connection, opts ExchangeOptions) error {
	ch := conn.RawChannel()
	if ch == nil {
		return fmt.Errorf("rabbitmq channel unavailable")
	}
	kind := opts.Kind
	if kind == "" {
		kind = "direct"
	}
	return ch.ExchangeDeclare(
		opts.Name,
		kind,
		opts.Durable,
		opts.AutoDelete,
		opts.Internal,
		opts.NoWait,
		opts.Args,
	)
}

func BindQueue(conn *Connection, queue, exchange, routingKey string, args amqp.Table) error {
	ch := conn.RawChannel()
	if ch == nil {
		return fmt.Errorf("rabbitmq channel unavailable")
	}
	return ch.QueueBind(queue, routingKey, exchange, false, args)
}
