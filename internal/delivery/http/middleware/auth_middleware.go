package middleware

import (
	"golang-project-boilerplate/internal/shared/response"
	"golang-project-boilerplate/internal/usecase/auth"

	"github.com/gofiber/fiber/v2"
)

func HandleAuth(authUsecase auth.AuthUsecaseContract) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		token := ctx.Get("Authorization")
		tokenAuth, errData := authUsecase.Verify(ctx.UserContext(), token)
		if errData != nil {
			return response.ResponseError(ctx, errData)
		}

		ctx.Locals("auth", tokenAuth)
		return ctx.Next()
	}
}
