package config

import (
	"log"

	"golang-project-boilerplate/internal/shared/kafka"
	"golang-project-boilerplate/internal/shared/logger"
)

// NewKafka membuat shared Kafka client (dialer + transport, TLS, SASL).
// Fatal saat gagal — konsisten dengan NewDatabase / NewRedis.
func NewKafka(cfg *Config, appLog *logger.Logger) *kafka.Client {
	client, err := kafka.NewClient(toKafkaConfig(cfg.Kafka), appLog)
	if err != nil {
		log.Fatalf("Gagal init Kafka: %v", err)
	}
	return client
}

func toKafkaConfig(c KafkaConfig) kafka.Config {
	return kafka.Config{
		Brokers:      c.Brokers,
		ClientID:     c.ClientID,
		TLS:          c.TLS,
		SASL:         kafka.SASLConfig(c.SASL),
		DialTimeout:  c.DialTimeout,
		WriteTimeout: c.WriteTimeout,
		ReadTimeout:  c.ReadTimeout,
		RequiredAcks: c.RequiredAcks,
		Compression:  c.Compression,
		BatchSize:    c.BatchSize,
		BatchTimeout: c.BatchTimeout,
		GroupID:      c.GroupID,
		StartOffset:  c.StartOffset,
		MinBytes:     c.MinBytes,
		MaxBytes:     c.MaxBytes,
		MaxAttempts:  c.MaxAttempts,
	}
}
