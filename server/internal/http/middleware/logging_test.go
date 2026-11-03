package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/logger"
)

func TestLogging(t *testing.T) {
	var buf bytes.Buffer
	log := logger.New(logger.Options{Level: logger.LevelInfo, Format: "console", Output: &buf})

	app := fiber.New()
	app.Use(RequestID())
	app.Use(Logging(log))
	app.Get("/ping", func(c *fiber.Ctx) error {
		return c.SendString("pong")
	})

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}

	out := buf.String()
	for _, want := range []string{"request handled", "method=GET", "path=/ping", "status=200"} {
		if !strings.Contains(out, want) {
			t.Errorf("log output missing %q, got: %q", want, out)
		}
	}
}

func TestFromContext(t *testing.T) {
	fallback := logger.New(logger.Options{Level: logger.LevelInfo})
	bound := logger.New(logger.Options{Level: logger.LevelInfo})

	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		c.Locals(LocalsLogger, bound)
		got := FromContext(c, fallback)
		if got != bound {
			t.Error("FromContext() did not return the bound logger")
		}
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
}
