package hooks

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"bitswarm/server/internal/db"
)

type memLog struct {
	mu    sync.Mutex
	lines []string
}

func (m *memLog) Add(level, _, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lines = append(m.lines, level+": "+msg)
}

func newSvc(t *testing.T, commands bool) (*Service, *memLog) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	hooks, deliveries := NewCollections(conn)
	l := &memLog{}
	svc := NewService(hooks, deliveries, l, commands)
	svc.sleep = func(time.Duration) {}
	svc.SetSamples(Samples{
		Torrent:  func(context.Context) map[string]any { return torrentInfo("Ubuntu 26.04", "Linux ISOs") },
		Schedule: func(context.Context) (int64, string, []string) { return 3, "Daily", []string{"https://x/s?q=a"} },
	})
	return svc, l
}

func torrentInfo(name, tag string) map[string]any {
	tags := []string{"4K", "keep"}
	if tag != "" {
		tags = []string{tag}
	}
	return map[string]any{"id": 1, "hash": strings.Repeat("a", 40), "name": name, "tags": tags, "size": 1536.0, "size_text": "1.50 KB", "progress": 1.0, "ratio": 0.5, "state": "seeding", "save_path": "/dl"}
}

func apiHook(url string) Hook {
	return Hook{Kind: "api", Name: "H", Method: "POST", URL: url, Headers: "Content-Type: application/json", Body: `{"n":"{{torrent.name}}","p":{{payload}}}`, Events: []string{"torrent.completed"}, Retries: 2, Timeout: 60, Enabled: true}
}

func TestFill(t *testing.T) {
	vars := map[string]string{"torrent.name": `A "quote" & more`, "payload": `{"x":1}`, "count": "3"}
	tests := []struct{ tpl, mode, want string }{
		{"{{torrent.name}}", "text", `A "quote" & more`},
		{"{{ torrent.name }}", "url", "A%20%22quote%22%20%26%20more"},
		{"{{torrent.name}}|{{payload}}", "json", `A \"quote\" & more|{"x":1}`},
		{"{{missing}}!", "text", "!"},
		{"n={{count}}", "shell", "n='3'"},
		{"{{torrent.name}}", "shell", `'A "quote" & more'`},
	}
	for _, tc := range tests {
		if got := Fill(tc.tpl, vars, tc.mode); got != tc.want {
			t.Errorf("Fill(%q, %s) = %q, want %q", tc.tpl, tc.mode, got, tc.want)
		}
	}
	if got := Sq("it's"); got != `'it'\''s'` {
		t.Errorf("Sq() = %s", got)
	}
}

func TestVars(t *testing.T) {
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	v := Vars("torrent.completed", map[string]any{"torrent": torrentInfo("N", ""), "count": 4, "schedule": map[string]any{"name": "S"}, "task": map[string]any{"name": "T"}}, now)
	want := map[string]string{"event": "torrent.completed", "time": "2026-11-02T10:00:00.000Z", "torrent.name": "N", "torrent.tags": "4K, keep",
		"torrent.size": "1.50 KB", "torrent.bytes": "1536", "torrent.path": "/dl", "torrent.ratio": "0.5", "torrent.state": "seeding", "schedule.name": "S", "task.name": "T", "count": "4"}
	for k, w := range want {
		if v[k] != w {
			t.Errorf("Vars[%q] = %q, want %q", k, v[k], w)
		}
	}
	if !strings.Contains(v["payload"], `"event":"torrent.completed"`) || !strings.Contains(v["payload"], `"torrent"`) {
		t.Errorf("payload = %s", v["payload"])
	}
	if _, ok := Vars("settings.saved", map[string]any{}, now)["torrent.name"]; ok {
		t.Error("no torrent should mean no torrent vars")
	}
}

