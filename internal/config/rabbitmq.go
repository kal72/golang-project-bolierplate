package config

import (
	"fmt"
	"log"
	"net/url"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func NewRabbitMQ(config *Config) *amqp.Connection {
	cfg := config.RabbitMQ

	vhost := cfg.VHost
	if vhost == "" {
		vhost = "/"
	}

	dsn := fmt.Sprintf("amqp://%s:%s@%s:%d/%s",
		url.QueryEscape(cfg.Username),
		url.QueryEscape(cfg.Password),
		cfg.Host,
		cfg.Port,
		url.PathEscape(vhost),
	)

	connectTimeout := time.Duration(cfg.ConnectTimeout) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 30 * time.Second
	}

	heartbeat := time.Duration(cfg.Heartbeat) * time.Second
	if heartbeat <= 0 {
		heartbeat = 10 * time.Second
	}

	conn, err := amqp.DialConfig(dsn, amqp.Config{
		Heartbeat: heartbeat,
		Locale:    "en_US",
		Dial:      amqp.DefaultDial(connectTimeout),
	})
	if err != nil {
		log.Fatalf("Gagal konek ke RabbitMQ: %v", err)
	}

	return conn
}

func NewRabbitMQChannel(conn *amqp.Connection, config *Config) *amqp.Channel {
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Gagal membuka channel RabbitMQ: %v", err)
	}

	if config.RabbitMQ.PrefetchCount > 0 {
		if err := ch.Qos(config.RabbitMQ.PrefetchCount, 0, false); err != nil {
			log.Fatalf("Gagal set QoS RabbitMQ: %v", err)
		}
	}

	return ch
}
