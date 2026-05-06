package app

import (
	"fmt"
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/grpc/server"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
)

type App struct {
	grpcServer *server.GrpcServer
	config     *config.Config
}

func NewApp(fiber *fiber.App, cfg *config.Config, grpcServer *server.GrpcServer) *App {
	return &App{
		grpcServer: grpcServer,
		config:     cfg,
	}
}

func (a *App) RunWithGracefulShutdown() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", a.config.App.GrpcPort))
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	grpc := a.grpcServer.SetupGrpc()
	go func() {
		fmt.Printf("Grpc server started on port %d\n", a.config.App.GrpcPort)
		err := grpc.Serve(lis)
		if err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	done := make(chan struct{})
	go func() {
		grpc.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Println("graceful shutdown complete")
	case <-time.After(5 * time.Second):
		log.Println("force shutdown...")
		grpc.Stop() // force kill
	}

	log.Println("Server stopped gracefully")
}