func TestBuild_APIAndCommand(t *testing.T) {
	now := time.Now()
	api := apiHook("https://x.example/hook?name={{torrent.name}}")
	api.Body = `{"name":"{{torrent.name}}"}`
	d := Build(api, "torrent.completed", map[string]any{"torrent": torrentInfo(`Say "hi"`, "")}, now)
	if d.URL != "https://x.example/hook?name=Say%20%22hi%22" || d.Body != `{"name":"Say \"hi\""}` {
		t.Errorf("url/body = %q / %q", d.URL, d.Body)
	}
	if !slices.Contains(d.Headers, [2]string{"X-Swarm-Event", "torrent.completed"}) || !slices.Contains(d.Headers, [2]string{"User-Agent", "BitSwarm/" + Version}) {
		t.Errorf("headers = %v", d.Headers)
	}
	if !strings.HasPrefix(d.Copy, "curl -X POST 'https://x.example/hook") || !strings.HasPrefix(d.Text, "POST https://x.example") {
		t.Errorf("copy/text = %q / %q", d.Copy, d.Text)
	}
	api.Method = "GET"
	if d := Build(api, "torrent.completed", map[string]any{"torrent": torrentInfo("x", "")}, now); d.Body != "" {
		t.Error("GET must not send a body")
	}
	api.Method, api.Headers = "POST", "Content-Type: text/plain\nbogus"
	if d := Build(api, "torrent.completed", map[string]any{"torrent": torrentInfo(`a"b`, "")}, now); d.Body != `{"name":"a"b"}` {
		t.Errorf("non-JSON content type should not JSON-escape: %q", d.Body)
	}

	cmd := Hook{Kind: "cmd", Name: "C", Cmd: "/bin/echo {{torrent.name}} {{torrent.tags}}", Cwd: "/tmp", Timeout: 30, Stdin: true}
	d = Build(cmd, "torrent.completed", map[string]any{"torrent": torrentInfo("x'; rm -rf ~ #", "Lbl"), "schedule": map[string]any{"name": "S"}, "task": map[string]any{"name": "T"}}, now)
	if d.Command != `/bin/echo 'x'\''; rm -rf ~ #' 'Lbl'` {
		t.Errorf("command = %q", d.Command)
	}
	env := map[string]string{}
	for _, kv := range d.Env {
		env[kv[0]] = kv[1]
	}
	for _, k := range []string{"SWARM_EVENT", "SWARM_TIME", "SWARM_TORRENT_NAME", "SWARM_TORRENT_HASH", "SWARM_TORRENT_PATH", "SWARM_TORRENT_SIZE", "SWARM_SCHEDULE", "SWARM_TASK"} {
		if _, ok := env[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
	if d.Stdin == "" || !strings.Contains(d.Text, "# Standard input") || !strings.Contains(d.Text, "longer than 30 s") || !strings.HasPrefix(d.Copy, "cd '/tmp' && printf '%s' ") {
		t.Errorf("text/copy = %q / %q", d.Text, d.Copy)
	}
}

func TestSign(t *testing.T) {
	if got := Sign("secret", "body"); got != "sha256=dc46983557fea127b43af721467eb9b3fde2338fe3e14f51952aa8478c13d355" {
		t.Errorf("Sign() = %s", got)
	}
}

func TestDelivery_SendsSignedRequestWithRetries(t *testing.T) {
	var hits atomic.Int32
	var gotBody, gotSig, gotEvent, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotSig, gotEvent, gotUA = string(b), r.Header.Get("X-Swarm-Signature"), r.Header.Get("X-Swarm-Event"), r.Header.Get("User-Agent")
		if hits.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	svc, log := newSvc(t, false)
	ctx := context.Background()
	h := apiHook(srv.URL)
	h.Secret = "s3cret"
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if v.Secret != "" || !v.HasSecret {
		t.Errorf("secret leaked in view: %+v", v)
	}

	svc.Emit("torrent.completed", map[string]any{"torrent": torrentInfo("Ubuntu", "")})
	svc.Emit("torrent.added", map[string]any{"torrent": torrentInfo("Other", "")})
	svc.Wait()

	if hits.Load() != 3 {
		t.Fatalf("hits = %d, want 3 (two 503s then success)", hits.Load())
	}
	if gotSig != Sign("s3cret", gotBody) || gotEvent != "torrent.completed" || !strings.HasPrefix(gotUA, "BitSwarm/") {
		t.Errorf("sig=%q event=%q ua=%q", gotSig, gotEvent, gotUA)
	}
	ds, _ := svc.Deliveries(ctx)
	if len(ds) != 1 || ds[0].Status != "sent" || ds[0].Attempts != 3 || ds[0].Response != 204 {
		t.Fatalf("deliveries = %+v", ds)
	}
	if hs, _ := svc.List(ctx); hs[0].Count != 1 || hs[0].LastEvent != "torrent.completed" {
		t.Errorf("counters = %+v", hs[0])
	}
	if len(log.lines) != 1 || !strings.HasPrefix(log.lines[0], "debug: H: request for torrent.completed. Sent, the server answered 204.") {
		t.Errorf("log = %v", log.lines)
	}
}

func TestDelivery_Failures(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(404)
	}))
	defer srv.Close()
	svc, log := newSvc(t, false)
	ctx := context.Background()

	h := apiHook(srv.URL)
	h.Retries = 4
	v, _ := svc.Create(ctx, h)
	d, err := svc.Test(ctx, v.ID)
	if err != nil || d.Status != "failed" || d.Response != 404 || hits.Load() != 1 || !d.Test {
		t.Errorf("4xx should fail without retrying: %+v hits=%d err=%v", d, hits.Load(), err)
	}
	if !strings.HasPrefix(log.lines[0], "warn:") {
		t.Errorf("failures log as warnings: %v", log.lines)
	}

	dead := apiHook("http://127.0.0.1:1/x")
	vv, _ := svc.Create(ctx, dead)
	if d, _ := svc.Test(ctx, vv.ID); d.Status != "failed" || d.Error == "" || d.Attempts != 1 {
		t.Errorf("unreachable host = %+v", d)
	}
	if _, err := svc.Test(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
}

