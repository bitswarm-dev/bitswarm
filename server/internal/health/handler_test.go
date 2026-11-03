package health

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func newTestApp(repo Repository) *fiber.App {
	svc := NewService(repo, testLogger())
	h := NewHandler(svc)

	app := fiber.New()
	h.RegisterRoutes(app)
	return app
}

func TestHandler_Check(t *testing.T) {
	tests := []struct {
		name       string
		repoErr    error
		wantStatus int
	}{
		{name: "success", wantStatus: fiber.StatusOK},
		{name: "repository failure", repoErr: errors.New("db down"), wantStatus: fiber.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{recordResult: Check{ID: 1, CheckedAt: time.Now()}, recordErr: tc.repoErr}
			app := newTestApp(repo)

			req := httptest.NewRequest(http.MethodPost, "/health/check", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}

func TestHandler_Latest(t *testing.T) {
	tests := []struct {
		name       string
		repoErr    error
		wantStatus int
	}{
		{name: "success", wantStatus: fiber.StatusOK},
		{name: "not found", repoErr: errors.New("no rows"), wantStatus: fiber.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepository{latestResult: Check{ID: 1, CheckedAt: time.Now()}, latestErr: tc.repoErr}
			app := newTestApp(repo)

			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}
