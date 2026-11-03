package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name         string
		incomingID   string
		wantIncoming bool
	}{
		{name: "generates new id when absent"},
		{name: "reuses incoming header", incomingID: "existing-id", wantIncoming: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(RequestID())
			app.Get("/", func(c *fiber.Ctx) error {
				id, _ := c.Locals(LocalsRequestID).(string)
				return c.SendString(id)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.incomingID != "" {
				req.Header.Set("X-Request-ID", tc.incomingID)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}

			got := resp.Header.Get("X-Request-ID")
			if got == "" {
				t.Fatal("expected X-Request-ID header to be set")
			}
			if tc.wantIncoming && got != tc.incomingID {
				t.Errorf("X-Request-ID = %q, want reused %q", got, tc.incomingID)
			}
		})
	}
}
