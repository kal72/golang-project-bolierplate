package app

import (
	"time"

	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
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
