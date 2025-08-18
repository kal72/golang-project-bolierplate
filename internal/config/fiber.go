package config

import (
	"github.com/gofiber/fiber/v2"
	"github.com/spf13/viper"
)

func NewFiber(config *viper.Viper) *fiber.App {
	var app = fiber.New(fiber.Config{
		AppName:      config.GetString("app.name"),
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
