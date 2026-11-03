package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/logger"
)

func TestNew_RegistersRoutersAndMiddleware(t *testing.T) {
	log := logger.New(logger.Options{Level: logger.LevelInfo})

	called := false
	app := New(log, func(r fiber.Router) {
		called = true
		r.Get("/ping", func(c *fiber.Ctx) error {
			return c.SendString("pong")
		})
	})

	if !called {
		t.Fatal("expected router function to be called")
	}

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	if resp.StatusCode != fiber.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("expected X-Request-ID header set by middleware")
	}
}
