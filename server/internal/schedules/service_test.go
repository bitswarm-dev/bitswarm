package schedules

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
	svc *Service
	tor *torrents.Service
	bus *bus
	now time.Time
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
	tor := torrents.NewService(torrents.NewRepository(conn), set, cat, b, nopLog{})
	rules, log := NewCollections(conn)
	f := &fixture{tor: tor, bus: b, now: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)}
	f.svc = NewService(rules, log, tor, b)
	f.svc.now = func() time.Time { return f.now }
	return f
}

const page = `<ul>
 <li><a class="m" href="magnet:?xt=urn:btih:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&dn=Ubuntu+Desktop">Magnet</a></li>
 <li><a class="m" href="magnet:?xt=urn:btih:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb&dn=Ubuntu+Server">Magnet</a></li>
 <li><a class="m" href="magnet:?xt=urn:btih:cccccccccccccccccccccccccccccccccccccccc&dn=Fedora">Magnet</a></li>
 <li><a class="m" href="magnet:?xt=urn:btih:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa&dn=Ubuntu+Desktop">Magnet again</a></li>
 <li><a class="t" href="/dl/thing.torrent" title="A torrent file">T</a></li>
 <li><a class="t" href="/dl/other.zip">Z</a></li>
 <li><a class="m" href="">Empty</a></li>
</ul>`

func goodRule() Rule {
	return Rule{Name: "R", URL: "https://example.org/s?q={query}", Terms: "ubuntu\nfedora", Selector: "a.m", Attr: "href",
		LinkType: "magnet", Include: "ubuntu, desktop", Exclude: "server", Max: 5, Start: true, SkipSeen: true, Every: 6, Unit: "hours", Enabled: true, Tags: []string{"isos"}, Path: "/dl"}
}

func TestSearchURLs(t *testing.T) {
	r := goodRule()
	r.Terms = "a b\n\n c&d "
	want := []string{"https://example.org/s?q=a+b", "https://example.org/s?q=c%26d"}
	if got := searchURLs(r); !slices.Equal(got, want) {
		t.Errorf("searchURLs() = %v, want %v", got, want)
	}
	r.URL = "https://example.org/all"
	if got := searchURLs(r); len(got) != 1 || got[0] != r.URL {
		t.Errorf("no {query} = %v", got)
	}
	if searchURLs(Rule{}) != nil {
		t.Error("empty URL should give no URLs")
	}
}

