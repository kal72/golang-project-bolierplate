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

	"github.com/gofiber/fiber/v2"
)

const defaultShutdownTimeout = 15 * time.Second

func RunWithGracefulShutdown(fiberApp *fiber.App, cfg *config.Config, lifecycle *Lifecycle) {
	go func() {
		host := cfg.App.Host
		port := cfg.App.Port
		if err := fiberApp.Listen(fmt.Sprintf("%s:%d", host, port)); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 1. Stop menerima request HTTP baru, tunggu in-flight selesai.
	if err := fiberApp.ShutdownWithTimeout(5 * time.Second); err != nil {
		log.Printf("error fiber shutdown: %v", err)
	}

	// 2. Tutup resource lain (consumer → relayer → rabbitmq → db → logger)
	//    Lifecycle menutup dalam urutan LIFO.
	if lifecycle != nil {
		timeout := time.Duration(cfg.App.ShutdownTimeout) * time.Second
		if timeout <= 0 {
			timeout = defaultShutdownTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		lifecycle.Shutdown(ctx)
	}

	log.Println("Server stopped gracefully")
}
