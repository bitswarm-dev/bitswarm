package updates

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type bus struct{ events []string }

func (b *bus) Emit(ev string, _ map[string]any) { b.events = append(b.events, ev) }

func TestCompare(t *testing.T) {
	tests := []struct {
		a, b string
		sign int
	}{
		{"1.0.0", "0.9.0", 1}, {"0.9.0", "1.0.0", -1}, {"1.2.3", "1.2.3", 0}, {"1.10.0", "1.9.0", 1}, {"1.0", "1.0.0", 0}, {"2", "1.9.9", 1}, {"1.0.x", "1.0.0", 0},
	}
	for _, tc := range tests {
		got := Compare(tc.a, tc.b)
		if (got > 0) != (tc.sign > 0) || (got < 0) != (tc.sign < 0) {
			t.Errorf("Compare(%s, %s) = %d, want sign %d", tc.a, tc.b, got, tc.sign)
		}
	}
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	b := &bus{}
	svc := NewService("0.9.0", SampleFeed(), true, b)
	if st := svc.State(); st.State != "idle" || !st.Configured || !st.Sample {
		t.Fatalf("initial = %+v", st)
	}
	st := svc.Check(ctx)
	if st.State != "available" || st.Latest.Version != "1.0.0" || len(st.Latest.Notes) != 5 || st.Checked == 0 {
		t.Fatalf("available = %+v", st)
	}
	svc.Check(ctx)
	if len(b.events) != 1 || b.events[0] != "update.available" {
		t.Errorf("update.available should be emitted once per version: %v", b.events)
	}

	cur := NewService("1.0.0", SampleFeed(), true, b)
	if st := cur.Check(ctx); st.State != "current" {
		t.Errorf("current = %+v", st)
	}

	none := NewService("0.9.0", nil, false, b)
	if st := none.Check(ctx); st.State != "unconfigured" || st.Configured {
		t.Errorf("unconfigured = %+v", st)
	}

	bad := NewService("0.9.0", func(context.Context) ([]Release, error) { return nil, errors.New("offline") }, false, b)
	if st := bad.Check(ctx); st.State != "error" || st.Error != "offline" {
		t.Errorf("error = %+v", st)
	}
	empty := NewService("0.9.0", func(context.Context) ([]Release, error) { return nil, nil }, false, b)
	if st := empty.Check(ctx); st.State != "error" || st.Error == "" {
		t.Errorf("empty = %+v", st)
	}
	nilNotes := NewService("0.9.0", func(context.Context) ([]Release, error) { return []Release{{Version: "2.0.0"}}, nil }, false, b)
	if st := nilNotes.Check(ctx); st.Latest.Notes == nil {
		t.Error("notes should never be null")
	}
}

func TestURLFeed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte(`[{"version":"1.1.0","date":"2026-01-01","notes":["a"]}]`))
		case "/junk":
			_, _ = w.Write([]byte(`{"nope":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	if rs, err := URLFeed(srv.URL + "/ok")(ctx); err != nil || len(rs) != 1 || rs[0].Version != "1.1.0" {
		t.Errorf("ok feed = %+v, %v", rs, err)
	}
	if _, err := URLFeed(srv.URL + "/junk")(ctx); err == nil || !strings.Contains(err.Error(), "isn’t a list") {
		t.Errorf("junk err = %v", err)
	}
	if _, err := URLFeed(srv.URL + "/missing")(ctx); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404 err = %v", err)
	}
	if _, err := URLFeed("http://127.0.0.1:1/x")(ctx); err == nil {
		t.Error("unreachable feed should fail")
	}
	if _, err := URLFeed("::bad::")(ctx); err == nil {
		t.Error("bad URL should fail")
	}
}

func TestHandler(t *testing.T) {
	app := fiber.New()
	NewHandler(NewService("0.9.0", SampleFeed(), true, &bus{})).RegisterRoutes(app)
	for _, tc := range []struct{ method, path string }{{http.MethodGet, "/updates"}, {http.MethodPost, "/updates/check"}} {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, nil))
		if err != nil || resp.StatusCode != 200 {
			t.Errorf("%s %s = %v, %v", tc.method, tc.path, resp, err)
		}
	}
}
