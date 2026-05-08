package rabbitmq

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"golang-project-boilerplate/internal/shared/logger"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// Executor adalah abstraksi untuk middleware eksekusi seperti circuit breaker.
// Caller menyuntikkan implementasi konkret; package ini tidak bergantung pada
// package breaker secara langsung.
type Executor interface {
	Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error)
}

const (
	defaultReconnectDelayFallback = 2 * time.Second
	maxReconnectDelayFallback     = 30 * time.Second
	publishConfirmTimeoutFallback = 10 * time.Second
	defaultChannelPoolSize        = 5
)

// pooledChannel adalah satu slot di publish channel pool. Setiap slot memiliki
// channel AMQP sendiri dengan confirm mode aktif, sehingga banyak goroutine
// bisa publish secara paralel tanpa saling memblokir.
type pooledChannel struct {
	ch       *amqp.Channel
	confirms chan amqp.Confirmation
	returns  chan amqp.Return
}

type Connection struct {
	cfg     Config
	log     *logger.Logger
	breaker Executor

	mu      sync.Mutex
	conn    *amqp.Connection
	adminCh *amqp.Channel      // channel khusus consumer & topology (tanpa confirm mode)
	pool    chan *pooledChannel // publish channel pool (dengan confirm mode)

	closed bool
	done   chan struct{}
}

// NewConnection membuat koneksi RabbitMQ dengan auto-reconnect dan channel pool
// untuk publish paralel. Parameter log boleh nil.
func NewConnection(cfg Config, log *logger.Logger) (*Connection, error) {
	return newConnection(cfg, log, nil)
}

// NewConnectionWithBreaker sama seperti NewConnection tetapi melindungi setiap
// Publish dengan circuit breaker — berlaku untuk semua jalur publish termasuk
// outbox relayer yang memanggil conn.Publish() langsung.
func NewConnectionWithBreaker(cfg Config, log *logger.Logger, cb Executor) (*Connection, error) {
	return newConnection(cfg, log, cb)
}

func newConnection(cfg Config, log *logger.Logger, cb Executor) (*Connection, error) {
	size := cfg.ChannelPoolSize
	if size <= 0 {
		size = defaultChannelPoolSize
	}
	c := &Connection{
		cfg:     cfg,
		log:     log,
		breaker: cb,
		pool:    make(chan *pooledChannel, size),
		done:    make(chan struct{}),
	}

	if err := c.open(); err != nil {
		return nil, err
	}

	go c.supervise()
	return c, nil
}

// open membuka koneksi AMQP, admin channel, dan mengisi publish pool.
func (c *Connection) open() error {
	conn, err := c.dial()
	if err != nil {
		return err
	}

	adminCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("open admin channel: %w", err)
	}

	if c.cfg.PrefetchCount > 0 {
		if err := adminCh.Qos(c.cfg.PrefetchCount, 0, false); err != nil {
			_ = adminCh.Close()
			_ = conn.Close()
			return fmt.Errorf("set qos: %w", err)
		}
	}

	c.mu.Lock()
	c.conn = conn
	c.adminCh = adminCh
	c.mu.Unlock()

	return c.fillPool(conn)
}

func (c *Connection) dial() (*amqp.Connection, error) {
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
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	return conn, nil
}

// fillPool membuat cap(c.pool) channel dengan confirm mode dan memasukkannya ke pool.
func (c *Connection) fillPool(conn *amqp.Connection) error {
	for i := 0; i < cap(c.pool); i++ {
		pc, err := c.newPooledChannel(conn)
		if err != nil {
			return fmt.Errorf("fill publish pool (slot %d): %w", i, err)
		}
		c.pool <- pc
	}
	return nil
}

func (c *Connection) newPooledChannel(conn *amqp.Connection) (*pooledChannel, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("open publish channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("enable confirm mode: %w", err)
	}
	return &pooledChannel{
		ch:       ch,
		confirms: ch.NotifyPublish(make(chan amqp.Confirmation, 1)),
		returns:  ch.NotifyReturn(make(chan amqp.Return, 1)),
	}, nil
}

// drainPool mengosongkan pool dan menutup semua channel di dalamnya.
// Channel yang sedang dipakai (checked-out) tidak terpengaruh — mereka akan
// discarded saat dikembalikan via releaseChannel.
func (c *Connection) drainPool() {
	for {
		select {
		case pc := <-c.pool:
			_ = pc.ch.Close()
		default:
			return
		}
	}
}

// acquireChannel mengambil satu channel dari pool. Memblokir sampai ada channel
// tersedia, context dibatalkan, atau koneksi ditutup.
func (c *Connection) acquireChannel(ctx context.Context) (*pooledChannel, error) {
	select {
	case pc := <-c.pool:
		return pc, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("rabbitmq connection is closed")
	}
}

// releaseChannel mengembalikan channel ke pool. Jika channel sudah closed
// (akibat reconnect), channel dibuang dan tidak dikembalikan.
func (c *Connection) releaseChannel(pc *pooledChannel) {
	if pc.ch.IsClosed() {
		return
	}
	select {
	case c.pool <- pc:
	default:
		_ = pc.ch.Close()
	}
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
			c.drainPool()
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

		if err := c.open(); err != nil {
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

func (c *Connection) Logger() *logger.Logger { return c.log }

// RawChannel mengembalikan admin channel — dipakai oleh consumer dan topology.
// Channel ini tidak dalam confirm mode dan tidak berasal dari publish pool.
func (c *Connection) RawChannel() *amqp.Channel {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.adminCh
}

func (c *Connection) Publish(ctx context.Context, exchange, routingKey, contentType string, pub amqp.Publishing) error {
	if c.breaker != nil {
		_, err := c.breaker.Execute(ctx, func(ctx context.Context) (interface{}, error) {
			return nil, c.doPublish(ctx, exchange, routingKey, contentType, pub)
		})
		return err
	}
	return c.doPublish(ctx, exchange, routingKey, contentType, pub)
}

// doPublish mengambil channel dari pool, publish, dan menunggu confirm.
// Tidak ada global mutex — setiap goroutine bekerja pada channel sendiri.
func (c *Connection) doPublish(ctx context.Context, exchange, routingKey, contentType string, pub amqp.Publishing) error {
	pc, err := c.acquireChannel(ctx)
	if err != nil {
		return err
	}
	defer c.releaseChannel(pc)

	pub.ContentType = contentType
	if err := pc.ch.PublishWithContext(ctx, exchange, routingKey, true, false, pub); err != nil {
		return fmt.Errorf("publish: %w", err)
	}

	publishTimeout := c.publishTimeout()
	timeout := time.NewTimer(publishTimeout)
	defer timeout.Stop()

	select {
	case ret, ok := <-pc.returns:
		if ok {
			select {
			case <-pc.confirms:
			case <-time.After(2 * time.Second):
			}
			return fmt.Errorf("message unroutable: %d %s (exchange=%q key=%q)",
				ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
		}
		return fmt.Errorf("publish failed: return channel closed")
	case confirm, ok := <-pc.confirms:
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

func (c *Connection) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	adminCh := c.adminCh
	conn := c.conn
	c.mu.Unlock()

	close(c.done)
	c.drainPool()

	if adminCh != nil && !adminCh.IsClosed() {
		_ = adminCh.Close()
	}
	if conn != nil && !conn.IsClosed() {
		return conn.Close()
	}
	return nil
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
