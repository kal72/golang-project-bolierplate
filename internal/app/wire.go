//go:build wireinject

package app

import (
	"golang-project-boilerplate/internal/config"
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"
	"golang-project-boilerplate/internal/delivery/http/router"
	"golang-project-boilerplate/internal/usecase/auth"

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

var usecaseSet = wire.NewSet(
	auth.NewAuthUsecase,
)

var handlerSet = wire.NewSet(
	handler.NewPingHandler,
)

var routerSet = wire.NewSet(
	router.NewRoute,
)

// InitializeApp
func InitializeApp() (*App, error) {
	wire.Build(
		configSet,
		middlewareSet,
		usecaseSet,
		handlerSet,
		routerSet,
		NewApp,
	)
	return nil, nil
}
