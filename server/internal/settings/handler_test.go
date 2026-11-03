package settings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestHandler(t *testing.T) {
	svc, _, _ := newService(t)
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{"get", http.MethodGet, "/settings", "", 200},
		{"defaults", http.MethodGet, "/settings/defaults", "", 200},
		{"save ok", http.MethodPut, "/settings", `{"port":6000}`, 200},
		{"save invalid", http.MethodPut, "/settings", `{"port":0}`, 422},
		{"save bad json", http.MethodPut, "/settings", `{`, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.want)
			}
		})
	}
}
