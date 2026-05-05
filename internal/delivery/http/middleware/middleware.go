package middleware

import (
	"golang-project-boilerplate/internal/shared/logger"
	"golang-project-boilerplate/internal/usecase/auth"

	"github.com/gofiber/fiber/v2"
)

type Middleware struct {
	Recovery fiber.Handler
	Logging  fiber.Handler
	Auth     fiber.Handler
}

func NewMiddleware(
	log *logger.Logger,
	authUsecase auth.AuthUsecaseContract,
) *Middleware {
	return &Middleware{
		Recovery: HandleRecoveryPanic(),
		Logging:  HandleReqLogging(log),
		Auth:     HandleAuth(authUsecase),
	}
}
