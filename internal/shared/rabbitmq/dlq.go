package rabbitmq

import (
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DLQOptions mengkonfigurasi pasangan main queue + dead-letter queue.
//
// Pola yang dipakai (RabbitMQ DLX standard):
//   - main queue dideklarasikan dengan x-dead-letter-exchange → DLX
//   - DLX di-bind ke DLQ via routing key = nama main queue
//   - Pesan yang di-Nack(requeue=false), expired (TTL), atau di-reject akan
//     otomatis dirutekan ke DLQ.
//
// Convention default penamaan (jika tidak di-override):
//   - DLX:  "<queue>.dlx"
//   - DLQ:  "<queue>.dlq"
type DLQOptions struct {
	Queue            string        // nama main queue (wajib)
	DLXName          string        // nama dead-letter exchange (default "<queue>.dlx")
	DLQName          string        // nama dead-letter queue (default "<queue>.dlq")
	MessageTTL       time.Duration // TTL pesan di main queue; 0 = tanpa TTL
	MaxLength        int           // batas jumlah pesan di main queue; 0 = unlimited
	DLQMessageTTL    time.Duration // TTL pesan di DLQ; 0 = tanpa TTL
	ExtraQueueArgs   amqp.Table    // arg tambahan untuk main queue
	ExtraDLQArgs     amqp.Table    // arg tambahan untuk DLQ
}

// DeclareQueueWithDLQ membuat main queue + dead-letter exchange + dead-letter queue
// secara idempotent. Aman dipanggil berkali-kali.
//
// Mengembalikan info main queue. Caller bisa mengabaikan return value-nya.
func DeclareQueueWithDLQ(conn *Connection, opts DLQOptions) (amqp.Queue, error) {
	if opts.Queue == "" {
		return amqp.Queue{}, fmt.Errorf("queue name is required")
	}

	ch := conn.RawChannel()
	if ch == nil {
		return amqp.Queue{}, fmt.Errorf("rabbitmq channel unavailable")
	}

	dlxName := opts.DLXName
	if dlxName == "" {
		dlxName = opts.Queue + ".dlx"
	}
	dlqName := opts.DLQName
	if dlqName == "" {
		dlqName = opts.Queue + ".dlq"
	}
	routingKey := opts.Queue

	// 1. Declare DLX (durable, direct).
	if err := ch.ExchangeDeclare(
		dlxName,
		"direct",
		true,  // durable
		false, // autoDelete
		false, // internal
		false, // noWait
		nil,
	); err != nil {
		return amqp.Queue{}, fmt.Errorf("declare DLX %q: %w", dlxName, err)
	}

	// 2. Declare DLQ.
	dlqArgs := amqp.Table{}
	for k, v := range opts.ExtraDLQArgs {
		dlqArgs[k] = v
	}
	if opts.DLQMessageTTL > 0 {
		dlqArgs["x-message-ttl"] = opts.DLQMessageTTL.Milliseconds()
	}
	if _, err := ch.QueueDeclare(
		dlqName,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		dlqArgs,
	); err != nil {
		return amqp.Queue{}, fmt.Errorf("declare DLQ %q: %w", dlqName, err)
	}

	// 3. Bind DLQ ke DLX.
	if err := ch.QueueBind(dlqName, routingKey, dlxName, false, nil); err != nil {
		return amqp.Queue{}, fmt.Errorf("bind DLQ %q to DLX %q: %w", dlqName, dlxName, err)
	}

	// 4. Declare main queue dengan DLX reference.
	queueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxName,
		"x-dead-letter-routing-key": routingKey,
	}
	for k, v := range opts.ExtraQueueArgs {
		queueArgs[k] = v
	}
	if opts.MessageTTL > 0 {
		queueArgs["x-message-ttl"] = opts.MessageTTL.Milliseconds()
	}
	if opts.MaxLength > 0 {
		queueArgs["x-max-length"] = opts.MaxLength
	}

	q, err := ch.QueueDeclare(
		opts.Queue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		queueArgs,
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("declare main queue %q: %w", opts.Queue, err)
	}

	return q, nil
}
