package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/logger"
)

const LocalsLogger = "logger"

func Logging(log *logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID, _ := c.Locals(LocalsRequestID).(string)
		reqLog := log.With("request_id", requestID)
		c.Locals(LocalsLogger, reqLog)

		start := time.Now()
		err := c.Next()

		reqLog.Info("request handled",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
		)

		return err
	}
}

func FromContext(c *fiber.Ctx, fallback *logger.Logger) *logger.Logger {
	if l, ok := c.Locals(LocalsLogger).(*logger.Logger); ok {
		return l
	}
	return fallback
}
