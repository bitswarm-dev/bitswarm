package tokens

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
)

type memLog struct{ lines []string }

func (m *memLog) Add(level, _, msg string) { m.lines = append(m.lines, level+": "+msg) }

func newSvc(t *testing.T) (*Service, *memLog, *time.Time) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	l := &memLog{}
	svc := NewService(NewCollection(conn), l)
	now := time.Now()
	svc.now = func() time.Time { return now }
	return svc, l, &now
}

func TestCreateVerifyRevoke(t *testing.T) {
	svc, log, now := newSvc(t)
	ctx := context.Background()

	c, err := svc.Create(ctx, CreateReq{Name: "  Home Assistant ", Scope: "read", Days: 30})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.Token, "swm_") || len(c.Token) != 44 || c.View.Last4 != c.Token[40:] || c.View.Name != "Home Assistant" {
		t.Errorf("token = %q, view = %+v", c.Token, c.View)
	}
	if c.View.Expires != now.UnixMilli()+30*day {
		t.Errorf("expires = %d", c.View.Expires)
	}
	stored, _ := svc.col.Get(ctx, c.View.ID)
	if stored.Hash == c.Token || stored.Hash != Hash(c.Token) {
		t.Error("the raw token must never be stored")
	}
	if !strings.Contains(log.lines[0], "read only access, expiring in 30 days") {
		t.Errorf("log = %v", log.lines)
	}

	if scope, ok := svc.Verify(ctx, c.Token); !ok || scope != "read" {
		t.Errorf("Verify() = %q, %v", scope, ok)
	}
	if v, _ := svc.List(ctx); v[0].LastUsed == 0 {
		t.Error("lastUsed not recorded")
	}
	for _, bad := range []string{"", "swm_nope", "Bearer x", strings.Replace(c.Token, c.Token[10:12], "ZZ", 1)} {
		if _, ok := svc.Verify(ctx, bad); ok {
			t.Errorf("Verify(%q) accepted", bad)
		}
	}

	*now = now.Add(31 * 24 * time.Hour)
	if _, ok := svc.Verify(ctx, c.Token); ok {
		t.Error("expired token accepted")
	}
	if v, _ := svc.List(ctx); !v[0].Expired || svc.ActiveCount(ctx) != 0 {
		t.Errorf("expired view = %+v", v[0])
	}

	never, _ := svc.Create(ctx, CreateReq{Name: "n", Scope: "admin", Days: 0})
	if never.View.Expires != 0 || svc.ActiveCount(ctx) != 1 {
		t.Errorf("never-expiring token = %+v", never.View)
	}
	if v, _ := svc.List(ctx); v[0].Name != "n" {
		t.Errorf("list should be newest first: %+v", v)
	}
	if err := svc.Revoke(ctx, never.View.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := svc.Verify(ctx, never.Token); ok {
		t.Error("revoked token accepted")
	}
	if err := svc.Revoke(ctx, never.View.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke twice err = %v", err)
	}
	if !strings.HasPrefix(log.lines[len(log.lines)-1], "warn: API tokens") && !strings.Contains(log.lines[len(log.lines)-1], "Revoked token “n”") {
		t.Errorf("log = %v", log.lines)
	}
}

func TestCreateValidation(t *testing.T) {
	svc, _, _ := newSvc(t)
	tests := []struct {
		name    string
		req     CreateReq
		wantErr string
	}{
		{"no name", CreateReq{Scope: "read", Days: 30}, "Give the token a name"},
		{"long name", CreateReq{Name: strings.Repeat("x", 61), Scope: "read"}, "60 characters"},
		{"bad scope", CreateReq{Name: "x", Scope: "root"}, "access level"},
		{"bad expiry", CreateReq{Name: "x", Scope: "read", Days: 7}, "expires"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Create(context.Background(), tc.req); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestAllowed(t *testing.T) {
	tests := []struct {
		scope, method, path string
		want                bool
	}{
		{"admin", "DELETE", "/api/settings", true},
		{"admin", "POST", "/api/tokens", true},
		{"read", "GET", "/api/torrents", true},
		{"read", "GET", "/api/torrents/contents", true},
		{"read", "GET", "/api/stats", true},
		{"read", "GET", "/api/log", true},
		{"read", "POST", "/api/torrents", false},
		{"read", "POST", "/api/torrents/batch", false},
		{"read", "GET", "/api/settings", false},
		{"read", "GET", "/api/hooks", false},
		{"read", "GET", "/api/tokens", false},
		{"control", "POST", "/api/torrents", true},
		{"control", "POST", "/api/torrents/batch", true},
		{"control", "DELETE", "/api/torrents/5", true},
		{"control", "PUT", "/api/stats/alt", false},
		{"control", "GET", "/api/stats", true},
		{"control", "PUT", "/api/settings", false},
		{"control", "POST", "/api/hooks/1/test", false},
		{"control", "GET", "/api/torrentsx", false},
		{"nope", "GET", "/api/torrents", false},
	}
	for _, tc := range tests {
		if got := Allowed(tc.scope, tc.method, tc.path); got != tc.want {
			t.Errorf("Allowed(%s, %s, %s) = %v, want %v", tc.scope, tc.method, tc.path, got, tc.want)
		}
	}
}

func TestHandler(t *testing.T) {
	svc, _, _ := newSvc(t)
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/tokens", "", 200},
		{"create", http.MethodPost, "/tokens", `{"name":"x","scope":"read","days":90}`, 201},
		{"create invalid", http.MethodPost, "/tokens", `{"name":""}`, 400},
		{"create bad json", http.MethodPost, "/tokens", `{`, 400},
		{"revoke", http.MethodDelete, "/tokens/1", "", 204},
		{"revoke missing", http.MethodDelete, "/tokens/1", "", 404},
		{"revoke bad id", http.MethodDelete, "/tokens/x", "", 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			if err != nil || resp.StatusCode != tc.want {
				t.Errorf("status = %v (%v), want %d", resp, err, tc.want)
			}
		})
	}
}
