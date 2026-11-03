package backup

import (
	"context"
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
	"bitswarm/server/internal/hooks"
	"bitswarm/server/internal/maintenance"
	"bitswarm/server/internal/renames"
	"bitswarm/server/internal/schedules"
	"bitswarm/server/internal/settings"
	"bitswarm/server/internal/torrents"
)

type bus struct{}

func (bus) Emit(string, map[string]any) {}

type nopLog struct{ lines []string }

func (l *nopLog) Add(_, _, msg string) { l.lines = append(l.lines, msg) }

type env struct {
	svc   *Service
	log   *nopLog
	set   *settings.Service
	cat   *catalog.Service
	ren   *renames.Service
	sch   *schedules.Service
	maint *maintenance.Service
	hk    *hooks.Service
}

func newEnv(t *testing.T) *env {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	kv := docstore.NewKV(conn)
	log := &nopLog{}
	set, _ := settings.NewService(kv, bus{}, nil)
	cat, _ := catalog.NewService(kv)
	tor := torrents.NewService(torrents.NewRepository(conn), set, cat, bus{}, log)
	rp, rl := renames.NewCollections(conn)
	ren := renames.NewService(rp, rl, tor, bus{})
	sr, sl := schedules.NewCollections(conn)
	sch := schedules.NewService(sr, sl, tor, bus{})
	mt, ml := maintenance.NewCollections(conn)
	maint := maintenance.NewService(mt, ml, tor, bus{})
	if err := maint.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	hh, hd := hooks.NewCollections(conn)
	hk := hooks.NewService(hh, hd, log, false)

	svc := NewService()
	svc.Settings, svc.Catalog, svc.Renames, svc.Schedules, svc.Maintenance, svc.Hooks, svc.Log = set, cat, ren, sch, maint, hk, log
	svc.now = func() time.Time { return time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC) }
	return &env{svc: svc, log: log, set: set, cat: cat, ren: ren, sch: sch, maint: maint, hk: hk}
}

