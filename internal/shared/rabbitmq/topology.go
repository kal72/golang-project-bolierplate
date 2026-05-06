package rabbitmq

import (
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

func DeclareQueue(ch *amqp.Channel, opts QueueOptions) (amqp.Queue, error) {
	return ch.QueueDeclare(
		opts.Name,
		opts.Durable,
		opts.AutoDelete,
		opts.Exclusive,
		opts.NoWait,
		opts.Args,
	)
}

func DeclareExchange(ch *amqp.Channel, opts ExchangeOptions) error {
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

func BindQueue(ch *amqp.Channel, queue, exchange, routingKey string, args amqp.Table) error {
	return ch.QueueBind(queue, routingKey, exchange, false, args)
}
