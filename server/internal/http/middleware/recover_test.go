package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/logger"
)

func TestRecover(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{Level: logger.LevelInfo, Format: "console", Output: &buf})

	app := fiber.New()
	app.Use(Recover(log))
	app.Get("/boom", func(c *fiber.Ctx) error {
		panic("kaboom")
	})

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}

	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("status = %d, want %d", resp.StatusCode, fiber.StatusInternalServerError)
	}
}
