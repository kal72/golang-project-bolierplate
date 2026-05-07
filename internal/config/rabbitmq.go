package config

import (
	"log"

	"golang-project-boilerplate/internal/shared/logger"
	"golang-project-boilerplate/internal/shared/rabbitmq"
)

// NewRabbitMQ membuka koneksi RabbitMQ dengan auto-reconnect & confirm mode.
// Fatal saat gagal — konsisten dengan NewDatabase / NewRedis.
func NewRabbitMQ(cfg *Config, appLog *logger.Logger) *rabbitmq.Connection {
	conn, err := rabbitmq.NewConnection(toRabbitMQConfig(cfg.RabbitMQ), appLog)
	if err != nil {
		log.Fatalf("Gagal konek ke RabbitMQ: %v", err)
	}
	return conn
}

func toRabbitMQConfig(c RabbitMQConfig) rabbitmq.Config {
	return rabbitmq.Config{
		Username:          c.Username,
		Password:          c.Password,
		Host:              c.Host,
		Port:              c.Port,
		VHost:             c.VHost,
		TLS:               c.TLS,
		ConnectTimeout:    c.ConnectTimeout,
		Heartbeat:         c.Heartbeat,
		PrefetchCount:     c.PrefetchCount,
		ReconnectDelay:    c.ReconnectDelay,
		MaxReconnectDelay: c.MaxReconnectDelay,
		PublishTimeout:    c.PublishTimeout,
	}
}
