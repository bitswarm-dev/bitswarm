package renames

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
	"bitswarm/server/internal/torrents"
)

type fakeStore struct {
	mu sync.Mutex
	ts []torrents.Torrent
}

func (f *fakeStore) Snapshot(context.Context) ([]torrents.Torrent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]torrents.Torrent(nil), f.ts...), nil
}

func (f *fakeStore) Mutate(_ context.Context, fn func([]torrents.Torrent)) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f.ts)
	return nil
}

type bus struct{ events []string }

func (b *bus) Emit(ev string, _ map[string]any) { b.events = append(b.events, ev) }

func newSvc(t *testing.T) (*Service, *fakeStore, *bus) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	p, l := NewCollections(conn)
	store := &fakeStore{ts: []torrents.Torrent{
		{ID: 1, Name: "ubuntu-26.04-desktop-amd64.iso", Tags: []string{"keep"}, Files: []torrents.File{{Name: "ubuntu-26.04-desktop-amd64.iso", Size: 1}}},
		{ID: 2, Name: "Sintel", Tags: []string{"4K"}, Files: []torrents.File{{Name: "Sintel.2010.4k.mkv"}, {Name: "poster.jpg"}}},
	}}
	b := &bus{}
	svc := NewService(p, l, store, b)
	if err := svc.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc, store, b
}

func TestSeedAndList(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	if err := svc.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	views, err := svc.List(ctx)
	if err != nil || len(views) != 3 {
		t.Fatalf("List() = %d, %v", len(views), err)
	}
	if views[0].Example == nil || views[0].Example.To != "Ubuntu 26.04 Desktop (amd64).iso" {
		t.Errorf("example = %+v", views[0].Example)
	}
	if views[2].Example != nil && views[2].Tag != "" {
		t.Error("unexpected example")
	}
	svc.SetUsers(func(id int64) []string { return []string{"S"} })
	if v, _ := svc.List(ctx); v[0].UsedBy[0] != "S" {
		t.Error("usedBy not wired")
	}
}

