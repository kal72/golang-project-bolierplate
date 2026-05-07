package kafka

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

// Handler processes one Kafka record. Returning nil → commit offset.
// Returning an error triggers the configured ErrorPolicy.
type Handler func(ctx context.Context, msg kafka.Message) error

// ErrorPolicy decides what to do when Handler returns an error.
type ErrorPolicy int

const (
	// PolicySkip: log + commit (skip the message). Use only if data loss is acceptable.
	PolicySkip ErrorPolicy = iota
	// PolicyDLQ: send to DLQ topic, then commit. Default best-practice.
	PolicyDLQ
	// PolicyRetryLocal: do not commit; the same message is re-fetched after
	// rebalance/restart. Risk: poison messages loop.
	PolicyRetryLocal
)

type ConsumerOptions struct {
	Topic         string
	GroupID       string        // overrides client default if set
	Workers       int           // number of parallel handler goroutines (default 1)
	HandleTimeout time.Duration // per-message timeout; 0 = no timeout
	ErrorPolicy   ErrorPolicy
	DLQProducer   *Producer // required when ErrorPolicy = PolicyDLQ
	StartOffset   string    // "earliest" | "latest" | "" (use client default)
	MinBytes      int
	MaxBytes      int
}

type Consumer struct {
	client  *Client
	reader  *kafka.Reader
	opts    ConsumerOptions
	handler Handler

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewConsumer(client *Client, opts ConsumerOptions, handler Handler) (*Consumer, error) {
	if opts.Topic == "" {
		return nil, fmt.Errorf("kafka: consumer topic required")
	}
	if opts.Workers <= 0 {
		opts.Workers = 1
	}
	if opts.ErrorPolicy == PolicyDLQ && opts.DLQProducer == nil {
		return nil, fmt.Errorf("kafka: PolicyDLQ requires DLQProducer")
	}

	groupID := opts.GroupID
	if groupID == "" {
		groupID = client.cfg.GroupID
	}
	if groupID == "" {
		return nil, fmt.Errorf("kafka: GroupID required (set in config or per-consumer)")
	}

	startOffset := opts.StartOffset
	if startOffset == "" {
		startOffset = client.cfg.StartOffset
	}

	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        client.cfg.Brokers,
		GroupID:        groupID,
		Topic:          opts.Topic,
		Dialer:         client.dialer,
		MinBytes:       intOrDefault(opts.MinBytes, intOrDefault(client.cfg.MinBytes, 1)),
		MaxBytes:       intOrDefault(opts.MaxBytes, intOrDefault(client.cfg.MaxBytes, 1<<20)),
		StartOffset:    startOffsetFromName(startOffset),
		CommitInterval: 0, // commit explicitly per-message
	})

	return &Consumer{
		client:  client,
		reader:  r,
		opts:    opts,
		handler: handler,
	}, nil
}

// Start begins consuming. Non-blocking. Stop() drains in-flight handlers.
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.started {
		c.mu.Unlock()
		return fmt.Errorf("kafka: consumer already started for topic %q", c.opts.Topic)
	}
	c.started = true
	c.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	for i := 0; i < c.opts.Workers; i++ {
		c.wg.Add(1)
		go c.worker(runCtx, i)
	}

	c.logInfo("consumer started", logrus.Fields{
		"topic":   c.opts.Topic,
		"group":   c.reader.Config().GroupID,
		"workers": c.opts.Workers,
	})
	return nil
}

// worker fetches and processes records sequentially. Multiple workers share
// the same Reader; kafka-go's FetchMessage is safe for concurrent use, and
// Kafka's per-partition ordering is preserved within each partition.
func (c *Consumer) worker(ctx context.Context, id int) {
	defer c.wg.Done()

	for {
		if ctx.Err() != nil {
			return
		}

		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
				return
			}
			c.logError("fetch message failed", logrus.Fields{
				"topic":     c.opts.Topic,
				"worker_id": id,
				"error":     err.Error(),
			})
			// brief backoff before retrying to avoid hot loop on persistent error
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
			continue
		}

		c.process(ctx, id, msg)
	}
}

func (c *Consumer) process(parent context.Context, workerID int, msg kafka.Message) {
	ctx, traceID, requestID, messageID := extractHeaders(parent, msg.Headers)

	var cancel context.CancelFunc
	if c.opts.HandleTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.opts.HandleTimeout)
		defer cancel()
	}

	baseFields := logrus.Fields{
		"topic":      msg.Topic,
		"partition":  msg.Partition,
		"offset":     msg.Offset,
		"key":        string(msg.Key),
		"worker_id":  workerID,
		"message_id": messageID,
		"trace_id":   traceID,
		"request_id": requestID,
	}

	defer func() {
		if r := recover(); r != nil {
			fields := cloneFields(baseFields)
			fields["panic"] = fmt.Sprintf("%v", r)
			c.logError("consumer handler panic", fields)
			c.handleError(ctx, msg, fmt.Errorf("panic: %v", r), fields)
		}
	}()

	start := time.Now()
	err := c.handler(ctx, msg)

	fields := cloneFields(baseFields)
	fields["duration_ms"] = time.Since(start).Milliseconds()

	if err != nil {
		fields["error"] = err.Error()
		c.logError("consumer handler error", fields)
		c.handleError(ctx, msg, err, fields)
		return
	}

	if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
		fields["commit_error"] = commitErr.Error()
		c.logError("commit failed", fields)
		return
	}
	c.logInfo("consumer handled", fields)
}

func (c *Consumer) handleError(ctx context.Context, msg kafka.Message, handlerErr error, fields logrus.Fields) {
	switch c.opts.ErrorPolicy {
	case PolicyRetryLocal:
		// Do not commit. The message will be re-delivered after consumer
		// restart or rebalance. Caller must accept poison-message risk.
		return
	case PolicyDLQ:
		if err := SendToDLQ(ctx, c.opts.DLQProducer, msg, handlerErr); err != nil {
			fields["dlq_error"] = err.Error()
			c.logError("send to DLQ failed; offset NOT committed", fields)
			return // leave uncommitted so the message survives restart
		}
		// Successfully sent to DLQ → safe to commit and move on.
	case PolicySkip:
		// fallthrough to commit
	}

	if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
		fields["commit_error"] = commitErr.Error()
		c.logError("commit after error-policy failed", fields)
	}
}

// Stop performs a graceful shutdown: cancel fetch, drain in-flight handlers,
// close the reader.
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
	c.wg.Wait()

	if err := c.reader.Close(); err != nil {
		c.logError("reader close failed", logrus.Fields{"topic": c.opts.Topic, "error": err.Error()})
	}
	c.logInfo("consumer stopped", logrus.Fields{"topic": c.opts.Topic})
}

func (c *Consumer) logInfo(msg string, fields logrus.Fields) {
	if log := c.client.log; log != nil {
		fields["component"] = "kafka.consumer"
		log.Info(msg, fields)
	}
}

func (c *Consumer) logError(msg string, fields logrus.Fields) {
	if log := c.client.log; log != nil {
		fields["component"] = "kafka.consumer"
		log.Error(msg, fields)
	}
}

func startOffsetFromName(name string) int64 {
	switch name {
	case "earliest":
		return kafka.FirstOffset
	default:
		return kafka.LastOffset
	}
}

func cloneFields(in logrus.Fields) logrus.Fields {
	out := make(logrus.Fields, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
