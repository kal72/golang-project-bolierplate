package app

import (
	"fmt"
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/router"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	fiber  *fiber.App
	config *config.Config
}

func NewApp(fiber *fiber.App, cfg *config.Config, r *router.Route) *App {
	r.SetupRouter()
	return &App{
		fiber:  fiber,
		config: cfg,
	}
}

func (a *App) RunWithGracefulShutdown() {
	go func() {
		err := a.fiber.Listen(fmt.Sprintf("%s:%d", a.config.App.Host, a.config.App.Port))
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	if err := a.fiber.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("error shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
