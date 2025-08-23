package app

import (
	"context"
	"fmt"
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
	"golang-project-boilerplate/internal/usecase/auth"
	"golang-project-boilerplate/internal/utils/logger"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type App struct {
	FiberApp *fiber.App
	Log      *logger.Logger
	DB       *gorm.DB
	Validate *validator.Validate
	Config   *config.Config
	Route    *router.Route
}

func NewContainer(cfg *config.Config) *App {
	// setup external libraries
	logger := config.NewLogger(cfg)
	fiberApp := config.NewFiber(cfg)
	db := config.NewDatabase(cfg, logger)
	validate := config.NewValidator()

	// setup repositories

	// setup usecases
	authUsecase := auth.NewAuthUsecase(cfg)

	// setup handler
	pingHandler := handler.NewPingHandler()

	// setup middleware
	loggingMiddleware := middleware.HandleReqLogging(logger)
	authMiddleware := middleware.HandleAuth(authUsecase)

	return &App{
		FiberApp: fiberApp,
		Log:      logger,
		DB:       db,
		Validate: validate,
		Config:   cfg,
		Route: &router.Route{
			App:            fiberApp,
			LogMiddleware:  loggingMiddleware,
			AuthMiddleware: authMiddleware,
			PingHandler:    pingHandler,
		},
	}
}

func (a *App) SetupRoutes() {
	a.Route.RegisterRoutes()
}

func (a *App) Run() {
	host := a.Config.App.Host
	port := a.Config.App.Port
	err := a.FiberApp.Listen(fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (a *App) GracefullyShutdown(appRun func()) {
	go appRun()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit // block sampai dapat signal
	fmt.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.FiberApp.Shutdown(); err != nil {
		log.Printf("error shutdown: %v", err)
	}

	<-ctx.Done()
	fmt.Println("Server stopped gracefully")
}