func (e *env) populate(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	e.cat.EnsureTags([]string{"keep"})
	p, err := e.ren.Create(ctx, renames.Preset{Name: "Tidy", Find: `^a$`, Replace: "b", KeepExt: true, Target: "files", Tag: "keep", When: "add", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(48 * time.Hour).UnixMilli()
	if _, err := e.sch.Create(ctx, schedules.Rule{Name: "Ubuntu", URL: "https://example.org/s?q=x", Selector: "a.m", Attr: "href", LinkType: "magnet",
		Tags: []string{"keep"}, Path: t.TempDir(), Max: 3, Start: true, SkipSeen: true, Every: 6, Unit: "hours", Enabled: true, RenamePreset: p.ID, EndDate: end}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hk.Create(ctx, hooks.Hook{Kind: "api", Name: "Alert", Method: "POST", URL: "https://example.org/h", Headers: "Content-Type: application/json\nAuthorization: Bearer TOPSECRET",
		Body: "{{payload}}", Events: []string{"torrent.completed"}, Secret: "sign-me", Retries: 2, Timeout: 60, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.hk.Create(ctx, hooks.Hook{Kind: "cmd", Name: "Cmd", Cmd: "echo {{torrent.name}}", Cwd: "/tmp", Timeout: 30, Stdin: true, Events: []string{"torrent.added"}, Retries: 0, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	s := e.set.Get()
	s.Port, s.ProxyPass, s.ProxyType, s.ProxyHost = 6000, "proxy-pw", "socks5", "p.example"
	if _, errs := e.set.Save(ctx, s); len(errs) > 0 {
		t.Fatal(errs)
	}
	mt, _ := e.maint.List(ctx)
	cl := mt[1].Task
	cl.P.Days, cl.Tag = 3, "keep"
	if _, err := e.maint.Update(ctx, cl); err != nil {
		t.Fatal(err)
	}
}

func allParts() []string {
	return []string{"settings", "tags", "presets", "schedules", "maintenance", "hooks"}
}

func TestExport_ShapeAndSecrets(t *testing.T) {
	e := newEnv(t)
	e.populate(t)
	ctx := context.Background()

	plain, err := e.svc.Export(ctx, ExportReq{Parts: allParts()})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"TOPSECRET", "sign-me", "proxy-pw", "webPass"} {
		if strings.Contains(plain.JSON, bad) {
			t.Errorf("export without secrets leaked %q", bad)
		}
	}
	for _, want := range []string{`"swarm_backup": 1`, `"app_version": "0.9.0"`, `"exported_at": "2026-11-02T10:00:00.000Z"`, `Authorization: <removed>`, `"renamePreset": "Tidy"`, `"rename_presets"`, `"command": "echo {{torrent.name}}"`} {
		if !strings.Contains(plain.JSON, want) {
			t.Errorf("export lacks %s", want)
		}
	}
	if plain.Size != len(plain.JSON) {
		t.Errorf("size = %d", plain.Size)
	}

	full, _ := e.svc.Export(ctx, ExportReq{Parts: allParts(), Secrets: true})
	for _, want := range []string{"TOPSECRET", "sign-me", "proxy-pw"} {
		if !strings.Contains(full.JSON, want) {
			t.Errorf("export with secrets lacks %q", want)
		}
	}
	if strings.Contains(full.JSON, "webPass") {
		t.Error("the Web UI password field must never be exported")
	}

	if _, err := e.svc.Export(ctx, ExportReq{}); err == nil {
		t.Error("exporting nothing should fail")
	}
	if !strings.Contains(e.log.lines[len(e.log.lines)-1], "Created an export of") {
		t.Errorf("log = %v", e.log.lines)
	}
	counts := e.svc.Counts(ctx)
	if counts["schedules"] != "1 schedule" || counts["hooks"] != "2 hooks" || counts["presets"] != "1 preset" || counts["maintenance"] != "6 tasks" || !strings.Contains(counts["tags"], "tag") {
		t.Errorf("counts = %v", counts)
	}
}

func TestRoundTrip(t *testing.T) {
	src := newEnv(t)
	src.populate(t)
	ctx := context.Background()
	out, _ := src.svc.Export(ctx, ExportReq{Parts: append(allParts(), "theme"), Secrets: true})

	dst := newEnv(t)
	plan, err := dst.svc.Check(ctx, out.JSON)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for _, p := range plan.Parts {
		keys = append(keys, p.Key)
	}
	if strings.Join(keys, ",") != "settings,tags,schedules,presets,maintenance,hooks" || plan.From != "0.9.0" || plan.At == "" {
		t.Errorf("plan = %+v", plan)
	}

	res, err := dst.svc.Apply(ctx, ApplyReq{Text: out.JSON, Use: allParts(), Mode: "merge"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Skipped) != 0 {
		t.Errorf("skipped = %+v", res.Skipped)
	}
	if !strings.Contains(res.Summary, "1 rename preset") || !strings.Contains(res.Summary, "2 hooks, turned off") {
		t.Errorf("summary = %q", res.Summary)
	}

	if dst.set.Get().Port != 6000 || dst.set.Get().ProxyPass != "proxy-pw" {
		t.Errorf("settings = %+v", dst.set.Get())
	}
	if c := dst.cat.Snapshot(); !slices.Contains(c.Tags, "keep") {
		t.Errorf("tags = %+v", c.Tags)
	}
	presets, _ := dst.ren.List(ctx)
	rules, _ := dst.sch.List(ctx)
	if len(presets) != 1 || len(rules) != 1 || rules[0].RenamePreset != presets[0].ID {
		t.Errorf("schedule did not relink to its preset by name: %+v / %+v", rules, presets)
	}
	hs, _ := dst.hk.Export(ctx)
	if len(hs) != 2 || hs[0].Enabled || hs[1].Enabled || hs[0].Secret != "sign-me" {
		t.Errorf("hooks = %+v", hs)
	}
	mt, _ := dst.maint.List(ctx)
	if mt[1].P.Days != 3 || mt[1].Tag != "keep" {
		t.Errorf("maintenance = %+v", mt[1])
	}

	res2, _ := dst.svc.Apply(ctx, ApplyReq{Text: out.JSON, Use: []string{"presets", "schedules", "hooks"}, Mode: "merge"})
	if p, _ := dst.ren.List(ctx); len(p) != 1 {
		t.Errorf("merge duplicated presets: %d", len(p))
	}
	if h, _ := dst.hk.Export(ctx); len(h) != 2 {
		t.Errorf("merge duplicated hooks: %d", len(h))
	}
	if !strings.Contains(res2.Summary, "0 rename presets") {
		t.Errorf("merge summary = %q", res2.Summary)
	}

	if _, err := dst.svc.Apply(ctx, ApplyReq{Text: out.JSON, Use: []string{"presets", "schedules", "hooks"}, Mode: "replace"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := dst.ren.List(ctx); len(p) != 1 {
		t.Errorf("replace left %d presets", len(p))
	}
}

func TestApply_InvalidItemsAndEdgeCases(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	text := `{"swarm_backup":1,"app_version":"0.8","parts":{
	  "settings":{"port":"not a number","webPass":"hack","savePath":"/x","queueing":false,"unknownKey":1,"webPort":6000,"ratioOn":true,"ratio":-1},
	  "rename_presets":[{"name":"ok","find":"a","replace":"b","target":"weird","when":"add"},{"name":"bad regex","find":"("},{"name":"","find":"a"},"junk"],
	  "schedules":[
	    {"name":"expired","url":"https://e.org/s","selector":"a","endDate":"2001-01-01T00:00:00Z","enabled":true,"max":999,"unit":"weeks","path":"/tmp/x"},
	    {"name":"no selector","template":"https://e.org/s"},
	    {"name":"broken css","url":"https://e.org/s","selector":"a[[","path":"/tmp/x"}],
	  "maintenance":[{"type":"disk","enabled":false,"every":15,"unit":"minutes","settings":{"gb":50,"resume":"yes"}},{"type":"nope"}],
	  "hooks":[{"name":"bad","kind":"api","url":"nope","events":["torrent.added"]},{"name":"good","kind":"api","url":"https://e.org/h","events":["torrent.added","bogus.event"]},{"name":"no events","url":"https://e.org"}]}}`

	plan, err := e.svc.Check(ctx, text)
	if err != nil {
		t.Fatal(err)
	}
	sums := map[string]string{}
	for _, p := range plan.Parts {
		sums[p.Key] = p.Summary
	}
	if sums["presets"] != "2 rename presets (2 invalid, skipped)" && sums["presets"] != "1 rename preset (3 invalid, skipped)" {
		t.Errorf("presets summary = %q", sums["presets"])
	}
	if !strings.Contains(sums["schedules"], "(1 invalid, skipped)") || !strings.Contains(sums["maintenance"], "1 maintenance task (1 invalid") || !strings.Contains(sums["hooks"], "(1 invalid") {
		t.Errorf("summaries = %v", sums)
	}

	res, err := e.svc.Apply(ctx, ApplyReq{Text: text, Use: allParts()})
	if err != nil {
		t.Fatal(err)
	}
	s := e.set.Get()
	if s.WebPort != 6000 || s.Port != 51413 || s.WebPass != "" || s.Ratio != 2 || !s.RatioOn {
		t.Errorf("settings after import = %+v", s)
	}
	if !strings.Contains(res.Done[0], "kept as before") {
		t.Errorf("settings note = %q", res.Done[0])
	}
	rules, _ := e.sch.List(ctx)
	var expired *schedules.View
	for i := range rules {
		if rules[i].Name == "expired" {
			expired = &rules[i]
		}
	}
	if expired == nil || expired.Enabled || expired.EndDate != 0 || expired.Max != 5 || expired.Unit != "hours" {
		t.Errorf("expired schedule should import off with defaults for bad values: %+v", expired)
	}
	if len(res.Skipped) == 0 || res.Skipped[0].Part != "Schedules" || !strings.Contains(res.Skipped[0].Reason, "CSS selector") {
		t.Errorf("skipped = %+v", res.Skipped)
	}
	mt, _ := e.maint.List(ctx)
	for _, m := range mt {
		if m.Type == "disk" && (m.Enabled || m.P.GB != 50 || !m.P.Resume) {
			t.Errorf("disk task = %+v", m)
		}
	}
	hs, _ := e.hk.Export(ctx)
	if len(hs) != 1 || hs[0].Name != "good" || len(hs[0].Events) != 1 || hs[0].Enabled {
		t.Errorf("hooks = %+v", hs)
	}
	presets, _ := e.ren.List(ctx)
	if len(presets) != 1 || presets[0].Target != "files" || presets[0].When != "add" {
		t.Errorf("presets = %+v", presets)
	}
}

func TestCheck_Errors(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name, in, want string
	}{
		{"not json", "{nope", "isn’t valid JSON"},
		{"not a swarm export", `{"a":1}`, "isn’t a Bitswarm export"},
		{"wrong version", `{"swarm_backup":2,"parts":{}}`, "isn’t a Bitswarm export"},
		{"nothing importable", `{"swarm_backup":1,"parts":{"settings":{"nope":1}}}`, "doesn’t contain anything"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.svc.Check(context.Background(), tc.in); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := e.svc.Apply(context.Background(), ApplyReq{Text: "{nope", Use: []string{"settings"}}); err == nil {
		t.Error("Apply with bad JSON should fail")
	}
	if _, err := e.svc.Apply(context.Background(), ApplyReq{Text: `{"swarm_backup":1,"parts":{"settings":{"port":1}}}`}); err == nil {
		t.Error("Apply without parts should fail")
	}
	plan, err := e.svc.Check(context.Background(), `{"swarm_backup":1,"parts":{"theme":{"mode":"dark"},"tags":["a"]}}`)
	if err != nil || plan.Theme == nil || plan.From != "unknown" {
		t.Errorf("theme-only plan = %+v, %v", plan, err)
	}
}

func TestHandler(t *testing.T) {
	e := newEnv(t)
	e.populate(t)
	app := fiber.New()
	NewHandler(e.svc).RegisterRoutes(app)
	exp, _ := e.svc.Export(context.Background(), ExportReq{Parts: allParts()})
	escaped := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(exp.JSON)

	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"counts", http.MethodGet, "/backup/counts", "", 200},
		{"export", http.MethodPost, "/backup/export", `{"parts":["settings"]}`, 200},
		{"export nothing", http.MethodPost, "/backup/export", `{"parts":[]}`, 400},
		{"export bad json", http.MethodPost, "/backup/export", `{`, 400},
		{"check", http.MethodPost, "/backup/check", `{"text":"` + escaped + `"}`, 200},
		{"check invalid", http.MethodPost, "/backup/check", `{"text":"nope"}`, 400},
		{"check bad json", http.MethodPost, "/backup/check", `{`, 400},
		{"apply", http.MethodPost, "/backup/apply", `{"text":"` + escaped + `","use":["tags"],"mode":"merge"}`, 200},
		{"apply invalid", http.MethodPost, "/backup/apply", `{"text":"nope","use":["tags"]}`, 400},
		{"apply bad json", http.MethodPost, "/backup/apply", `{`, 400},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, 5000)
			if err != nil || resp.StatusCode != tc.want {
				t.Errorf("status = %v (%v), want %d", resp, err, tc.want)
			}
		})
	}
}
