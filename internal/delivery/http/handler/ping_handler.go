package handler

import (
	"github.com/gofiber/fiber/v2"
)

type PingHandler struct {
}

func NewPingHandler() *PingHandler {
	return &PingHandler{}
}

func (h *PingHandler) Ping(ctx *fiber.Ctx) error {
	return ctx.SendString("Server running...")
}
