package router

import (
	"golang-project-boilerplate/internal/delivery/http/handler"
	"golang-project-boilerplate/internal/delivery/http/middleware"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type Route struct {
	App         *fiber.App
	Middleware  *middleware.Middleware
	PingHandler *handler.PingHandler
}

func NewRoute(
	app *fiber.App,
	middleware *middleware.Middleware,
	pingHandler *handler.PingHandler,
) *Route {
	return &Route{
		App:         app,
		Middleware:  middleware,
		PingHandler: pingHandler,
	}
}

func (r *Route) SetupRouter() {
	r.App.Use(r.Middleware.Recovery)
	r.App.Use(r.Middleware.Logging)
	r.App.Use(cors.New())

	r.App.Get("/ping", r.PingHandler.Ping)

	// Example of using auth middleware
	// v1 := r.App.Group("/api/v1")
	// v1.Use(r.Middleware.Auth)
}