func TestDelivery_Commands(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	off, _ := newSvc(t, false)
	h := Hook{Kind: "cmd", Name: "C", Cmd: "touch " + filepath.Join(dir, "ran"), Timeout: 5, Events: []string{"torrent.completed"}, Enabled: true}
	v, _ := off.Create(ctx, h)
	if d, _ := off.Test(ctx, v.ID); d.Status != "not_run" || !strings.Contains(d.Error, "SWARM_ALLOW_COMMANDS=1") {
		t.Errorf("disabled commands = %+v", d)
	}
	if _, err := os.Stat(filepath.Join(dir, "ran")); err == nil {
		t.Fatal("a command ran while commands were off")
	}

	on, _ := newSvc(t, true)
	if !on.Config().CommandsEnabled || off.Config().CommandsEnabled {
		t.Error("Config() wrong")
	}
	evil := "x'; echo pwned > " + filepath.Join(dir, "pwned") + " #"
	on.SetSamples(Samples{Torrent: func(context.Context) map[string]any { return torrentInfo(evil, "") }})
	out := filepath.Join(dir, "out")
	h = Hook{Kind: "cmd", Name: "C", Cmd: "printf '%s' {{torrent.name}} > " + Sq(out) + "; cat >> " + Sq(out+".stdin"), Cwd: dir, Timeout: 5, Stdin: true, Events: []string{"torrent.completed"}, Enabled: true}
	v, _ = on.Create(ctx, h)
	d, _ := on.Test(ctx, v.ID)
	if d.Status != "ran" {
		t.Fatalf("command = %+v", d)
	}
	if b, _ := os.ReadFile(out); string(b) != evil {
		t.Errorf("name did not arrive as one literal argument: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("shell injection succeeded")
	}
	if b, _ := os.ReadFile(out + ".stdin"); !strings.Contains(string(b), `"event":"torrent.completed"`) {
		t.Errorf("stdin = %q", b)
	}

	h.Cmd, h.Timeout, h.Cwd = "sleep 5", 1, ""
	h.ID = v.ID
	if _, err := on.Update(ctx, h); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if d, _ := on.Test(ctx, v.ID); d.Status != "failed" || !strings.Contains(d.Error, "stopped after 1 seconds") || time.Since(start) > 4*time.Second {
		t.Errorf("timeout = %+v", d)
	}
	h.Cmd, h.Timeout = "echo oops >&2; exit 3", 5
	_, _ = on.Update(ctx, h)
	if d, _ := on.Test(ctx, v.ID); d.Status != "failed" || !strings.Contains(d.Output, "oops") || !strings.Contains(d.Error, "exit status 3") {
		t.Errorf("failing command = %+v", d)
	}
	if expandHome("~/x") == "~/x" || expandHome("/a") != "/a" {
		t.Error("expandHome wrong")
	}
}

func TestEmit_FiltersByTagAndEnabled(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1) }))
	defer srv.Close()
	svc, _ := newSvc(t, false)
	ctx := context.Background()

	scoped := apiHook(srv.URL)
	scoped.Tag, scoped.Retries = "movies", 0
	svc.Create(ctx, scoped)
	off := apiHook(srv.URL)
	off.Enabled, off.Retries = false, 0
	svc.Create(ctx, off)
	global := apiHook(srv.URL)
	global.Events, global.Tag, global.Retries = []string{"settings.saved"}, "movies", 0
	svc.Create(ctx, global)

	svc.Emit("torrent.completed", map[string]any{"torrent": torrentInfo("a", "docs")})
	svc.Emit("torrent.completed", map[string]any{"torrent": torrentInfo("b", "movies")})
	svc.Emit("settings.saved", map[string]any{})
	svc.Wait()
	if hits.Load() != 2 {
		t.Errorf("hits = %d, want 2 (tag match + non-torrent event ignoring the tag)", hits.Load())
	}
}

