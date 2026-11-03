package maintenance

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/catalog"
	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/settings"
	"bitswarm/server/internal/torrents"
)

type bus struct{ events []string }

func (b *bus) Emit(ev string, _ map[string]any) { b.events = append(b.events, ev) }

type nopLog struct{}

func (nopLog) Add(_, _, _ string) {}

type fixture struct {
	svc  *Service
	tor  *torrents.Service
	repo torrents.Repository
	bus  *bus
	now  time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	kv := docstore.NewKV(conn)
	cat, _ := catalog.NewService(kv)
	set, _ := settings.NewService(kv, &bus{}, nil)
	b := &bus{}
	repo := torrents.NewRepository(conn)
	tor := torrents.NewService(repo, set, cat, b, nopLog{})
	tasks, log := NewCollections(conn)
	f := &fixture{tor: tor, repo: repo, bus: b, now: time.Now()}
	f.svc = NewService(tasks, log, tor, b)
	f.svc.now = func() time.Time { return f.now }
	if err := f.svc.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *fixture) task(t *testing.T, typ string) Task {
	t.Helper()
	ts, _ := f.svc.tasks.List(context.Background())
	for _, x := range ts {
		if x.Type == typ {
			return x
		}
	}
	t.Fatalf("no %s task", typ)
	return Task{}
}

func (f *fixture) add(t *testing.T, mutate func(*torrents.Torrent), name string) torrents.Torrent {
	t.Helper()
	ctx := context.Background()
	v, err := f.tor.Add(ctx, torrents.AddSpec{Name: name, Size: 100 * torrents.MiB, Start: true, Tags: []string{"isos"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = f.tor.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			if ts[i].ID == v.ID {
				ts[i].Status = torrents.StatusDownloading
				mutate(&ts[i])
				v.Torrent = ts[i]
			}
		}
	})
	return v.Torrent
}

func (f *fixture) get(t *testing.T, id int64) torrents.Torrent {
	t.Helper()
	x, err := f.repo.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return x
}

func TestSeedAndList(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	v, err := f.svc.List(ctx)
	if err != nil || len(v) != 6 {
		t.Fatalf("List() = %d, %v", len(v), err)
	}
	if v[0].Title != "Retry torrents with errors" || !v[1].Scoped || v[3].Scoped || v[4].Enabled {
		t.Errorf("views wrong: %+v", v)
	}
	wantSummary := map[string]string{
		"retry": "Re-announce and resume every torrent with an error", "cleanup": "Finished 7+ days ago, ratio 1+ · keep files",
		"stalled": "No seeds, or no data for 6+ hours · pause it", "disk": "Pause below 20 GB free · resume when space frees up",
		"verify": "Re-check every piece of each finished torrent", "logs": "Entries older than 30 days",
	}
	for _, x := range v {
		if x.Summary != wantSummary[x.Type] {
			t.Errorf("%s summary = %q", x.Type, x.Summary)
		}
	}
	f.svc.tasks.Put(ctx, Task{ID: v[1].ID, Type: "cleanup", Every: 1, Unit: "days", P: Params{Days: 2, Files: "delete"}})
	if s := summary(f.task(t, "cleanup")); s != "Finished 2+ days ago · delete files" {
		t.Errorf("cleanup summary = %q", s)
	}
	if summary(Task{Type: "nope"}) != "" {
		t.Error("unknown type should have no summary")
	}
}

func TestRetry(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	bad := f.add(t, func(x *torrents.Torrent) { x.Status, x.Err, x.Seeds = torrents.StatusError, "timeout", 0 }, "bad")
	f.add(t, func(*torrents.Torrent) {}, "fine")
	retry := f.task(t, "retry")

	pv, _ := f.svc.Preview(ctx, retry.ID)
	if len(pv.Items) != 1 || pv.Items[0].Why != "timeout" || pv.Done != "No torrents had errors" {
		t.Errorf("preview = %+v", pv)
	}
	res, err := f.svc.Run(ctx, retry.ID)
	if err != nil || res.Count != 1 || res.Result != "Retried 1 torrent" {
		t.Fatalf("Run() = %+v, %v", res, err)
	}
	if got := f.get(t, bad.ID); got.Status != torrents.StatusDownloading || got.Err != "" || got.Seeds < 12 {
		t.Errorf("not retried: %+v", got)
	}
	if res, _ := f.svc.Run(ctx, retry.ID); res.Result != "No torrents had errors" {
		t.Errorf("second run = %+v", res)
	}
	if !slices.Contains(f.bus.events, "maintenance.run") || !slices.Contains(f.bus.events, "torrent.resumed") {
		t.Errorf("events = %v", f.bus.events)
	}
}

