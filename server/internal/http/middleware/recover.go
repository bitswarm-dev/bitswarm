package middleware

import (
	"fmt"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/logger"
)

func Recover(log *logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				requestID, _ := c.Locals(LocalsRequestID).(string)
				log.With("request_id", requestID).Error("panic recovered", "panic", fmt.Sprintf("%v", r))
				err = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
			}
		}()
		return c.Next()
	}
}
