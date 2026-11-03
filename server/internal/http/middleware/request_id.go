package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
)

const LocalsRequestID = "request_id"

func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		c.Locals(LocalsRequestID, id)
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
