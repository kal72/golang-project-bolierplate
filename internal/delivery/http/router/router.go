package router

import (
	"golang-project-boilerplate/internal/delivery/http/handler"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

type Route struct {
	App           *fiber.App
	LogMiddleware fiber.Handler
	PingHandler   *handler.PingHandler
}

func (r *Route) Setup() {
	r.App.Use(recover.New())
	r.App.Use(r.LogMiddleware)
	r.App.Get("/ping", r.PingHandler.Ping)
}
