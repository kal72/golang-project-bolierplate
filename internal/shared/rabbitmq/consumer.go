package rabbitmq

import (
	"context"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	defaultConsumerRestartDelay = 2 * time.Second
	maxConsumerRestartDelay     = 30 * time.Second
)

// Handler memproses satu pesan. Return nil → Ack, return error → Nack.
type Handler func(ctx context.Context, msg amqp.Delivery) error

type ConsumerOptions struct {
	Queue          string
	ConsumerTag    string
	AutoAck        bool          // jika true, broker auto-ack saat deliver (tidak direkomendasikan)
	Exclusive      bool
	NoLocal        bool
	NoWait         bool
	Args           amqp.Table
	Workers        int           // jumlah goroutine paralel (default 1)
	RequeueOnError bool          // jika handler error, requeue pesan (default false → masuk DLQ jika ada)
	HandleTimeout  time.Duration // timeout per pesan; 0 = tanpa timeout
}

type Consumer struct {
	conn    *Connection
	opts    ConsumerOptions
	handler Handler

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewConsumer(conn *Connection, opts ConsumerOptions, handler Handler) *Consumer {
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	return &Consumer{
		conn:    conn,
		opts:    opts,
		handler: handler,
	}
}

// Start mulai konsumsi pesan. Non-blocking. Saat connection drop & reconnect,
// consumer otomatis restart sesi konsumsi (re-consume + spawn worker baru).
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("consumer already started for queue %q", c.opts.Queue)
	}
	c.started = true
	c.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	c.wg.Add(1)
	go c.supervise(runCtx)

	c.logInfo("consumer started", logrus.Fields{
		"queue":   c.opts.Queue,
		"workers": c.opts.Workers,
	})
	return nil
}

// supervise menjalankan satu sesi konsumsi; saat delivery channel ditutup
// (mis. karena reconnect), tunggu sebentar lalu mulai sesi baru. Loop berhenti
// hanya saat ctx dibatalkan via Stop().
func (c *Consumer) supervise(ctx context.Context) {
	defer c.wg.Done()

	delay := defaultConsumerRestartDelay
	for {
		if ctx.Err() != nil {
			return
		}

		ok := c.runSession(ctx)
		if ctx.Err() != nil {
			return
		}

		if ok {
			// sesi sebelumnya jalan normal lalu delivery ditutup → reconnect terjadi.
			// Reset delay & coba langsung restart (Connection.supervise sudah pasti
			// sedang reconnect; consume() akan retry sampai berhasil).
			delay = defaultConsumerRestartDelay
		}

		c.logWarn("consumer session ended, restarting", logrus.Fields{
			"queue":    c.opts.Queue,
			"retry_in": delay.String(),
		})

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		delay *= 2
		if delay > maxConsumerRestartDelay {
			delay = maxConsumerRestartDelay
		}
	}
}

// runSession membuka satu sesi konsumsi: consume() + spawn workers.
// Return true jika sesi berhasil dimulai (worker sempat jalan), false jika
// consume() gagal langsung. Sesi berakhir saat ctx canceled atau delivery
// channel ditutup oleh broker/reconnect.
func (c *Consumer) runSession(ctx context.Context) bool {
	deliveries, err := c.consume()
	if err != nil {
		c.logWarn("consume failed", logrus.Fields{"queue": c.opts.Queue, "error": err.Error()})
		return false
	}

	var sessionWg sync.WaitGroup
	for i := 0; i < c.opts.Workers; i++ {
		sessionWg.Add(1)
		go func(id int) {
			defer sessionWg.Done()
			c.worker(ctx, id, deliveries)
		}(i)
	}

	sessionWg.Wait()
	return true
}

func (c *Consumer) consume() (<-chan amqp.Delivery, error) {
	ch := c.conn.RawChannel()
	if ch == nil || ch.IsClosed() {
		return nil, fmt.Errorf("rabbitmq channel unavailable")
	}
	return ch.Consume(
		c.opts.Queue,
		c.opts.ConsumerTag,
		c.opts.AutoAck,
		c.opts.Exclusive,
		c.opts.NoLocal,
		c.opts.NoWait,
		c.opts.Args,
	)
}

func (c *Consumer) worker(ctx context.Context, id int, deliveries <-chan amqp.Delivery) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-deliveries:
			if !ok {
				// Delivery channel ditutup → sesi berakhir. supervise akan restart.
				return
			}
			c.process(ctx, id, msg)
		}
	}
}

func (c *Consumer) process(parent context.Context, workerID int, msg amqp.Delivery) {
	ctx, traceID, requestID := extractHeaders(parent, msg.Headers)

	var cancel context.CancelFunc
	if c.opts.HandleTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.opts.HandleTimeout)
		defer cancel()
	}

	baseFields := logrus.Fields{
		"queue":      c.opts.Queue,
		"worker_id":  workerID,
		"message_id": msg.MessageId,
		"trace_id":   traceID,
		"request_id": requestID,
	}

	defer func() {
		if r := recover(); r != nil {
			fields := cloneFields(baseFields)
			fields["panic"] = fmt.Sprintf("%v", r)
			c.logError("consumer handler panic", fields)
			if !c.opts.AutoAck {
				_ = msg.Nack(false, c.opts.RequeueOnError)
			}
		}
	}()

	start := time.Now()
	err := c.handler(ctx, msg)

	fields := cloneFields(baseFields)
	fields["duration_ms"] = time.Since(start).Milliseconds()

	if c.opts.AutoAck {
		if err != nil {
			fields["error"] = err.Error()
			c.logError("consumer handler error (auto-ack)", fields)
		} else {
			c.logInfo("consumer handled", fields)
		}
		return
	}

	if err != nil {
		fields["error"] = err.Error()
		fields["requeue"] = c.opts.RequeueOnError
		c.logError("consumer handler error → nack", fields)
		_ = msg.Nack(false, c.opts.RequeueOnError)
		return
	}

	c.logInfo("consumer handled", fields)
	_ = msg.Ack(false)
}

func (c *Consumer) logInfo(msg string, fields logrus.Fields) {
	if log := c.conn.Logger(); log != nil {
		fields["component"] = "rabbitmq.consumer"
		log.Info(msg, fields)
	}
}

func (c *Consumer) logWarn(msg string, fields logrus.Fields) {
	if log := c.conn.Logger(); log != nil {
		fields["component"] = "rabbitmq.consumer"
		log.Warn(msg, fields)
	}
}

func (c *Consumer) logError(msg string, fields logrus.Fields) {
	if log := c.conn.Logger(); log != nil {
		fields["component"] = "rabbitmq.consumer"
		log.Error(msg, fields)
	}
}

func cloneFields(in logrus.Fields) logrus.Fields {
	out := make(logrus.Fields, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Stop graceful shutdown: berhenti terima pesan baru, tunggu in-flight selesai.
func (c *Consumer) Stop() {
	c.mu.Lock()
	if !c.started {
		c.mu.Unlock()
		return
	}
	c.started = false
	cancel := c.cancel
	c.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if c.opts.ConsumerTag != "" {
		if ch := c.conn.RawChannel(); ch != nil && !ch.IsClosed() {
			_ = ch.Cancel(c.opts.ConsumerTag, false)
		}
	}

	c.wg.Wait()
	c.logInfo("consumer stopped", logrus.Fields{"queue": c.opts.Queue})
}
