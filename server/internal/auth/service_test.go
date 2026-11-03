package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
)

type users struct {
	name   string
	bypass bool
}

func (u *users) Username() string  { return u.name }
func (u *users) LocalBypass() bool { return u.bypass }

type memLog struct{ lines []string }

func (m *memLog) Add(_, _, msg string) { m.lines = append(m.lines, msg) }

func newSvc(t *testing.T) (*Service, *users, *memLog) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	u, l := &users{name: "admin"}, &memLog{}
	svc, err := NewService(docstore.NewKV(conn), u, l)
	if err != nil {
		t.Fatal(err)
	}
	return svc, u, l
}

func TestLogin_DemoPasswordAndLockout(t *testing.T) {
	svc, _, log := newSvc(t)
	if !svc.UsingDemoPassword() {
		t.Fatal("fresh install should use the demo password")
	}
	now := time.Now()
	svc.now = func() time.Time { return now }

	for i := 1; i <= 4; i++ {
		res := svc.Login("admin", "wrong")
		if res.OK || res.TriesLeft != 5-i || !strings.Contains(res.Message, "Wrong username or password") {
			t.Fatalf("try %d = %+v", i, res)
		}
	}
	if res := svc.Login("admin", "wrong"); res.LockedFor != lockFor {
		t.Fatalf("5th failure = %+v", res)
	}
	if res := svc.Login("admin", "swarm"); res.OK || res.LockedFor <= 0 {
		t.Errorf("correct password during lockout = %+v", res)
	}
	now = now.Add(31 * time.Second)
	res := svc.Login("admin", "swarm")
	if !res.OK || !svc.Valid(res.Token) {
		t.Fatalf("after lockout = %+v", res)
	}
	svc.Logout(res.Token)
	if svc.Valid(res.Token) || svc.Valid("") {
		t.Error("token still valid after logout")
	}
	if len(log.lines) < 6 {
		t.Errorf("log = %v", log.lines)
	}
	if tries(1) != "1 try" || tries(4) != "4 tries" {
		t.Error("tries() wrong")
	}
	if res := svc.Login("someone", "swarm"); res.OK {
		t.Error("wrong user should fail")
	}
}

func TestSessionExpiry(t *testing.T) {
	svc, _, _ := newSvc(t)
	now := time.Now()
	svc.now = func() time.Time { return now }
	res := svc.Login("admin", "swarm")
	now = now.Add(SessionTTL + time.Hour)
	if svc.Valid(res.Token) {
		t.Error("expired session still valid")
	}
}

func TestSetPassword(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.SetPassword(ctx, "abc"); err != ErrShortPassword {
		t.Errorf("short password err = %v", err)
	}
	if err := svc.SetPassword(ctx, "hunter22"); err != nil {
		t.Fatal(err)
	}
	if svc.UsingDemoPassword() {
		t.Error("still demo after change")
	}
	if svc.Login("admin", "swarm").OK {
		t.Error("old password should stop working")
	}
	if !svc.Login("admin", "hunter22").OK {
		t.Error("new password should work")
	}
	if err := svc.ResetPassword(ctx); err != nil || !svc.UsingDemoPassword() {
		t.Errorf("reset err = %v", err)
	}
	if svc.Username() != "admin" || svc.LocalBypass() {
		t.Error("user source not wired")
	}
}

func TestPersistsAcrossRestart(t *testing.T) {
	conn, _ := db.Open(":memory:")
	defer conn.Close()
	kv := docstore.NewKV(conn)
	a, _ := NewService(kv, &users{name: "admin"}, &memLog{})
	a.SetPassword(context.Background(), "persisted1")
	b, _ := NewService(kv, &users{name: "admin"}, &memLog{})
	if !b.Login("admin", "persisted1").OK {
		t.Error("password not loaded from storage")
	}
}

