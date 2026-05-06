//go:build wireinject

package di

import (
	"golang-project-boilerplate/internal/app"
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/grpc/handler"
	"golang-project-boilerplate/internal/delivery/grpc/middleware"
	"golang-project-boilerplate/internal/delivery/grpc/server"

	"github.com/google/wire"
)

var configSet = wire.NewSet(
	config.NewConfig,
	config.NewFiber,
	config.NewLogger,
	// config.NewDatabase, // uncomment when ready
	// config.NewRedis,    // uncomment when ready
)

var middlewareSet = wire.NewSet(
	middleware.NewMiddleware,
)

// var usecaseSet = wire.NewSet()

var handlerSet = wire.NewSet(
	handler.NewPingHandler,
)

var appSet = wire.NewSet(
	server.NewGrpcServer,
	app.NewApp,
)

// InitializeApp
func InitializeApp() (*app.App, error) {
	wire.Build(
		configSet,
		middlewareSet,
		// usecaseSet,
		handlerSet,
		appSet,
	)
	return nil, nil
}