func TestNameFor(t *testing.T) {
	tests := []struct {
		name string
		r    Rule
		want string
	}{
		{"first term", Rule{URL: "https://e.org/s?q={query}", Terms: "\n Tensei Goblin \nother"}, "Tensei Goblin"},
		{"query parameter", Rule{URL: "https://nyaa.si/user/x?f=0&q=Tensei+Goblin++1080p"}, "Tensei Goblin 1080p"},
		{"encoded parameter", Rule{URL: "https://example.org/s?search=ubuntu%2026.04"}, "ubuntu 26.04"},
		{"host", Rule{URL: "https://example.org/all"}, "example.org"},
		{"not a URL", Rule{URL: "nope"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nameFor(tc.r); got != tc.want {
				t.Errorf("nameFor() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtract(t *testing.T) {
	r := goodRule()
	r.Include, r.Exclude = "", ""
	res := Extract(r, page, nil)
	if res.Error != "" || res.Matched != 5 || len(res.Items) != 3 {
		t.Fatalf("magnets: matched %d, items %d, err %q", res.Matched, len(res.Items), res.Error)
	}
	if res.Items[0].Name != "Ubuntu Desktop" || res.Items[0].Btih != strings.Repeat("a", 40) || res.Items[0].Key != res.Items[0].Btih {
		t.Errorf("first item = %+v", res.Items[0])
	}

	r.Include, r.Exclude = "ubuntu, desktop", "server"
	res = Extract(r, page, nil)
	states := map[string]string{}
	for _, i := range res.Items {
		states[i.Name] = i.State
	}
	if states["Ubuntu Desktop"] != "new" || states["Ubuntu Server"] != "filtered" || states["Fedora"] != "filtered" {
		t.Errorf("states = %v", states)
	}

	r.Include, r.Exclude, r.Max = "", "", 1
	res = Extract(r, page, nil)
	if res.Items[0].State != "new" || res.Items[1].State != "over" {
		t.Errorf("max not applied: %+v", res.Items)
	}

	r.Max, r.Seen = 5, []string{strings.Repeat("b", 40)}
	res = Extract(r, page, known{[]torrents.Torrent{{Name: "Fedora"}}})
	if res.Items[0].State != "new" || res.Items[1].State != "have" || res.Items[2].State != "have" {
		t.Errorf("seen/have not applied: %+v", res.Items)
	}
	r.SkipSeen = false
	if res = Extract(r, page, known{[]torrents.Torrent{{Name: "Fedora"}}}); res.Items[2].State != "new" {
		t.Error("skipSeen off should ignore known items")
	}

	r.Selector, r.LinkType = "a.t", "torrent"
	res = Extract(r, page, nil)
	if len(res.Items) != 1 || res.Items[0].URL != "https://example.org/dl/thing.torrent" || res.Items[0].Name != "A torrent file" || res.Items[0].Type != "torrent" {
		t.Errorf("torrent link = %+v", res.Items)
	}
	r.LinkType = "any"
	if res = Extract(r, page, nil); len(res.Items) != 1 {
		t.Errorf("non-torrent links should be skipped: %+v", res.Items)
	}

	r.Selector, r.Attr = "a.t", "text"
	if res = Extract(r, page, nil); len(res.Items) != 0 {
		t.Errorf("text attr with no links = %+v", res.Items)
	}
	r.Selector = "a[["
	if res = Extract(r, page, nil); res.Error == "" {
		t.Error("invalid selector should be reported")
	}
	if err := CheckSelector("a["); err == nil {
		t.Error("CheckSelector should fail")
	}
}

func TestExtract_NameFallbacks(t *testing.T) {
	r := goodRule()
	r.LinkType, r.Selector, r.Include, r.Exclude = "any", "a", "", ""
	html := `<a href="/files/My%20File.torrent">x</a><a href="/files/Long.torrent">A much longer link text</a>`
	res := Extract(r, html, nil)
	if len(res.Items) != 2 || res.Items[0].Name != "My File" || res.Items[1].Name != "A much longer link text" {
		t.Errorf("names = %+v", res.Items)
	}
}

func TestValidate(t *testing.T) {
	f := newFixture(t)
	tests := []struct {
		name    string
		mutate  func(*Rule)
		wantErr string
	}{
		{"ok", func(*Rule) {}, ""},
		{"bad url", func(r *Rule) { r.URL = "nope" }, "full search URL"},
		{"no terms", func(r *Rule) { r.Terms = "" }, "at least one search term"},
		{"no selector", func(r *Rule) { r.Selector = " " }, "CSS selector for the links"},
		{"invalid selector", func(r *Rule) { r.Selector = "a[[" }, "isn’t a valid CSS selector"},
		{"bad attr", func(r *Rule) { r.Attr = "x" }, "read the link from"},
		{"bad type", func(r *Rule) { r.LinkType = "x" }, "kind of link"},
		{"bad unit", func(r *Rule) { r.Unit = "weeks" }, "hours or days"},
		{"bad every", func(r *Rule) { r.Every = 0 }, "hours or days"},
		{"bad max", func(r *Rule) { r.Max = 99 }, "from 1 to 50"},
		{"past end", func(r *Rule) { r.EndDate = f.now.Add(-time.Hour).UnixMilli() }, "in the past"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := goodRule()
			tc.mutate(&r)
			_, err := f.svc.Create(context.Background(), r)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestRun_AddsNewAndSkipsSeen(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var fetched []string
	f.svc.SetFetcher(func(_ context.Context, u string) (string, error) {
		fetched = append(fetched, u)
		return page, nil
	})
	v, err := f.svc.Create(ctx, goodRule())
	if err != nil {
		t.Fatal(err)
	}
	first, err := f.svc.Run(ctx, v.ID, "manual")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "ok" || first.LastNew != 1 || first.LastFound != 3 || len(fetched) != 2 {
		t.Fatalf("first run = %+v, fetched %v", first.Rule, fetched)
	}
	if first.NextRun != f.now.UnixMilli()+6*hour {
		t.Errorf("next run = %d", first.NextRun)
	}
	ts, _ := f.tor.Snapshot(ctx)
	if len(ts) != 1 || ts[0].Name != "Ubuntu Desktop" || !slices.Equal(ts[0].Tags, []string{"isos"}) || ts[0].Path != "/dl" || ts[0].Tracker != "DHT only" {
		t.Fatalf("added = %+v", ts)
	}
	second, _ := f.svc.Run(ctx, v.ID, "manual")
	if second.LastNew != 0 {
		t.Errorf("second run added %d", second.LastNew)
	}
	want := []string{"schedule.search", "torrent.added", "schedule.links", "schedule.finished"}
	for _, ev := range want {
		if !slices.Contains(f.bus.events, ev) {
			t.Errorf("missing %s in %v", ev, f.bus.events)
		}
	}
	log, _ := f.svc.Log(ctx)
	if len(log) != 2 || log[1].Kind != "ok" || log[0].Kind != "none" || len(log[1].Added) != 1 {
		t.Errorf("log = %+v", log)
	}
	if _, err := f.svc.Run(ctx, 99, "manual"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}
}

func TestRun_Failures(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	v, _ := f.svc.Create(ctx, goodRule())

	f.svc.SetFetcher(func(context.Context, string) (string, error) {
		return "", errors.New("example.org answered with status 503")
	})
	res, _ := f.svc.Run(ctx, v.ID, "manual")
	if res.Status != "error" || !strings.Contains(res.LastError, "503") {
		t.Errorf("fetch failure = %+v", res.Rule)
	}

	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	bad := goodRule()
	bad.ID = v.ID
	bad.Selector = "a.m"
	stored, _ := f.svc.rules.Get(ctx, v.ID)
	stored.Selector = "a[["
	_ = f.svc.rules.Put(ctx, stored)
	res, _ = f.svc.Run(ctx, v.ID, "manual")
	if res.Status != "error" || !strings.Contains(res.LastError, "valid CSS selector") {
		t.Errorf("selector failure = %+v", res.Rule)
	}
	log, _ := f.svc.Log(ctx)
	if log[0].Kind != "err" {
		t.Errorf("log = %+v", log[0])
	}
}

func TestPreviewTestAndDownload(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := goodRule()
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	v, _ := f.svc.Create(ctx, r)

	pv, err := f.svc.Preview(ctx, v.ID)
	if err != nil || len(pv.Urls) != 2 || len(pv.Extraction.Items) != 3 {
		t.Fatalf("Preview() = %+v, %v", pv, err)
	}
	var key string
	for _, i := range pv.Extraction.Items {
		if i.State == "new" {
			key = i.Key
		}
	}
	n, err := f.svc.DownloadFromPreview(ctx, v.ID, []string{key, "unknown"})
	if err != nil || n != 1 {
		t.Fatalf("Download = %d, %v", n, err)
	}
	if n, _ = f.svc.DownloadFromPreview(ctx, v.ID, []string{key}); n != 0 {
		t.Errorf("already-have item downloaded again: %d", n)
	}
	if _, err := f.svc.DownloadFromPreview(ctx, 99, nil); err == nil {
		t.Error("missing schedule should fail")
	}
	if _, err := f.svc.Preview(ctx, 99); err == nil {
		t.Error("missing schedule should fail")
	}

	r2 := goodRule()
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return "", errors.New("offline") })
	v2, _ := f.svc.Create(ctx, r2)
	if pv, _ := f.svc.Preview(ctx, v2.ID); pv.Extraction.Error != "offline" {
		t.Errorf("preview without sample and offline = %+v", pv)
	}
	if _, err := f.svc.DownloadFromPreview(ctx, v2.ID, nil); err == nil {
		t.Error("offline download should fail")
	}

	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	if _, err := f.svc.Test(ctx, Rule{Selector: "a"}); err == nil {
		t.Error("Test() without a URL should fail")
	}
	if _, err := f.svc.Test(ctx, Rule{URL: "https://example.org/s"}); err == nil {
		t.Error("Test() without a selector should fail")
	}
	if res, err := f.svc.Test(ctx, Rule{URL: "https://example.org/s", Selector: "a.m"}); err != nil || len(res.Items) != 3 {
		t.Errorf("Test() = %+v, %v", res, err)
	}
}

func TestFetchesAreSpacedByTheDelay(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	var stamps []time.Time
	f.svc.SetFetcher(func(context.Context, string) (string, error) {
		stamps = append(stamps, time.Now())
		return page, nil
	})
	f.svc.SetDelay(func() time.Duration { return 120 * time.Millisecond })
	if _, err := f.svc.Test(ctx, goodRule()); err != nil {
		t.Fatal(err)
	}
	if len(stamps) != 2 {
		t.Fatalf("fetched %d pages, want 2 (one per term)", len(stamps))
	}
	if gap := stamps[1].Sub(stamps[0]); gap < 110*time.Millisecond {
		t.Errorf("gap between searches = %v, want at least the delay", gap)
	}
	f.svc.SetDelay(func() time.Duration { return 0 })
	stamps = nil
	start := time.Now()
	if _, err := f.svc.Test(ctx, goodRule()); err != nil || time.Since(start) > 100*time.Millisecond {
		t.Errorf("zero delay still waited: %v, %v", time.Since(start), err)
	}
	cctx, cancel := context.WithCancel(ctx)
	f.svc.SetDelay(func() time.Duration { return time.Minute })
	cancel()
	if _, err := f.svc.pageFor(cctx, goodRule()); err == nil {
		t.Error("a cancelled run should stop waiting")
	}
}

func TestEnableUpdateDeleteAndEnd(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	r := goodRule()
	r.EndDate = f.now.Add(48 * time.Hour).UnixMilli()
	v, _ := f.svc.Create(ctx, r)
	if v.RunsLeft != 8 {
		t.Errorf("runsLeft = %d", v.RunsLeft)
	}

	off, err := f.svc.SetEnabled(ctx, v.ID, false)
	if err != nil || off.Enabled {
		t.Fatalf("SetEnabled(false) = %+v, %v", off.Rule, err)
	}
	f.now = f.now.Add(3 * 24 * time.Hour)
	if _, err := f.svc.SetEnabled(ctx, v.ID, true); err == nil || !strings.Contains(err.Error(), "end date has passed") {
		t.Errorf("enabling after end date err = %v", err)
	}
	if _, err := f.svc.SetEnabled(ctx, 99, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing err = %v", err)
	}

	f.now = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	upd := goodRule()
	upd.ID, upd.Every, upd.Unit = v.ID, 2, "days"
	out, err := f.svc.Update(ctx, upd)
	if err != nil || out.NextRun != f.now.UnixMilli()+2*day {
		t.Errorf("Update() next run = %d, %v", out.NextRun, err)
	}
	upd.ID = 99
	if _, err := f.svc.Update(ctx, upd); !errors.Is(err, ErrNotFound) {
		t.Errorf("update missing err = %v", err)
	}
	upd.Name = ""
	if _, err := f.svc.Update(ctx, upd); err == nil {
		t.Error("invalid update should fail")
	}

	f.svc.ClearPreset(7)
	if err := f.svc.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Delete(ctx, v.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete twice err = %v", err)
	}
}

func TestTick_RunsDueAndEnds(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	r := goodRule()
	r.EndDate = f.now.Add(8 * time.Hour).UnixMilli()
	v, _ := f.svc.Create(ctx, r)

	f.svc.Tick(ctx)
	if got, _ := f.svc.Get(ctx, v.ID); got.LastRun != 0 {
		t.Error("ran before it was due")
	}
	f.now = f.now.Add(7 * time.Hour)
	f.svc.Tick(ctx)
	got, _ := f.svc.Get(ctx, v.ID)
	if got.LastRun == 0 || got.Status != "ok" {
		t.Fatalf("due run missing: %+v", got.Rule)
	}
	if !got.Ended || got.Enabled {
		t.Errorf("rule should end when the next run is past the end date: %+v", got.Rule)
	}
	name, _, ok := f.svc.NextRun(ctx)
	if ok || name != "" {
		t.Error("ended rules have no next run")
	}

	on, _ := f.svc.Create(ctx, goodRule())
	if name, _, ok := f.svc.NextRun(ctx); !ok || name != on.Name {
		t.Errorf("NextRun() = %q, %v", name, ok)
	}
	f.now = f.now.Add(3 * 24 * time.Hour)
	stored, _ := f.svc.rules.Get(ctx, on.ID)
	stored.EndDate = f.now.Add(-time.Hour).UnixMilli()
	_ = f.svc.rules.Put(ctx, stored)
	f.svc.Tick(ctx)
	if got, _ := f.svc.Get(ctx, on.ID); !got.Ended || got.Enabled {
		t.Errorf("past end date not ended: %+v", got.Rule)
	}
	if users := f.svc.UsersOf(0); len(users) != 2 {
		t.Errorf("UsersOf(0) = %v", users)
	}
}

func TestSeed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if err := f.svc.Seed(ctx, 1, "/srv/dl/"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Seed(ctx, 1, "/srv/dl/"); err != nil {
		t.Fatal(err)
	}
	v, _ := f.svc.List(ctx)
	if len(v) != 3 || v[0].RenamePreset != 1 || v[1].EndDate == 0 || v[0].Path != "/srv/dl/linux-isos" || v[2].Path != "/srv/dl/datasets" {
		t.Fatalf("seeded = %d rules", len(v))
	}
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	if pv, _ := f.svc.Preview(ctx, v[0].ID); len(pv.Urls) != 1 || pv.Extraction.Error != "" {
		t.Errorf("seeded preview = %+v", pv)
	}
}

func TestHTTPFetcher(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "Bitswarm/") {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()
	fetch := HTTPFetcher()
	ctx := context.Background()
	if body, err := fetch(ctx, srv.URL+"/x"); err != nil || body != "<html>ok</html>" {
		t.Errorf("fetch = %q, %v", body, err)
	}
	if _, err := fetch(ctx, srv.URL+"/missing"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("404 err = %v", err)
	}
	if _, err := fetch(ctx, "ftp://example.org/x"); err == nil {
		t.Error("non-http URL should be rejected")
	}
	if _, err := fetch(ctx, "http://127.0.0.1:1/x"); err == nil {
		t.Error("unreachable host should fail")
	}
}

func TestHandler(t *testing.T) {
	f := newFixture(t)
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	app := fiber.New()
	NewHandler(f.svc).RegisterRoutes(app)
	body := `{"name":"R","url":"https://example.org/s?q=x","selector":"a.m","attr":"href","linkType":"magnet","max":5,"every":6,"unit":"hours","start":true,"skipSeen":true}`
	tests := []struct {
		name, method, path, body string
		want                     int
	}{
		{"list", http.MethodGet, "/schedules", "", 200},
		{"create", http.MethodPost, "/schedules", body, 201},
		{"create invalid", http.MethodPost, "/schedules", `{"name":"x"}`, 400},
		{"create bad json", http.MethodPost, "/schedules", `{`, 400},
		{"update", http.MethodPut, "/schedules/1", body, 200},
		{"update missing", http.MethodPut, "/schedules/9", body, 404},
		{"update bad id", http.MethodPut, "/schedules/x", body, 400},
		{"update bad json", http.MethodPut, "/schedules/1", `{`, 400},
		{"enable", http.MethodPatch, "/schedules/1/enabled", `{"enabled":true}`, 200},
		{"enable bad json", http.MethodPatch, "/schedules/1/enabled", `{`, 400},
		{"enable bad id", http.MethodPatch, "/schedules/x/enabled", `{}`, 400},
		{"preview", http.MethodGet, "/schedules/1/preview", "", 200},
		{"preview bad id", http.MethodGet, "/schedules/x/preview", "", 400},
		{"preview missing", http.MethodGet, "/schedules/9/preview", "", 404},
		{"download", http.MethodPost, "/schedules/1/download", `{"keys":["dddddddddddddddddddddddddddddddddddddddd"]}`, 200},
		{"download bad json", http.MethodPost, "/schedules/1/download", `{`, 400},
		{"download bad id", http.MethodPost, "/schedules/x/download", `{}`, 400},
		{"run", http.MethodPost, "/schedules/1/run", "", 200},
		{"run bad id", http.MethodPost, "/schedules/x/run", "", 400},
		{"run missing", http.MethodPost, "/schedules/9/run", "", 404},
		{"test", http.MethodPost, "/schedules/test", `{"url":"https://example.org/s","selector":"a"}`, 200},
		{"test invalid", http.MethodPost, "/schedules/test", `{}`, 400},
		{"test bad json", http.MethodPost, "/schedules/test", `{`, 400},
		{"log", http.MethodGet, "/schedules/log", "", 200},
		{"delete", http.MethodDelete, "/schedules/1", "", 204},
		{"delete missing", http.MethodDelete, "/schedules/1", "", 404},
		{"delete bad id", http.MethodDelete, "/schedules/x", "", 400},
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

func TestPathCheck(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	locked := true
	f.svc.SetPathCheck(func(p string) error {
		if locked {
			return errors.New("Bitswarm can’t write to " + p + ": permission denied.")
		}
		return nil
	})
	if _, err := f.svc.Create(ctx, goodRule()); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Create() into a locked folder err = %v", err)
	}
	locked = false
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	v, err := f.svc.Create(ctx, goodRule())
	if err != nil {
		t.Fatal(err)
	}
	locked = true
	res, _ := f.svc.Run(ctx, v.ID, "manual")
	if res.Status != "error" || !strings.Contains(res.LastError, "permission denied") {
		t.Errorf("run with a locked folder = %+v", res.Rule)
	}
	if ts, _ := f.tor.Snapshot(ctx); len(ts) != 0 {
		t.Errorf("run added %d torrents despite the folder check", len(ts))
	}
}

func TestRun_FailedAddIsRetriedNextRun(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.svc.SetFetcher(func(context.Context, string) (string, error) { return page, nil })
	f.tor.SetPathCheck(func(string) error { return errors.New("folder is read-only") })
	r := goodRule()
	v, err := f.svc.Create(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := f.svc.Run(ctx, v.ID, "manual")
	if first.SeenCount != 0 {
		t.Errorf("a link that failed to add was marked as seen (%d)", first.SeenCount)
	}
	log, _ := f.svc.Log(ctx)
	hasErr := false
	for _, e := range log {
		hasErr = hasErr || (e.Kind == "err" && strings.Contains(e.Text, "folder is read-only"))
	}
	if !hasErr {
		t.Errorf("log has no entry for the failed add: %+v", log)
	}

	f.tor.SetPathCheck(nil)
	second, _ := f.svc.Run(ctx, v.ID, "manual")
	if second.LastNew != 1 || second.SeenCount != 1 {
		t.Errorf("retry run = new %d seen %d, want 1 and 1", second.LastNew, second.SeenCount)
	}
}

func TestCreate_EmptyPathUsesSystemDefault(t *testing.T) {
	f := newFixture(t)
	r := goodRule()
	r.Path = ""
	v, err := f.svc.Create(context.Background(), r)
	if err != nil || v.Path != "" {
		t.Fatalf("Create() path = %q, err = %v, want empty path and no error", v.Path, err)
	}
}
