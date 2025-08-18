package config

import (
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
	"golang-project-boilerplate/internal/utils/logger"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

type AppConfig struct {
	FiberApp *fiber.App
	Log      *logger.Logger
	DB       *gorm.DB
	Validate *validator.Validate
	Config   *viper.Viper
}

func Container(config *AppConfig) {
	// setup middleware
	loggingMiddleware := middleware.HandleReqLogging(config.Log)

	// setup repositories

	// setup use cases

	// setup handler
	pingHandler := handler.NewPingHandler()

	route := router.Route{
		App:           config.FiberApp,
		LogMiddleware: loggingMiddleware,
		PingHandler:   pingHandler,
	}
	route.Setup()
}
