package app

import (
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
	"golang-project-boilerplate/internal/usecase/auth"

	"github.com/gofiber/fiber/v2"
)

func Container(fiberApp *fiber.App, cfg *config.Config) {
	// setup infrastructure
	logger := config.NewLogger(cfg)
	// db := config.NewDatabase(cfg, logger)
	// validate := config.NewValidator()
	// redisClient := config.NewRedis(cfg)

	// setup repositories

	// setup usecases
	authUsecase := auth.NewAuthUsecase(cfg)

	// setup handler
	pingHandler := handler.NewPingHandler()

	// setup middleware
	loggingMiddleware := middleware.HandleReqLogging(logger)
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
}