func TestCleanup(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	old := f.add(t, func(x *torrents.Torrent) {
		x.Done, x.Status, x.SeedSince, x.Uploaded = x.Size, torrents.StatusSeeding, f.now.Add(-10*24*time.Hour).UnixMilli(), x.Size*2
	}, "old")
	f.add(t, func(x *torrents.Torrent) {
		x.Done, x.Status, x.SeedSince, x.Uploaded = x.Size, torrents.StatusSeeding, f.now.Add(-10*24*time.Hour).UnixMilli(), 0
	}, "lowratio")
	f.add(t, func(x *torrents.Torrent) {
		x.Done, x.Status, x.SeedSince, x.Uploaded = x.Size, torrents.StatusSeeding, f.now.UnixMilli(), x.Size*2
	}, "recent")
	cleanup := f.task(t, "cleanup")

	pv, _ := f.svc.Preview(ctx, cleanup.ID)
	if len(pv.Items) != 1 || pv.Items[0].Name != "old" || pv.Items[0].What != "Remove, keep files" || !strings.Contains(pv.Items[0].Why, "10 days ago") {
		t.Fatalf("preview = %+v", pv)
	}
	res, _ := f.svc.Run(ctx, cleanup.ID)
	if res.Result != "Removed 1 finished torrent" {
		t.Errorf("result = %q", res.Result)
	}
	if _, err := f.repo.Get(ctx, old.ID); err == nil {
		t.Error("old torrent still there")
	}

	cleanup = f.task(t, "cleanup")
	cleanup.P.Files, cleanup.P.Ratio = "delete", 0
	_ = f.svc.tasks.Put(ctx, cleanup)
	if pv, _ := f.svc.Preview(ctx, cleanup.ID); len(pv.Items) != 1 || pv.Items[0].Name != "lowratio" || pv.Items[0].What != "Remove and delete files" {
		t.Errorf("delete preview = %+v", pv.Items)
	}
}

func TestStalled(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	noSeeds := f.add(t, func(x *torrents.Torrent) { x.Seeds = 0 }, "noseeds")
	idle := f.add(t, func(x *torrents.Torrent) { x.Seeds, x.LastActive = 5, f.now.Add(-8*time.Hour).UnixMilli() }, "idle")
	f.add(t, func(x *torrents.Torrent) { x.Seeds, x.LastActive = 5, f.now.UnixMilli() }, "active")
	st := f.task(t, "stalled")

	pv, _ := f.svc.Preview(ctx, st.ID)
	if len(pv.Items) != 2 || pv.Items[0].Why != "No seeds" || !strings.HasPrefix(pv.Items[1].Why, "No data for 8h") {
		t.Fatalf("preview = %+v", pv.Items)
	}
	if res, _ := f.svc.Run(ctx, st.ID); res.Result != "Handled 2 stalled downloads" {
		t.Errorf("result = %q", res.Result)
	}
	if f.get(t, noSeeds.ID).Status != torrents.StatusPaused {
		t.Error("pause action not applied")
	}

	st.P.Action = "reannounce"
	_ = f.svc.tasks.Put(ctx, st)
	_ = f.tor.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			if ts[i].ID == idle.ID {
				ts[i].Status = torrents.StatusDownloading
			}
		}
	})
	before := f.get(t, idle.ID).Seeds
	f.svc.Run(ctx, st.ID)
	if got := f.get(t, idle.ID); got.Seeds <= before {
		t.Errorf("reannounce did not add seeds: %d -> %d", before, got.Seeds)
	}

	st.P.Action = "remove"
	_ = f.svc.tasks.Put(ctx, st)
	_ = f.tor.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			if ts[i].ID == idle.ID {
				ts[i].Status, ts[i].LastActive = torrents.StatusDownloading, 0
			}
		}
	})
	f.svc.Run(ctx, st.ID)
	if _, err := f.repo.Get(ctx, idle.ID); err == nil {
		t.Error("remove action did not remove")
	}
}

