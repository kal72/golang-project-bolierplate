package middleware

import (
	"golang-project-boilerplate/internal/utils/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

func HandleReqLogging(log *logger.Logger) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		cid := ctx.Get("x-correlation-id")
		source := ctx.Get("x-source")
		reqId := uuid.New().String()
		if cid == "" {
			cid = uuid.New().String()
		}

		logFieldMap := logrus.Fields{
			"corelation_id": cid,
			"request_id":    reqId,
			"client_ip":     ctx.IP(),
			"service":       log.AppName,
			"source":        source,
			"user_agent":    string(ctx.Context().UserAgent()),
			"http_method":   ctx.Method(),
			"http_status":   ctx.Response().StatusCode(),
			"endpoint":      ctx.OriginalURL(),
		}

		ctx.Locals(logger.SessionLogKey, logFieldMap)
		return ctx.Next()
	}
}
