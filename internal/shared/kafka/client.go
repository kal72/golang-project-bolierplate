package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"golang-project-boilerplate/internal/shared/logger"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"
	"github.com/sirupsen/logrus"
)

// Executor adalah abstraksi untuk middleware eksekusi seperti circuit breaker.
// Caller menyuntikkan implementasi konkret; package ini tidak bergantung pada
// package breaker secara langsung.
type Executor interface {
	Execute(ctx context.Context, fn func(context.Context) (interface{}, error)) (interface{}, error)
}

// Client packages config, logger, dialer, and transport so producer/consumer
// can share a consistent, secure connection setup.
type Client struct {
	cfg       Config
	log       *logger.Logger
	breaker   Executor
	dialer    *kafka.Dialer
	transport *kafka.Transport
}

func NewClient(cfg Config, log *logger.Logger) (*Client, error) {
	return newClient(cfg, log, nil)
}

// NewClientWithBreaker sama seperti NewClient tetapi setiap WriteMessages dari
// Producer yang dibuat dari Client ini akan dilindungi circuit breaker.
func NewClientWithBreaker(cfg Config, log *logger.Logger, cb Executor) (*Client, error) {
	return newClient(cfg, log, cb)
}

func newClient(cfg Config, log *logger.Logger, cb Executor) (*Client, error) {
	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka: brokers cannot be empty")
	}

	dialTimeout := durationFrom(cfg.DialTimeout, 10*time.Second)

	var tlsCfg *tls.Config
	if cfg.TLS {
		tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	saslMech, err := buildSASL(cfg.SASL)
	if err != nil {
		return nil, err
	}

	dialer := &kafka.Dialer{
		Timeout:       dialTimeout,
		ClientID:      cfg.ClientID,
		DualStack:     true,
		TLS:           tlsCfg,
		SASLMechanism: saslMech,
	}

	transport := &kafka.Transport{
		ClientID:    cfg.ClientID,
		DialTimeout: dialTimeout,
		TLS:         tlsCfg,
		SASL:        saslMech,
	}

	return &Client{
		cfg:       cfg,
		log:       log,
		breaker:   cb,
		dialer:    dialer,
		transport: transport,
	}, nil
}

func (c *Client) Config() Config              { return c.cfg }
func (c *Client) Logger() *logger.Logger     { return c.log }
func (c *Client) Dialer() *kafka.Dialer      { return c.dialer }
func (c *Client) Transport() *kafka.Transport { return c.transport }

// Close releases shared transport resources. Producers/consumers built from
// this client should be stopped first.
func (c *Client) Close() error {
	if c.transport != nil {
		c.transport.CloseIdleConnections()
	}
	return nil
}

func (c *Client) logInfo(msg string, fields logrus.Fields) {
	if c.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "kafka"
	c.log.Info(msg, fields)
}

func (c *Client) logError(msg string, fields logrus.Fields) {
	if c.log == nil {
		return
	}
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["component"] = "kafka"
	c.log.Error(msg, fields)
}

func buildSASL(cfg SASLConfig) (sasl.Mechanism, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	switch cfg.Mechanism {
	case "PLAIN", "":
		return plain.Mechanism{Username: cfg.Username, Password: cfg.Password}, nil
	case "SCRAM-SHA-256":
		m, err := scram.Mechanism(scram.SHA256, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("kafka: build SCRAM-SHA-256: %w", err)
		}
		return m, nil
	case "SCRAM-SHA-512":
		m, err := scram.Mechanism(scram.SHA512, cfg.Username, cfg.Password)
		if err != nil {
			return nil, fmt.Errorf("kafka: build SCRAM-SHA-512: %w", err)
		}
		return m, nil
	default:
		return nil, fmt.Errorf("kafka: unsupported SASL mechanism %q", cfg.Mechanism)
	}
}

func durationFrom(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}
