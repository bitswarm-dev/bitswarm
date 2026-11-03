package eventlog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
)

func newSvc(t *testing.T) *Service {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewService(NewCollection(conn))
}

func TestDescribe(t *testing.T) {
	torrent := map[string]any{"name": "Ubuntu", "tags": []string{"a", "b"}}
	tests := []struct {
		name      string
		event     string
		data      map[string]any
		wantLevel string
		wantMsg   string
	}{
		{"added", "torrent.added", map[string]any{"torrent": torrent, "files_total": 4, "files_selected": 2}, "info", "Added Ubuntu, 2 of 4 files"},
		{"added from schedule", "torrent.added", map[string]any{"torrent": torrent, "source": "schedule", "schedule": map[string]any{"name": "S"}}, "info", "Added Ubuntu from schedule “S”"},
		{"paused with reason", "torrent.paused", map[string]any{"torrent": torrent, "reason": "ratio_limit"}, "info", "Paused Ubuntu (ratio limit)"},
		{"removed with data", "torrent.removed", map[string]any{"torrent": torrent, "deleted_files": true}, "warn", "Removed Ubuntu and deleted its files"},
		{"error", "torrent.error", map[string]any{"torrent": torrent, "error": "boom"}, "error", "Ubuntu: boom"},
		{"tagged", "torrent.tagged", map[string]any{"torrent": torrent}, "info", "Tags of Ubuntu are now: a, b"},
		{"schedule failed", "schedule.finished", map[string]any{"schedule": map[string]any{"name": "S"}, "status": "error", "error": "bad"}, "error", "Schedule “S” failed: bad"},
		{"schedule ok", "schedule.finished", map[string]any{"schedule": map[string]any{"name": "S"}, "status": "ok", "count": 1}, "info", "Schedule “S” finished and added 1 torrent"},
		{"settings", "settings.saved", map[string]any{"changed": []string{"port"}}, "info", "Settings saved. Changed: port"},
		{"unknown", "x.y", nil, "info", "x.y"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			level, msg := Describe(tc.event, tc.data)
			if level != tc.wantLevel || msg != tc.wantMsg {
				t.Errorf("Describe() = %q, %q; want %q, %q", level, msg, tc.wantLevel, tc.wantMsg)
			}
		})
	}
}

func TestSourceOf(t *testing.T) {
	for ev, want := range map[string]string{"torrent.added": "Torrents", "schedule.search": "Schedules", "maintenance.run": "Maintenance", "settings.saved": "Settings", "update.available": "Updates", "other": "App"} {
		if got := SourceOf(ev); got != want {
			t.Errorf("SourceOf(%q) = %q, want %q", ev, got, want)
		}
	}
}

func TestService_QueryAndClear(t *testing.T) {
	svc := newSvc(t)
	ctx := context.Background()
	svc.Add("info", "App", "started")
	svc.Add("error", "Torrents", "broke")
	svc.Addf("debug", "Hooks", "sent %d", 1)
	svc.Emit("torrent.completed", map[string]any{"torrent": map[string]any{"name": "X"}})

	res, _ := svc.Query(ctx, Filter{Levels: map[string]bool{"error": true, "info": true}})
	if res.Total != 3 || res.Entries[0].Msg != "Finished downloading X" {
		t.Errorf("filtered = %+v", res)
	}
	if res.Counts["debug"] != 1 || len(res.Sources) != 3 {
		t.Errorf("counts/sources = %v %v", res.Counts, res.Sources)
	}
	if r, _ := svc.Query(ctx, Filter{Src: "Torrents", Q: "BROKE"}); r.Total != 1 {
		t.Errorf("src+q total = %d", r.Total)
	}
	if r, _ := svc.Query(ctx, Filter{Limit: 1}); len(r.Entries) != 1 || r.Total != 4 {
		t.Errorf("limit = %d entries, total %d", len(r.Entries), r.Total)
	}
	if svc.Count(ctx) != 4 {
		t.Errorf("Count() = %d", svc.Count(ctx))
	}
	if err := svc.Clear(ctx); err != nil || svc.Count(ctx) != 0 {
		t.Errorf("Clear() err = %v", err)
	}
}

func TestHandler(t *testing.T) {
	svc := newSvc(t)
	svc.Add("info", "App", "hello")
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)

	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/log?levels=info,error&src=App&q=hel&limit=5", 200},
		{http.MethodDelete, "/log", 204},
	} {
		resp, err := app.Test(httptest.NewRequest(tc.method, tc.path, strings.NewReader("")))
		if err != nil || resp.StatusCode != tc.want {
			t.Errorf("%s %s = %v, %v; want %d", tc.method, tc.path, resp, err, tc.want)
		}
	}
}
