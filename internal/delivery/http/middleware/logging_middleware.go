package middleware

import (
	"golang-project-boilerplate/internal/utils/logger"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func HandleReqLogging(log *logger.Logger) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		cid := ctx.Get("x-correlation-id")
		source := ctx.Get("x-source")
		reqId := uuid.New().String()
		if cid == "" {
			cid = uuid.New().String()
		}

		newLogField := logger.LogField{
			CorelationID: cid,
			RequestID:    reqId,
			Source:       source,
			Service:      log.AppName,
			ClientIP:     ctx.IP(),
			UserAgent:    string(ctx.Context().UserAgent()),
			HTTPMethod:   ctx.Method(),
			Endpoint:     ctx.OriginalURL(),
			StartTime:    time.Now(),
		}

		ctx.Locals(logger.SessionLogKey, newLogField)
		return ctx.Next()
	}
}

// func GetSessionLogger(ctx *fiber.Ctx) logrus.FieldLogger {
// 	return ctx.Locals(LoggerKey).(logrus.FieldLogger)
// }