func TestValidate(t *testing.T) {
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	tests := []struct {
		name    string
		mutate  func(*Hook)
		wantErr string
	}{
		{"ok", func(*Hook) {}, ""},
		{"no name", func(h *Hook) { h.Name = " " }, "Give the hook a name."},
		{"bad kind", func(h *Hook) { h.Kind = "x" }, "call an API, run a command or run curl"},
		{"curl ok", func(h *Hook) {
			*h = Hook{Kind: "curl", Name: "c", Cmd: "curl -X POST https://example.org/h -d name={{torrent.name}}", Timeout: 5, Events: []string{"torrent.added"}}
		}, ""},
		{"curl must start with curl", func(h *Hook) {
			*h = Hook{Kind: "curl", Name: "c", Cmd: "wget https://example.org", Timeout: 5, Events: []string{"torrent.added"}}
		}, "must start with curl"},
		{"unknown event", func(h *Hook) { h.Events = []string{"nope"} }, "isn’t an event BitSwarm knows"},
		{"bad method", func(h *Hook) { h.Method = "DELETE" }, "POST, PUT, PATCH or GET"},
		{"bad url", func(h *Hook) { h.URL = "ftp://x" }, "full URL to call"},
		{"no events", func(h *Hook) { h.Events = nil }, "at least one event"},
		{"bad header", func(h *Hook) { h.Headers = "Content-Type: application/json\nnonsense" }, "isn’t a valid header"},
		{"bad header name", func(h *Hook) { h.Headers = "Bad Name: x" }, "isn’t a valid header"},
		{"retries", func(h *Hook) { h.Retries = 11 }, "Retries must be"},
		{"invalid json body", func(h *Hook) { h.Body = `{"n": {{torrent.name}}}` }, "isn’t valid JSON"},
		{"non-json body is fine", func(h *Hook) { h.Headers, h.Body = "Content-Type: text/plain", "{{torrent.name" }, ""},
		{"cmd ok", func(h *Hook) {
			*h = Hook{Kind: "cmd", Name: "c", Cmd: "ls", Timeout: 5, Events: []string{"torrent.added"}}
		}, ""},
		{"cmd empty", func(h *Hook) { *h = Hook{Kind: "cmd", Name: "c", Timeout: 5, Events: []string{"torrent.added"}} }, "Enter the command to run."},
		{"cmd relative cwd", func(h *Hook) {
			*h = Hook{Kind: "cmd", Name: "c", Cmd: "ls", Cwd: "scripts", Timeout: 5, Events: []string{"torrent.added"}}
		}, "full path"},
		{"cmd no events", func(h *Hook) { *h = Hook{Kind: "cmd", Name: "c", Cmd: "ls", Timeout: 5} }, "at least one event"},
		{"cmd timeout", func(h *Hook) {
			*h = Hook{Kind: "cmd", Name: "c", Cmd: "ls", Timeout: 0, Events: []string{"torrent.added"}}
		}, "from 1 to 3600"},
		{"cmd retries", func(h *Hook) {
			*h = Hook{Kind: "cmd", Name: "c", Cmd: "ls", Timeout: 5, Retries: -1, Events: []string{"torrent.added"}}
		}, "Retries must be"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := apiHook("https://example.org/hook")
			tc.mutate(&h)
			_, err := svc.Create(ctx, h)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestCRUDAndSecrets(t *testing.T) {
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	h := apiHook("https://example.org/hook")
	h.Secret = "one"
	v, _ := svc.Create(ctx, h)

	h.ID, h.Secret = v.ID, ""
	if _, err := svc.Update(ctx, h); err != nil {
		t.Fatal(err)
	}
	if got, _ := svc.hooks.Get(ctx, v.ID); got.Secret != "one" {
		t.Errorf("blank secret should keep the old one, got %q", got.Secret)
	}
	h.Secret = "two"
	_, _ = svc.Update(ctx, h)
	if got, _ := svc.hooks.Get(ctx, v.ID); got.Secret != "two" {
		t.Errorf("secret = %q", got.Secret)
	}
	h.Secret, h.ClearSecret = "", true
	if out, _ := svc.Update(ctx, h); out.HasSecret {
		t.Error("clearSecret should remove the secret")
	}
	h.ID = 99
	if _, err := svc.Update(ctx, h); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing err = %v", err)
	}
	h.ID, h.Name = v.ID, ""
	if _, err := svc.Update(ctx, h); err == nil {
		t.Error("invalid update accepted")
	}

	if off, err := svc.SetEnabled(ctx, v.ID, false); err != nil || off.Enabled {
		t.Errorf("SetEnabled = %+v, %v", off, err)
	}
	if _, err := svc.SetEnabled(ctx, 99, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
	dup, err := svc.Duplicate(ctx, v.ID)
	if err != nil || !strings.HasSuffix(dup.Name, " copy") || dup.Enabled {
		t.Errorf("Duplicate() = %+v, %v", dup, err)
	}
	if _, err := svc.Duplicate(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("duplicate missing err = %v", err)
	}
	if err := svc.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(ctx, v.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice err = %v", err)
	}
}

func TestPreviewAndSamples(t *testing.T) {
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	h := apiHook("https://example.org/{{torrent.name}}")
	h.Secret = "s"
	h.Body = "{{torrent.name}} {{bogus}} {{schedule.name}}"
	p := svc.Preview(ctx, h)
	if p.Event != "torrent.completed" || !p.Signed || !slices.Equal(p.Unknown, []string{"bogus"}) || !slices.Equal(p.Empty, []string{"schedule.name"}) || !strings.Contains(p.Text, "X-Swarm-Signature") {
		t.Errorf("preview = %+v", p)
	}
	h.Events = []string{"schedule.finished"}
	if p := svc.Preview(ctx, h); p.Event != "schedule.finished" || len(p.Empty) != 1 || p.Empty[0] != "torrent.name" {
		t.Errorf("schedule preview = %+v", p)
	}

	for _, ev := range Events {
		if d := svc.sample(ctx, ev); d == nil {
			t.Errorf("no sample for %s", ev)
		}
	}
	plain := NewService(svc.hooks, svc.deliveries, &memLog{}, false)
	if d := plain.sample(ctx, "torrent.added"); d["torrent"] == nil || d["source"] != "manual" {
		t.Errorf("fallback sample = %v", d)
	}
	if d := plain.sample(ctx, "schedule.search"); d["trigger"] != "schedule" {
		t.Errorf("fallback schedule sample = %v", d)
	}
	if d := plain.sample(ctx, "unknown.event"); len(d) != 0 {
		t.Errorf("unknown event sample = %v", d)
	}
	for ev, key := range map[string]string{"torrent.renamed": "preset", "schedule.links": "links", "update.available": "latest", "maintenance.run": "result"} {
		if _, ok := svc.sample(ctx, ev)[key]; !ok {
			t.Errorf("%s sample lacks %s", ev, key)
		}
	}
}

func TestSeed(t *testing.T) {
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	if err := svc.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	if err := svc.Seed(ctx); err != nil {
		t.Fatal(err)
	}
	hs, _ := svc.List(ctx)
	if len(hs) != 5 || hs[4].Enabled || hs[3].Kind != "curl" || hs[3].Enabled || !hs[0].Enabled || len(hs[1].Events) != len(Events) || !hs[1].HasSecret {
		t.Errorf("seed = %d hooks", len(hs))
	}
	if StatusText(Delivery{Status: "ran"}) != "Ran and exited with 0." || !strings.HasPrefix(StatusText(Delivery{Status: "not_run", Error: "x"}), "Not run:") || !strings.HasPrefix(StatusText(Delivery{Status: "failed", Error: "x"}), "Failed:") {
		t.Error("StatusText wrong")
	}
	if Marshal(map[string]int{"a": 1}) != `{"a":1}` {
		t.Error("Marshal wrong")
	}
}

func TestHandler(t *testing.T) {
	svc, _ := newSvc(t, false)
	app := fiber.New()
	NewHandler(svc).RegisterRoutes(app)
	body := `{"kind":"api","name":"H","method":"POST","url":"https://example.org/h","headers":"Content-Type: application/json","body":"{{payload}}","events":["torrent.added"],"retries":0,"timeout":60,"enabled":true}`
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/hooks", "", 200},
		{"config", http.MethodGet, "/hooks/config", "", 200},
		{"deliveries", http.MethodGet, "/hooks/deliveries", "", 200},
		{"create", http.MethodPost, "/hooks", body, 201},
		{"create invalid", http.MethodPost, "/hooks", `{"name":"x"}`, 400},
		{"create bad json", http.MethodPost, "/hooks", `{`, 400},
		{"preview", http.MethodPost, "/hooks/preview", body, 200},
		{"preview bad json", http.MethodPost, "/hooks/preview", `{`, 400},
		{"update", http.MethodPut, "/hooks/1", body, 200},
		{"update missing", http.MethodPut, "/hooks/9", body, 404},
		{"update bad id", http.MethodPut, "/hooks/x", body, 400},
		{"update bad json", http.MethodPut, "/hooks/1", `{`, 400},
		{"enable", http.MethodPatch, "/hooks/1/enabled", `{"enabled":false}`, 200},
		{"enable bad json", http.MethodPatch, "/hooks/1/enabled", `{`, 400},
		{"enable bad id", http.MethodPatch, "/hooks/x/enabled", `{}`, 400},
		{"duplicate", http.MethodPost, "/hooks/1/duplicate", "", 201},
		{"duplicate bad id", http.MethodPost, "/hooks/x/duplicate", "", 400},
		{"test missing", http.MethodPost, "/hooks/9/test", "", 404},
		{"test bad id", http.MethodPost, "/hooks/x/test", "", 400},
		{"delete", http.MethodDelete, "/hooks/2", "", 204},
		{"delete missing", http.MethodDelete, "/hooks/2", "", 404},
		{"delete bad id", http.MethodDelete, "/hooks/x", "", 400},
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

func TestDelivery_CurlHook(t *testing.T) {
	if _, err := exec.LookPath("curl"); err != nil {
		t.Skip("curl isn't installed")
	}
	ctx := context.Background()
	var (
		mu  sync.Mutex
		got struct{ method, body, event string }
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		defer mu.Unlock()
		got.method, got.body, got.event = r.Method, string(b), r.Header.Get("X-Test")
		w.WriteHeader(204)
	}))
	defer srv.Close()

	h := Hook{Kind: "curl", Name: "C", Cmd: "curl -sf -X POST " + srv.URL + " -H 'X-Test: yes' --data-raw name={{torrent.name}}", Timeout: 10, Events: []string{"torrent.completed"}, Enabled: true}

	off, _ := newSvc(t, false)
	v, _ := off.Create(ctx, h)
	mu.Lock()
	called := got.method != ""
	mu.Unlock()
	if d, _ := off.Test(ctx, v.ID); d.Status != "not_run" || called {
		t.Fatalf("curl ran with commands off: %+v", d)
	}

	on, _ := newSvc(t, true)
	evil := "x'; echo pwned #"
	on.SetSamples(Samples{Torrent: func(context.Context) map[string]any { return torrentInfo(evil, "") }})
	v, err := on.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := on.Test(ctx, v.ID)
	if d.Status != "ran" || d.Kind != "curl" {
		t.Fatalf("delivery = %+v", d)
	}
	mu.Lock()
	defer mu.Unlock()
	if got.method != "POST" || got.body != "name="+evil || got.event != "yes" {
		t.Errorf("server saw %+v", got)
	}
}

func TestBuild_CurlNeutralizesOptionsAndFiles(t *testing.T) {
	tests := []struct{ name, want string }{
		{"@/etc/passwd", `' @/etc/passwd'`},
		{"<secret.txt", `' <secret.txt'`},
		{"-o /tmp/x", `' -o /tmp/x'`},
		{"plain name", `'plain name'`},
		{"x'; echo pwned #", `'x'\''; echo pwned #'`},
	}
	for _, tc := range tests {
		h := Hook{Kind: "curl", Cmd: "curl -d {{torrent.name}}", Timeout: 5}
		d := Build(h, "torrent.completed", map[string]any{"torrent": torrentInfo(tc.name, "")}, time.Unix(0, 0))
		if want := "curl -d " + tc.want; d.Command != want {
			t.Errorf("%q: command = %s, want %s", tc.name, d.Command, want)
		}
	}
	c := Build(Hook{Kind: "cmd", Cmd: "echo {{torrent.name}}", Timeout: 5}, "torrent.completed", map[string]any{"torrent": torrentInfo("@x", "")}, time.Unix(0, 0))
	if c.Command != "echo '@x'" {
		t.Errorf("command hooks must stay unchanged, got %s", c.Command)
	}
}

func TestDelivery_CurlRefusesUnsafePlaceholders(t *testing.T) {
	ctx := context.Background()
	svc, _ := newSvc(t, true)
	v, err := svc.Create(ctx, Hook{Kind: "curl", Name: "C", Cmd: "curl -sf https://example.org -d name={{torrent.name}}", Timeout: 5, Events: []string{"torrent.completed"}, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, Hook{Kind: "curl", Name: "Bad", Cmd: "curl --data-urlencode {{torrent.name}} https://example.org", Timeout: 5, Events: []string{"torrent.completed"}}); err == nil || !strings.Contains(err.Error(), "equals sign") {
		t.Errorf("save error = %v", err)
	}
	stored, _ := svc.hooks.Get(ctx, v.ID)
	stored.Cmd = "curl --data-urlencode {{torrent.name}} https://example.org"
	_ = svc.hooks.Put(ctx, stored)
	if d, _ := svc.Test(ctx, v.ID); d.Status != "failed" || !strings.Contains(d.Error, "not run") {
		t.Errorf("an unsafe saved hook ran: %+v", d)
	}
}
