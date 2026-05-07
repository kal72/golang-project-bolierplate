package outbox

import (
	"context"
	"sync"
	"time"

	"golang-project-boilerplate/internal/shared/logger"
	"golang-project-boilerplate/internal/shared/rabbitmq"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type RelayerOptions struct {
	PollInterval time.Duration // jeda antar polling (default 2s)
	BatchSize    int           // jumlah row diambil per polling (default 50)
	MaxAttempts  int           // di atas ini → status=failed (default 10)
	BaseBackoff  time.Duration // backoff awal saat retry (default 5s)
	MaxBackoff   time.Duration // batas atas backoff (default 5m)
}

func (o *RelayerOptions) defaults() {
	if o.PollInterval <= 0 {
		o.PollInterval = 2 * time.Second
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 50
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = 10
	}
	if o.BaseBackoff <= 0 {
		o.BaseBackoff = 5 * time.Second
	}
	if o.MaxBackoff <= 0 {
		o.MaxBackoff = 5 * time.Minute
	}
}

// Relayer adalah background worker yang membaca outbox, publish ke RabbitMQ,
// dan menandai row sesuai hasil. Aman dijalankan di banyak instance (horizontal
// scale) berkat SKIP LOCKED di FetchPending.
type Relayer struct {
	repo *Repository
	conn *rabbitmq.Connection
	log  *logger.Logger
	opts RelayerOptions

	mu      sync.Mutex
	started bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewRelayer(db *gorm.DB, conn *rabbitmq.Connection, log *logger.Logger, opts RelayerOptions) *Relayer {
	opts.defaults()
	return &Relayer{
		repo: NewRepository(db),
		conn: conn,
		log:  log,
		opts: opts,
	}
}

func (r *Relayer) Start(ctx context.Context) {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return
	}
	r.started = true
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.mu.Unlock()

	r.wg.Add(1)
	go r.loop(runCtx)
	r.logInfo("outbox relayer started", logrus.Fields{
		"poll_interval": r.opts.PollInterval.String(),
		"batch_size":    r.opts.BatchSize,
	})
}

func (r *Relayer) Stop() {
	r.mu.Lock()
	if !r.started {
		r.mu.Unlock()
		return
	}
	r.started = false
	cancel := r.cancel
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	r.wg.Wait()
	r.logInfo("outbox relayer stopped", nil)
}

func (r *Relayer) loop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.opts.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

func (r *Relayer) tick(ctx context.Context) {
	rows, err := r.repo.FetchPending(ctx, r.opts.BatchSize)
	if err != nil {
		r.logError("fetch pending outbox failed", logrus.Fields{"error": err.Error()})
		return
	}
	if len(rows) == 0 {
		return
	}

	for _, row := range rows {
		if ctx.Err() != nil {
			return
		}
		r.publishOne(ctx, row)
	}
}

func (r *Relayer) publishOne(ctx context.Context, row Outbox) {
	pubCtx := rabbitmq.WithTraceID(ctx, row.TraceID)
	pubCtx = rabbitmq.WithRequestID(pubCtx, row.RequestID)

	pub := amqp.Publishing{
		MessageId:    row.ID,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         row.Payload,
	}

	err := r.conn.Publish(pubCtx, row.Exchange, row.RoutingKey, row.ContentType, pub)
	if err == nil {
		if mErr := r.repo.MarkSent(ctx, row.ID); mErr != nil {
			r.logError("mark sent failed", logrus.Fields{"id": row.ID, "error": mErr.Error()})
		}
		return
	}

	attempts := row.Attempts + 1
	fields := logrus.Fields{
		"id":       row.ID,
		"attempts": attempts,
		"error":    err.Error(),
	}

	if attempts >= r.opts.MaxAttempts {
		if mErr := r.repo.MarkFailed(ctx, row.ID, err.Error()); mErr != nil {
			r.logError("mark failed failed", logrus.Fields{"id": row.ID, "error": mErr.Error()})
		}
		r.logError("outbox row exhausted retries → failed", fields)
		return
	}

	backoff := r.computeBackoff(attempts)
	next := time.Now().Add(backoff)
	if mErr := r.repo.MarkRetry(ctx, row.ID, err.Error(), next); mErr != nil {
		r.logError("mark retry failed", logrus.Fields{"id": row.ID, "error": mErr.Error()})
	}
	fields["retry_in"] = backoff.String()
	r.logWarn("outbox publish failed → retry", fields)
}

func (r *Relayer) computeBackoff(attempts int) time.Duration {
	// exponential backoff: base * 2^(attempts-1), capped
	d := r.opts.BaseBackoff
	for i := 1; i < attempts; i++ {
		d *= 2
		if d >= r.opts.MaxBackoff {
			return r.opts.MaxBackoff
		}
	}
	return d
}

func (r *Relayer) logInfo(msg string, fields logrus.Fields) {
	if r.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "rabbitmq.outbox"
	r.log.Info(msg, fields)
}

func (r *Relayer) logWarn(msg string, fields logrus.Fields) {
	if r.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "rabbitmq.outbox"
	r.log.Warn(msg, fields)
}

func (r *Relayer) logError(msg string, fields logrus.Fields) {
	if r.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "rabbitmq.outbox"
	r.log.Error(msg, fields)
}
