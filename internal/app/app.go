package app

import (
	"fmt"
	"golang-project-boilerplate/internal/config"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

func RunWithGracefulShutdown(fiberApp *fiber.App, cfg *config.Config) {
	go func() {
		host := cfg.App.Host
		port := cfg.App.Port
		err := fiberApp.Listen(fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	if err := fiberApp.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("error shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}
