package middleware

import (
	"log"
	"runtime/debug"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func HandleRecoveryPanic() fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				stack := string(debug.Stack())
				lines := strings.Split(stack, "\n")

				// Cari baris file.go:line pertama
				var sourceLine string
				for _, line := range lines {
					if strings.Contains(line, ".go:") {
						sourceLine = strings.TrimSpace(line)
						break
					}
				}

				// Log ke console
				log.Printf("[PANIC] %v | %s\n", r, sourceLine)

				// Response JSON
				_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error":   "internal_server_error",
					"message": r,          // pesan panic
					"stack":   sourceLine, // lokasi file.go:line
				})
			}
		}()
		return c.Next()
	}
}