func TestCreateUpdateValidation(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	good := Preset{Name: "N", Find: "a", Replace: "b", Target: "files", When: "add", KeepExt: true}
	tests := []struct {
		name    string
		mutate  func(*Preset)
		wantErr string
	}{
		{"ok", func(*Preset) {}, ""},
		{"no name", func(p *Preset) { p.Name = " " }, "Give the preset a name."},
		{"no pattern", func(p *Preset) { p.Find = "" }, "Enter a pattern to find."},
		{"bad regex", func(p *Preset) { p.Find = "(" }, "isn’t a valid regular expression"},
		{"bad target", func(p *Preset) { p.Target = "x" }, "Choose what to rename"},
		{"bad when", func(p *Preset) { p.When = "x" }, "Choose when"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := good
			tc.mutate(&p)
			_, err := svc.Create(ctx, p)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
	created, _ := svc.Create(ctx, good)
	created.Name = "Renamed"
	if out, err := svc.Update(ctx, created); err != nil || out.Name != "Renamed" {
		t.Errorf("Update() = %+v, %v", out, err)
	}
	created.ID = 99
	if _, err := svc.Update(ctx, created); err == nil {
		t.Error("updating a missing preset should fail")
	}
	if p, err := svc.SetEnabled(ctx, 1, false); err != nil || p.Enabled {
		t.Errorf("SetEnabled() = %+v, %v", p, err)
	}
	if dup, err := svc.Duplicate(ctx, 1); err != nil || !strings.HasSuffix(dup.Name, " copy") || dup.Enabled {
		t.Errorf("Duplicate() = %+v, %v", dup, err)
	}
	if _, err := svc.Duplicate(ctx, 99); err == nil {
		t.Error("duplicating a missing preset should fail")
	}
}

func TestApplyRenamesOnAddAndComplete(t *testing.T) {
	svc, _, b := newSvc(t)
	ctx := context.Background()

	tor := torrents.Torrent{ID: 1, Name: "ubuntu-26.04-desktop-amd64.iso", Tags: []string{"keep"}, Files: []torrents.File{{Name: "ubuntu-26.04-desktop-amd64.iso"}}}
	svc.ApplyRenames(ctx, &tor, "add")
	if tor.Name != "ubuntu-26.04-desktop-amd64.iso" {
		t.Error("a completion preset ran on add")
	}
	svc.ApplyRenames(ctx, &tor, "complete")
	if tor.Name != "Ubuntu 26.04 Desktop (amd64).iso" || tor.Files[0].Name != tor.Name || tor.Files[0].Orig != "ubuntu-26.04-desktop-amd64.iso" {
		t.Errorf("after complete: %+v", tor)
	}
	if len(b.events) != 1 || b.events[0] != "torrent.renamed" {
		t.Errorf("events = %v", b.events)
	}

	other := torrents.Torrent{ID: 3, Name: "Sintel.2010.mkv", Tags: []string{"software"}, Files: []torrents.File{{Name: "Sintel.2010.mkv"}}}
	svc.ApplyRenames(ctx, &other, "complete")
	if other.Name != "Sintel.2010.mkv" {
		t.Error("tag-scoped preset ran on a torrent without the tag")
	}
	chosen := torrents.Torrent{ID: 4, Name: "Sintel.2010.mkv", Tags: []string{"software"}, Presets: []int64{2}, Files: []torrents.File{{Name: "Sintel.2010.mkv"}}}
	svc.ApplyRenames(ctx, &chosen, "complete")
	if chosen.Name != "Sintel (2010).mkv" {
		t.Errorf("explicitly chosen preset ignored: %q", chosen.Name)
	}
	if v, _ := svc.List(ctx); v[0].Count != 1 || v[1].Count != 1 {
		t.Errorf("counts = %d, %d", v[0].Count, v[1].Count)
	}
}

func TestApplyAndUndo(t *testing.T) {
	svc, store, _ := newSvc(t)
	ctx := context.Background()

	res, err := svc.Apply(ctx, 2, nil)
	if err != nil || res.Names != 1 || res.Torrents != 1 {
		t.Fatalf("Apply() = %+v, %v", res, err)
	}
	if store.ts[1].Files[0].Name != "Sintel (2010).mkv" {
		t.Fatalf("file = %q", store.ts[1].Files[0].Name)
	}
	if res, _ := svc.Apply(ctx, 2, []int64{1}); res.Names != 0 {
		t.Errorf("selection without matches renamed %d", res.Names)
	}
	if _, err := svc.Apply(ctx, 99, nil); err == nil {
		t.Error("applying a missing preset should fail")
	}

	log, _ := svc.Log(ctx)
	if len(log) != 1 || log[0].Undone {
		t.Fatalf("log = %+v", log)
	}
	if err := svc.Undo(ctx, log[0].ID); err != nil {
		t.Fatal(err)
	}
	if store.ts[1].Files[0].Name != "Sintel.2010.4k.mkv" {
		t.Errorf("undo left %q", store.ts[1].Files[0].Name)
	}
	if err := svc.Undo(ctx, log[0].ID); err != nil {
		t.Error("undoing twice should be harmless")
	}
	if err := svc.Undo(ctx, 99); err == nil {
		t.Error("undoing a missing entry should fail")
	}
}

func TestApplyIntoFolder(t *testing.T) {
	svc, store, _ := newSvc(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, Preset{Name: "Folder", Find: `^(Sintel)\.(\d+)\.4k$`, Replace: "$1/$1 ($2)", Target: "files", KeepExt: true, When: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := svc.Apply(ctx, p.ID, nil); err != nil || res.Names != 1 {
		t.Fatalf("Apply() = %+v, %v", res, err)
	}
	if got := store.ts[1].Files[0].Name; got != "Sintel/Sintel (2010).mkv" {
		t.Fatalf("file = %q", got)
	}
	log, _ := svc.Log(ctx)
	if err := svc.Undo(ctx, log[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := store.ts[1].Files[0].Name; got != "Sintel.2010.4k.mkv" {
		t.Errorf("undo left %q", got)
	}
}

func TestDelete(t *testing.T) {
	svc, store, _ := newSvc(t)
	ctx := context.Background()
	store.ts[0].Presets = []int64{1, 3}
	gone := int64(0)
	svc.OnDelete(func(id int64) { gone = id })
	if err := svc.Delete(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if gone != 1 || len(store.ts[0].Presets) != 1 || store.ts[0].Presets[0] != 3 {
		t.Errorf("gone=%d presets=%v", gone, store.ts[0].Presets)
	}
	if err := svc.Delete(ctx, 1); err == nil {
		t.Error("deleting twice should fail")
	}
}

func TestPreview(t *testing.T) {
	svc, _, _ := newSvc(t)
	ctx := context.Background()
	if p, _ := svc.Preview(ctx, PreviewReq{}); len(p.Rows) != 0 || p.Error != "" {
		t.Errorf("blank preview = %+v", p)
	}
	if p, _ := svc.Preview(ctx, PreviewReq{Preset: Preset{Find: "("}}); p.Error == "" {
		t.Error("invalid pattern should be reported")
	}
	p, err := svc.Preview(ctx, PreviewReq{Preset: Preset{Find: `^(.+)\.old$`, Replace: "$1.new", Target: "files", Tag: "software"}, Samples: "a.old\n\n b.txt "})
	if err != nil || p.Total != 2 || p.OK != 1 || len(p.Rows) != 2 || p.Rows[0].Src != "Your names" || !p.Rows[0].Match || p.Rows[1].Match || !p.Rows[1].Same {
		t.Errorf("Preview() = %+v, %v", p, err)
	}
	p, _ = svc.Preview(ctx, PreviewReq{Preset: Preset{Find: `.+`, Replace: "x", Target: "both"}})
	if p.Total != 5 || p.OK == 0 {
		t.Errorf("both-target preview = %+v", p)
	}
}

func TestHandler(t *testing.T) {
	svc, _, _ := newSvc(t)
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)
	body := `{"name":"X","find":"a","replace":"b","target":"files","when":"add"}`
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/renames", "", 200},
		{"create", http.MethodPost, "/renames", body, 201},
		{"create invalid", http.MethodPost, "/renames", `{"name":"X"}`, 400},
		{"create bad json", http.MethodPost, "/renames", `{`, 400},
		{"update", http.MethodPut, "/renames/4", body, 200},
		{"update missing", http.MethodPut, "/renames/99", body, 404},
		{"update bad id", http.MethodPut, "/renames/x", body, 400},
		{"update bad json", http.MethodPut, "/renames/4", `{`, 400},
		{"enable", http.MethodPatch, "/renames/4/enabled", `{"enabled":false}`, 200},
		{"enable bad json", http.MethodPatch, "/renames/4/enabled", `{`, 400},
		{"enable bad id", http.MethodPatch, "/renames/x/enabled", `{}`, 400},
		{"duplicate", http.MethodPost, "/renames/4/duplicate", "", 201},
		{"duplicate bad id", http.MethodPost, "/renames/x/duplicate", "", 400},
		{"apply all", http.MethodPost, "/renames/2/apply", "", 200},
		{"apply selection", http.MethodPost, "/renames/2/apply", `{"ids":[2]}`, 200},
		{"apply bad json", http.MethodPost, "/renames/2/apply", `{`, 400},
		{"apply missing", http.MethodPost, "/renames/99/apply", "", 404},
		{"apply bad id", http.MethodPost, "/renames/x/apply", "", 400},
		{"preview", http.MethodPost, "/renames/preview", `{"find":"a","target":"files"}`, 200},
		{"preview bad json", http.MethodPost, "/renames/preview", `{`, 400},
		{"log", http.MethodGet, "/renames/log", "", 200},
		{"undo missing", http.MethodPost, "/renames/log/99/undo", "", 404},
		{"undo bad id", http.MethodPost, "/renames/log/x/undo", "", 400},
		{"delete", http.MethodDelete, "/renames/4", "", 204},
		{"delete missing", http.MethodDelete, "/renames/4", "", 404},
		{"delete bad id", http.MethodDelete, "/renames/x", "", 400},
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