func TestDisk(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	dl := f.add(t, func(x *torrents.Torrent) { x.Status = torrents.StatusDownloading }, "dl")
	disk := f.task(t, "disk")

	if pv, _ := f.svc.Preview(ctx, disk.ID); len(pv.Items) != 0 || !strings.HasSuffix(pv.Done, "free, nothing to do") {
		t.Errorf("plenty of space = %+v", pv)
	}
	disk.P.GB = 100000
	_ = f.svc.tasks.Put(ctx, disk)
	res, _ := f.svc.Run(ctx, disk.ID)
	if !strings.HasPrefix(res.Result, "Paused 1 download, only") {
		t.Errorf("result = %q", res.Result)
	}
	if got := f.get(t, dl.ID); got.Status != torrents.StatusPaused || got.PausedBy != disk.ID {
		t.Errorf("not paused by task: %+v", got.Status)
	}

	disk.P.GB = 1
	_ = f.svc.tasks.Put(ctx, disk)
	res, _ = f.svc.Run(ctx, disk.ID)
	if !strings.HasPrefix(res.Result, "Resumed 1 download,") {
		t.Errorf("resume result = %q", res.Result)
	}
	if got := f.get(t, dl.ID); got.Status != torrents.StatusDownloading || got.PausedBy != 0 {
		t.Errorf("not resumed: %+v", got.Status)
	}
	disk.P.Resume = false
	disk.P.GB = 100000
	_ = f.svc.tasks.Put(ctx, disk)
	f.svc.Run(ctx, disk.ID)
	disk.P.GB = 1
	_ = f.svc.tasks.Put(ctx, disk)
	if pv, _ := f.svc.Preview(ctx, disk.ID); len(pv.Items) != 0 {
		t.Errorf("resume disabled should not resume: %+v", pv.Items)
	}
}

func TestVerifyAndLogs(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.add(t, func(x *torrents.Torrent) { x.Done, x.Status = x.Size, torrents.StatusSeeding }, "finished")
	f.add(t, func(*torrents.Torrent) {}, "unfinished")
	verify := f.task(t, "verify")
	if pv, _ := f.svc.Preview(ctx, verify.ID); len(pv.Items) != 1 || pv.Items[0].Name != "finished" {
		t.Errorf("verify preview = %+v", pv.Items)
	}
	if res, _ := f.svc.Run(ctx, verify.ID); res.Result != "Verified 1 torrent, all pieces match" {
		t.Errorf("verify result = %q", res.Result)
	}

	old := f.svc.nowMs() - 40*day
	_, _ = f.svc.log.Create(ctx, LogEntry{T: old, Task: "ancient"})
	other := []OldEntry{{ID: 5, Label: "x", T: old}}
	removed := int64(0)
	f.svc.AddPruner(AdaptPruner("Schedules", func(int64) []OldEntry {
		if removed != 0 {
			return nil
		}
		return other
	}, func(id int64) { removed = id }, func(e OldEntry) OldEntry { return e }))
	f.svc.AddPruner(f.svc)
	logs := f.task(t, "logs")
	pv, _ := f.svc.Preview(ctx, logs.ID)
	if len(pv.Items) != 2 || pv.Items[0].Name != "Schedules: x" || pv.Items[1].Name != "Maintenance: ancient" {
		t.Fatalf("logs preview = %+v", pv.Items)
	}
	res, _ := f.svc.Run(ctx, logs.ID)
	if res.Result != "Deleted 2 old entries" || removed != 5 {
		t.Errorf("logs result = %q removed %d", res.Result, removed)
	}
	if res, _ := f.svc.Run(ctx, logs.ID); res.Result != "No old entries" {
		t.Errorf("rerun = %q", res.Result)
	}
	if s := f.svc.Source(); s != "Maintenance" {
		t.Errorf("Source() = %q", s)
	}
	entries, _ := f.svc.Log(ctx)
	if len(entries) == 0 || entries[0].Added == nil {
		t.Errorf("log = %+v", entries)
	}
}

