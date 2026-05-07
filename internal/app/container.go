package app

import (
	"log"
	"time"

	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
	"golang-project-boilerplate/internal/shared/breaker"
	"golang-project-boilerplate/internal/shared/rabbitmq"
	"golang-project-boilerplate/internal/shared/rabbitmq/outbox"
	"golang-project-boilerplate/internal/usecase/auth"

	"github.com/gofiber/fiber/v2"
)

// Container menyiapkan semua dependency dan mengembalikan Lifecycle yang
// dipakai oleh RunWithGracefulShutdown untuk menutup resource saat SIGINT/SIGTERM.
func Container(fiberApp *fiber.App, cfg *config.Config) *Lifecycle {
	// === infrastructure ===
	appLog := config.NewLogger(cfg)
	lifecycle := NewLifecycle(appLog)
	// logger ditutup paling akhir (di-Add paling awal) → flush log shutdown lainnya
	lifecycle.AddFunc("logger", func() error { appLog.Close(); return nil })

	// db := config.NewDatabase(cfg, appLog)
	// lifecycle.AddFunc("database", func() error {
	// 	sqlDB, err := db.DB()
	// 	if err != nil { return err }
	// 	return sqlDB.Close()
	// })

	// validate := config.NewValidator()
	// redisClient := config.NewRedis(cfg)
	// lifecycle.AddFunc("redis", func() error { return redisClient.Close() })

	// === RabbitMQ ===
	rmqConn, err := rabbitmq.NewConnection(cfg.RabbitMQ, appLog)
	if err != nil {
		log.Fatalf("init rabbitmq: %v", err)
	}
	lifecycle.AddFunc("rabbitmq", func() error { return rmqConn.Close() })

	// Publisher: pakai breaker dari config bila threshold > 0
	var publisher *rabbitmq.Publisher
	if cfg.RabbitMQ.Breaker.FailureThreshold > 0 {
		cb := breaker.NewCircuitBreaker(cfg.RabbitMQ.Breaker)
		publisher = rabbitmq.NewPublisherWithBreaker(rmqConn, cb)
	} else {
		publisher = rabbitmq.NewPublisher(rmqConn)
	}
	_ = publisher // inject ke usecase/handler sesuai kebutuhan

	// Outbox relayer (aktif bila enabled dan DB sudah siap)
	if cfg.RabbitMQ.Outbox.Enabled {
		// db wajib aktif untuk outbox; uncomment baris di atas dulu.
		// outboxRelayer := outbox.NewRelayer(db, rmqConn, appLog, outboxRelayerOpts(cfg.RabbitMQ.Outbox))
		// outboxRelayer.Start(context.Background())
		// lifecycle.AddFunc("outbox-relayer", func() error { outboxRelayer.Stop(); return nil })
		_ = outboxRelayerOpts // referenced ketika DB siap & uncomment di atas
	}

	// === Kafka (uncomment & wire when needed) ===
	// kafkaClient, err := kafka.NewClient(cfg.Kafka, appLog)
	// if err != nil { log.Fatalf("init kafka: %v", err) }
	// lifecycle.AddFunc("kafka-client", func() error { return kafkaClient.Close() })
	//
	// kafkaProducer := kafka.NewProducer(kafkaClient)
	// // or with breaker:
	// // cb := breaker.NewCircuitBreaker(cfg.Kafka.Breaker)
	// // kafkaProducer := kafka.NewProducerWithBreaker(kafkaClient, cb)
	// lifecycle.AddFunc("kafka-producer", func() error { return kafkaProducer.Close() })
	//
	// // Consumer with DLQ-on-error:
	// kafkaDLQ := kafka.NewProducer(kafkaClient)
	// lifecycle.AddFunc("kafka-dlq-producer", func() error { return kafkaDLQ.Close() })
	// kafkaConsumer, err := kafka.NewConsumer(kafkaClient, kafka.ConsumerOptions{
	// 	Topic:         "user.events",
	// 	Workers:       3,
	// 	HandleTimeout: 30 * time.Second,
	// 	ErrorPolicy:   kafka.PolicyDLQ,
	// 	DLQProducer:   kafkaDLQ,
	// }, func(ctx context.Context, msg kafkago.Message) error { return nil })
	// if err != nil { log.Fatalf("kafka consumer: %v", err) }
	// _ = kafkaConsumer.Start(context.Background())
	// lifecycle.AddFunc("kafka-consumer-user.events", func() error { kafkaConsumer.Stop(); return nil })

	// --- RabbitMQ Consumer (contoh) ---
	// userConsumer := rabbitmq.NewConsumer(rmqConn, rabbitmq.ConsumerOptions{
	// 	Queue:          "user.events",
	// 	ConsumerTag:    "user-svc",
	// 	Workers:        5,
	// 	HandleTimeout:  30 * time.Second,
	// 	RequeueOnError: false,
	// }, func(ctx context.Context, msg amqp091.Delivery) error { return nil })
	// if err := userConsumer.Start(context.Background()); err != nil {
	// 	log.Fatalf("start consumer: %v", err)
	// }
	// lifecycle.AddFunc("consumer-user.events", func() error { userConsumer.Stop(); return nil })

	// === usecase ===
	authUsecase := auth.NewAuthUsecase(cfg)

	// === handler ===
	pingHandler := handler.NewPingHandler()

	// === middleware ===
	loggingMiddleware := middleware.HandleReqLogging(appLog)
	recoveryMiddleware := middleware.HandleRecoveryPanic()
	authMiddleware := middleware.HandleAuth(authUsecase)

	route := &router.Route{
		App:               fiberApp,
		RecoverMiddleware: recoveryMiddleware,
		LogMiddleware:     loggingMiddleware,
		AuthMiddleware:    authMiddleware,
		PingHandler:       pingHandler,
	}
	route.RegisterRoutes()

	return lifecycle
}

// outboxRelayerOpts mengkonversi config JSON → opsi runtime.
func outboxRelayerOpts(c config.RabbitMQOutboxConfig) outbox.RelayerOptions {
	return outbox.RelayerOptions{
		PollInterval: time.Duration(c.PollInterval) * time.Second,
		BatchSize:    c.BatchSize,
		MaxAttempts:  c.MaxAttempts,
		BaseBackoff:  time.Duration(c.BaseBackoff) * time.Second,
		MaxBackoff:   time.Duration(c.MaxBackoff) * time.Second,
	}
}

