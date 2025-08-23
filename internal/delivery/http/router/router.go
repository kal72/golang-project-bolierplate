package router

import (
	"golang-project-boilerplate/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type Route struct {
	App               *fiber.App
	RecoverMiddleware fiber.Handler
	LogMiddleware     fiber.Handler
	AuthMiddleware    fiber.Handler
	PingHandler       *handler.PingHandler
}

func (r *Route) RegisterRoutes() {
	r.App.Use(r.RecoverMiddleware)
	r.App.Use(r.LogMiddleware)
	r.App.Use(cors.New())
	r.App.Get("/ping", r.PingHandler.Ping)

	// v1Group := r.App.Group("/api/v1")

}