func TestUpdateEnableErrors(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	cleanup := f.task(t, "cleanup")

	tests := []struct {
		name    string
		mutate  func(*Task)
		wantErr string
	}{
		{"ok", func(t *Task) { t.P.Days = 3 }, ""},
		{"every zero", func(t *Task) { t.Every = 0 }, "“Run every”"},
		{"bad unit", func(t *Task) { t.Unit = "weeks" }, "minutes, hours or days"},
		{"days range", func(t *Task) { t.P.Days = -1 }, "“Finished at least”"},
		{"ratio range", func(t *Task) { t.P.Ratio = -2 }, "share ratio"},
		{"files option", func(t *Task) { t.P.Files = "x" }, "keep or delete"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := cleanup
			tc.mutate(&in)
			_, err := f.svc.Update(ctx, in)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}

	for typ, mutate := range map[string]func(*Task){
		"stalled": func(t *Task) { t.P.Hours = 0 },
		"disk":    func(t *Task) { t.P.GB = 0 },
		"logs":    func(t *Task) { t.P.Days = 99999 },
	} {
		in := f.task(t, typ)
		mutate(&in)
		if _, err := f.svc.Update(ctx, in); err == nil {
			t.Errorf("%s: invalid update accepted", typ)
		}
	}
	st := f.task(t, "stalled")
	st.P.Action = "dance"
	if _, err := f.svc.Update(ctx, st); err == nil {
		t.Error("bad stalled action accepted")
	}

	disk := f.task(t, "disk")
	disk.Tag = "isos"
	upd, err := f.svc.Update(ctx, disk)
	if err != nil || upd.Tag != "" {
		t.Errorf("unscoped task kept a tag: %+v, %v", upd.Tag, err)
	}
	cleanup.Every, cleanup.Unit = 3, "days"
	cleanup.Tag = "isos"
	upd, _ = f.svc.Update(ctx, cleanup)
	if upd.NextRun != f.svc.nowMs()+3*day {
		t.Errorf("NextRun not rescheduled: %d", upd.NextRun)
	}
	if _, err := f.svc.Update(ctx, Task{ID: 99}); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
	if v, err := f.svc.SetEnabled(ctx, cleanup.ID, false); err != nil || v.Enabled {
		t.Errorf("SetEnabled(false) = %+v, %v", v.Enabled, err)
	}
	if v, _ := f.svc.SetEnabled(ctx, cleanup.ID, true); v.NextRun <= f.svc.nowMs() {
		t.Error("enabling should reschedule")
	}
	if _, err := f.svc.SetEnabled(ctx, 99, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
	if _, err := f.svc.Run(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("run missing err = %v", err)
	}
	if _, err := f.svc.Preview(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("preview missing err = %v", err)
	}
}

func TestTick(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.svc.Tick(ctx)
	if f.task(t, "retry").LastRun != 0 {
		t.Error("ran before due")
	}
	f.now = f.now.Add(2 * time.Hour)
	f.svc.Tick(ctx)
	if f.task(t, "retry").LastRun == 0 || f.task(t, "verify").LastRun != 0 {
		t.Error("due enabled task should run, disabled should not")
	}
}

func TestHelpers(t *testing.T) {
	if got := plural(1, "x") + plural(2, "x"); got != "1 x2 xs" {
		t.Errorf("plural = %q", got)
	}
	if lower("ABC") != "aBC" || lower("") != "" {
		t.Error("lower wrong")
	}
	for in, want := range map[float64]string{30: "30s", 600: "10m", 7500: "2h 05m", 200000: "2d 7h"} {
		if got := torrentsEta(in); got != want {
			t.Errorf("torrentsEta(%v) = %q, want %q", in, got, want)
		}
	}
	if unitMs("minutes") != minute || unitMs("days") != day || unitMs("hours") != hour {
		t.Error("unitMs wrong")
	}
	if completedAt(torrents.Torrent{}) != 0 || completedAt(torrents.Torrent{SeedSince: 5}) != 5 {
		t.Error("completedAt wrong")
	}
}

func TestHandler(t *testing.T) {
	f := newFixture(t)
	app := fiber.New()
	NewHandler(f.svc).RegisterRoutes(app)
	body := `{"every":2,"unit":"days","tag":"","p":{"days":3,"ratio":0,"files":"keep"}}`
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/maintenance", "", 200},
		{"log", http.MethodGet, "/maintenance/log", "", 200},
		{"update", http.MethodPut, "/maintenance/2", body, 200},
		{"update invalid", http.MethodPut, "/maintenance/2", `{"every":0,"unit":"days","p":{}}`, 400},
		{"update missing", http.MethodPut, "/maintenance/99", body, 404},
		{"update bad id", http.MethodPut, "/maintenance/x", body, 400},
		{"update bad json", http.MethodPut, "/maintenance/2", `{`, 400},
		{"enable", http.MethodPatch, "/maintenance/2/enabled", `{"enabled":false}`, 200},
		{"enable bad json", http.MethodPatch, "/maintenance/2/enabled", `{`, 400},
		{"enable bad id", http.MethodPatch, "/maintenance/x/enabled", `{}`, 400},
		{"preview", http.MethodGet, "/maintenance/1/preview", "", 200},
		{"preview bad id", http.MethodGet, "/maintenance/x/preview", "", 400},
		{"preview missing", http.MethodGet, "/maintenance/99/preview", "", 404},
		{"run", http.MethodPost, "/maintenance/1/run", "", 200},
		{"run bad id", http.MethodPost, "/maintenance/x/run", "", 400},
		{"run missing", http.MethodPost, "/maintenance/99/run", "", 404},
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
