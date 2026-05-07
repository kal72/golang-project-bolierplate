package rabbitmq

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	defaultReconnectDelayFallback = 2 * time.Second
	maxReconnectDelayFallback     = 30 * time.Second
	publishConfirmTimeoutFallback = 10 * time.Second
)

func (c *Connection) reconnectDelays() (initial, max time.Duration) {
	initial = time.Duration(c.cfg.ReconnectDelay) * time.Second
	if initial <= 0 {
		initial = defaultReconnectDelayFallback
	}
	max = time.Duration(c.cfg.MaxReconnectDelay) * time.Second
	if max <= 0 {
		max = maxReconnectDelayFallback
	}
	if max < initial {
		max = initial
	}
	return
}

func (c *Connection) publishTimeout() time.Duration {
	d := time.Duration(c.cfg.PublishTimeout) * time.Second
	if d <= 0 {
		return publishConfirmTimeoutFallback
	}
	return d
}

type Connection struct {
	cfg config.RabbitMQConfig
	log *logger.Logger

	mu       sync.Mutex
	conn     *amqp.Connection
	channel  *amqp.Channel
	confirms chan amqp.Confirmation
	returns  chan amqp.Return

	closed bool
	done   chan struct{}
}

// NewConnection membuat koneksi RabbitMQ dengan auto-reconnect & confirm mode.
// Parameter log boleh nil; jika nil, log internal akan di-skip.
func NewConnection(cfg config.RabbitMQConfig, log *logger.Logger) (*Connection, error) {
	c := &Connection{
		cfg:  cfg,
		log:  log,
		done: make(chan struct{}),
	}

	if err := c.openConnAndChannel(); err != nil {
		return nil, err
	}

	go c.supervise()

	return c, nil
}

func (c *Connection) dsn() string {
	vhost := c.cfg.VHost
	if vhost == "" {
		vhost = "/"
	}
	scheme := "amqp"
	if c.cfg.TLS {
		scheme = "amqps"
	}
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		scheme,
		url.QueryEscape(c.cfg.Username),
		url.QueryEscape(c.cfg.Password),
		c.cfg.Host,
		c.cfg.Port,
		url.PathEscape(vhost),
	)
}

func (c *Connection) openConnAndChannel() error {
	connectTimeout := time.Duration(c.cfg.ConnectTimeout) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 30 * time.Second
	}
	heartbeat := time.Duration(c.cfg.Heartbeat) * time.Second
	if heartbeat <= 0 {
		heartbeat = 10 * time.Second
	}

	conn, err := amqp.DialConfig(c.dsn(), amqp.Config{
		Heartbeat: heartbeat,
		Locale:    "en_US",
		Dial:      amqp.DefaultDial(connectTimeout),
	})
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open channel: %w", err)
	}

	if c.cfg.PrefetchCount > 0 {
		if err := ch.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return fmt.Errorf("set qos: %w", err)
		}
	}

	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("enable confirm mode: %w", err)
	}

	confirms := ch.NotifyPublish(make(chan amqp.Confirmation, 1))
	returns := ch.NotifyReturn(make(chan amqp.Return, 1))

	c.mu.Lock()
	c.conn = conn
	c.channel = ch
	c.confirms = confirms
	c.returns = returns
	c.mu.Unlock()

	return nil
}

func (c *Connection) supervise() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))

		select {
		case <-c.done:
			return
		case err := <-closeCh:
			c.mu.Lock()
			isClosed := c.closed
			c.mu.Unlock()
			if isClosed {
				return
			}
			if err != nil {
				c.logWarn("rabbitmq connection closed, reconnecting", logrus.Fields{"error": err.Error()})
			}
			c.reconnectLoop()
		}
	}
}

func (c *Connection) reconnectLoop() {
	initial, max := c.reconnectDelays()
	delay := initial
	for {
		select {
		case <-c.done:
			return
		case <-time.After(delay):
		}

		c.mu.Lock()
		isClosed := c.closed
		c.mu.Unlock()
		if isClosed {
			return
		}

		if err := c.openConnAndChannel(); err != nil {
			c.logWarn("rabbitmq reconnect failed", logrus.Fields{"error": err.Error(), "retry_in": delay.String()})
			delay *= 2
			if delay > max {
				delay = max
			}
			continue
		}

		c.logInfo("rabbitmq reconnected", nil)
		return
	}
}

func (c *Connection) logInfo(msg string, fields logrus.Fields) {
	if c.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "rabbitmq"
	c.log.Info(msg, fields)
}

func (c *Connection) logWarn(msg string, fields logrus.Fields) {
	if c.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "rabbitmq"
	c.log.Warn(msg, fields)
}

func (c *Connection) Logger() *logger.Logger { return c.log }

func (c *Connection) RawChannel() *amqp.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.channel
}

func (c *Connection) Publish(ctx context.Context, exchange, routingKey, contentType string, pub amqp.Publishing) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("rabbitmq connection is closed")
	}
	if c.channel == nil || c.channel.IsClosed() {
		return fmt.Errorf("rabbitmq channel unavailable")
	}

	pub.ContentType = contentType
	if err := c.channel.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		true,  // mandatory: detect unroutable messages via NotifyReturn
		false, // immediate: deprecated by RabbitMQ
		pub,
	); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	publishTimeout := c.publishTimeout()
	timeout := time.NewTimer(publishTimeout)
	defer timeout.Stop()

	select {
	case ret, ok := <-c.returns:
		if ok {
			c.drainNextConfirm()
			return fmt.Errorf("message unroutable: %d %s (exchange=%q key=%q)",
				ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
		}
		return fmt.Errorf("publish failed: return channel closed")
	case confirm, ok := <-c.confirms:
		if !ok {
			return fmt.Errorf("publish failed: confirm channel closed")
		}
		if !confirm.Ack {
			return fmt.Errorf("publish nacked by broker (delivery_tag=%d)", confirm.DeliveryTag)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout.C:
		return fmt.Errorf("publish confirmation timeout after %s", publishTimeout)
	}
}

func (c *Connection) drainNextConfirm() {
	select {
	case <-c.confirms:
	case <-time.After(2 * time.Second):
	}
}

func (c *Connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	ch := c.channel
	conn := c.conn
	c.mu.Unlock()

	close(c.done)

	if ch != nil && !ch.IsClosed() {
		_ = ch.Close()
	}
	if conn != nil && !conn.IsClosed() {
		return conn.Close()
	}
	return nil
}
