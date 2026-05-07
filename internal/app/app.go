package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/router"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	fiber     *fiber.App
	config    *config.Config
	lifecycle *Lifecycle
}

func NewApp(fiber *fiber.App, cfg *config.Config, r *router.Route, lc *Lifecycle) *App {
	r.SetupRouter()
	return &App{
		fiber:     fiber,
		config:    cfg,
		lifecycle: lc,
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

	timeout := time.Duration(a.config.App.ShutdownTimeout) * time.Second
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := a.fiber.ShutdownWithContext(ctx); err != nil {
		log.Printf("fiber shutdown error: %v", err)
	}

	a.lifecycle.Shutdown(ctx)

	log.Println("Server stopped gracefully")
}