func TestHandler(t *testing.T) {
	svc, u, _ := newSvc(t)
	h := NewHandler(svc)
	app := fiber.New()
	api := app.Group("/api")
	h.RegisterPublic(api)
	api.Use(h.Require())
	api.Get("/secret", func(c *fiber.Ctx) error { return c.SendString("ok") })

	do := func(method, path, body, cookie string) *http.Response {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}

	if r := do("GET", "/api/secret", "", ""); r.StatusCode != 401 {
		t.Errorf("secret without session = %d", r.StatusCode)
	}
	if r := do("POST", "/api/auth/login", `{`, ""); r.StatusCode != 400 {
		t.Errorf("bad json = %d", r.StatusCode)
	}
	if r := do("POST", "/api/auth/login", `{"user":"admin","pass":"nope"}`, ""); r.StatusCode != 401 {
		t.Errorf("wrong password = %d", r.StatusCode)
	}
	r := do("POST", "/api/auth/login", `{"user":"admin","pass":"swarm","keep":true}`, "")
	if r.StatusCode != 200 || len(r.Cookies()) == 0 {
		t.Fatalf("login = %d, cookies %v", r.StatusCode, r.Cookies())
	}
	cookie := r.Cookies()[0].Name + "=" + r.Cookies()[0].Value
	if r := do("GET", "/api/secret", "", cookie); r.StatusCode != 200 {
		t.Errorf("secret with session = %d", r.StatusCode)
	}
	if r := do("GET", "/api/auth/status", "", cookie); r.StatusCode != 200 {
		t.Errorf("status = %d", r.StatusCode)
	}
	for i := 0; i < 5; i++ {
		r = do("POST", "/api/auth/login", `{"user":"admin","pass":"nope"}`, "")
	}
	if r.StatusCode != 429 {
		t.Errorf("lockout = %d", r.StatusCode)
	}
	if r := do("POST", "/api/auth/logout", "", cookie); r.StatusCode != 204 {
		t.Errorf("logout = %d", r.StatusCode)
	}
	if r := do("GET", "/api/secret", "", cookie); r.StatusCode != 401 {
		t.Errorf("secret after logout = %d", r.StatusCode)
	}
	u.bypass = true
	if r := do("GET", "/api/secret", "", ""); r.StatusCode != 200 && r.StatusCode != 401 {
		t.Errorf("bypass = %d", r.StatusCode)
	}
}

func TestHandler_BearerTokens(t *testing.T) {
	svc, u, _ := newSvc(t)
	u.bypass = false
	h := NewHandler(svc)
	h.SetTokens(func(_ context.Context, raw string) (string, bool) {
		switch raw {
		case "swm_read":
			return "read", true
		case "swm_admin":
			return "admin", true
		}
		return "", false
	}, func(scope, method, path string) bool {
		return scope == "admin" || (method == "GET" && path == "/api/torrents")
	})

	app := fiber.New()
	api := app.Group("/api")
	api.Use(h.Require())
	api.Get("/torrents", func(c *fiber.Ctx) error { return c.SendString("ok") })
	api.Post("/settings", func(c *fiber.Ctx) error { return c.SendString("ok") })

	tests := []struct {
		name, method, path, auth string
		want                     int
	}{
		{"no credentials", "GET", "/api/torrents", "", 401},
		{"valid read token on allowed route", "GET", "/api/torrents", "Bearer swm_read", 200},
		{"read token on forbidden route", "POST", "/api/settings", "Bearer swm_read", 403},
		{"admin token anywhere", "POST", "/api/settings", "Bearer swm_admin", 200},
		{"unknown token", "GET", "/api/torrents", "Bearer swm_nope", 401},
		{"not a bearer header", "GET", "/api/torrents", "Basic abc", 401},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			resp, err := app.Test(req)
			if err != nil || resp.StatusCode != tc.want {
				t.Errorf("status = %v (%v), want %d", resp, err, tc.want)
			}
		})
	}

	plain := NewHandler(svc)
	app2 := fiber.New()
	app2.Use(plain.Require())
	app2.Get("/", func(c *fiber.Ctx) error { return nil })
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer swm_read")
	if resp, _ := app2.Test(req); resp.StatusCode != 401 {
		t.Errorf("without a verifier a bearer token must not work: %d", resp.StatusCode)
	}
}
