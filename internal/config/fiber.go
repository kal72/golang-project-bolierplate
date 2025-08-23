package config

import (
	"github.com/gofiber/fiber/v2"
)

func NewFiber(config *Config) *fiber.App {
	var app = fiber.New(fiber.Config{
		AppName:      config.App.Name,
		ErrorHandler: NewErrorHandler(),
	})

	return app
}

func NewErrorHandler() fiber.ErrorHandler {
	return func(ctx *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		status := "99"
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
			if code == fiber.StatusBadRequest {
				status = "04"
			}
		}

		return ctx.Status(code).JSON(fiber.Map{
			"status":  status,
			"message": err.Error(),
		})
	}
}
