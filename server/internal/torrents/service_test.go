package torrents

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"bitswarm/server/internal/catalog"
	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/settings"
)

type memLog struct{ lines []string }

func (m *memLog) Add(_, _, msg string) { m.lines = append(m.lines, msg) }

type memBus struct{ events []string }

func (m *memBus) Emit(ev string, _ map[string]any) { m.events = append(m.events, ev) }

type fixedSettings struct{ s settings.Settings }

func (f *fixedSettings) Get() settings.Settings { return f.s }

type fixture struct {
	svc  *Service
	bus  *memBus
	log  *memLog
	set  *fixedSettings
	cat  *catalog.Service
	repo Repository
}

func newFixture(t *testing.T, seed bool) *fixture {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	cat, err := catalog.NewService(docstore.NewKV(conn))
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{bus: &memBus{}, log: &memLog{}, set: &fixedSettings{settings.Defaults()}, cat: cat, repo: NewRepository(conn)}
	f.svc = NewService(f.repo, f.set, cat, f.bus, f.log)
	f.svc.rnd = newRng(42).next
	if seed {
		if err := f.svc.Seed(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

const magnet = "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567&dn=My+Linux%20ISO.iso"

func TestSeed(t *testing.T) {
	f := newFixture(t, true)
	items, _ := f.svc.List(context.Background())
	if len(items) != 10 {
		t.Fatalf("seeded %d torrents, want 10", len(items))
	}
	if err := f.svc.Seed(context.Background()); err != nil {
		t.Fatal(err)
	}
	if again, _ := f.svc.List(context.Background()); len(again) != 10 {
		t.Errorf("second Seed() changed the count to %d", len(again))
	}
	if items[0].QueuePos == nil || *items[0].QueuePos != 1 || items[3].QueuePos != nil {
		t.Errorf("queue positions wrong: %v %v", items[0].QueuePos, items[3].QueuePos)
	}
}

func TestAdd(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		spec    AddSpec
		wantErr string
		check   func(t *testing.T, v View)
	}{
		{"nothing given", AddSpec{}, "Paste a magnet link or choose a .torrent file first.", nil},
		{"bad magnet", AddSpec{Magnet: "http://x"}, "This doesn’t look like a magnet link", nil},
		{"wrong file type", AddSpec{FileName: "x.zip"}, "That file isn’t a .torrent file", nil},
		{"no files selected", AddSpec{Magnet: magnet, Selected: []int{}}, "Select at least one file to download.", nil},
		{"magnet starts checking", AddSpec{Magnet: magnet, Start: true, Tags: []string{"new"}}, "", func(t *testing.T, v View) {
			if v.Name != "My Linux ISO.iso" || v.Status != StatusChecking || v.Hash != "0123456789abcdef0123456789abcdef01234567" {
				t.Errorf("got %+v", v.Torrent)
			}
		}},
		{"paused with a subset", AddSpec{FileName: "Show.torrent", Selected: []int{0}}, "", func(t *testing.T, v View) {
			if v.Status != StatusPaused || v.Files[0].Prio != "normal" || v.Files[1].Prio != "skip" {
				t.Errorf("status %s, files %+v", v.Status, v.Files)
			}
		}},
		{"skip hash check seeds", AddSpec{FileName: "a.torrent", Start: true, SkipHash: true}, "", func(t *testing.T, v View) {
			if v.Status != StatusSeeding || v.Progress != 1 {
				t.Errorf("status %s progress %v", v.Status, v.Progress)
			}
		}},
		{"top of queue and queued", AddSpec{FileName: "b.torrent", Top: true}, "", func(t *testing.T, v View) {
			if v.Status != StatusQueued || v.QueuePos == nil || *v.QueuePos != 1 {
				t.Errorf("status %s pos %v", v.Status, v.QueuePos)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, true)
			v, err := f.svc.Add(ctx, tc.spec)
			if tc.wantErr != "" {
				var inv Invalid
				if !errors.As(err, &inv) || !contains(inv.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, v)
			if !slices.Contains(f.bus.events, "torrent.added") {
				t.Error("torrent.added not emitted")
			}
		})
	}

	f := newFixture(t, true)
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet}); err == nil || !contains(err.Error(), "already in your list") {
		t.Errorf("duplicate err = %v", err)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func byName(t *testing.T, f *fixture, name string) Torrent {
	t.Helper()
	items, _ := f.svc.Snapshot(context.Background())
	for _, x := range items {
		if x.Name == name {
			return x
		}
	}
	t.Fatalf("no torrent %q", name)
	return Torrent{}
}

func TestBatch_Lifecycle(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	ubuntu := byName(t, f, "ubuntu-26.04-desktop-amd64.iso")
	sintel := byName(t, f, "Sintel (Blender Open Movie)")
	prel := byName(t, f, "Prelinger Archives — selected reels")
	done := byName(t, f, "Tears of Steel (1080p)")

	run := func(r BatchReq) string {
		t.Helper()
		msg, err := f.svc.Batch(ctx, r)
		if err != nil {
			t.Fatalf("%s: %v", r.Action, err)
		}
		return msg
	}

	if msg := run(BatchReq{Action: "pause", IDs: []int64{ubuntu.ID}}); msg != "Paused 1 torrent" {
		t.Errorf("pause msg = %q", msg)
	}
	if got := byName(t, f, ubuntu.Name); got.Status != StatusPaused {
		t.Errorf("status = %s", got.Status)
	}
	run(BatchReq{Action: "resume", IDs: []int64{ubuntu.ID, sintel.ID, prel.ID, done.ID}})
	if got := byName(t, f, prel.Name); got.Status != StatusDownloading || got.Err != "" {
		t.Errorf("errored torrent not resumed: %+v", got.Status)
	}
	if got := byName(t, f, done.Name); got.Status != StatusSeeding {
		t.Errorf("done torrent status = %s", got.Status)
	}

	run(BatchReq{Action: "force", IDs: []int64{sintel.ID}})
	if !byName(t, f, sintel.Name).Force {
		t.Error("force not set")
	}
	run(BatchReq{Action: "force", IDs: []int64{sintel.ID}})
	if byName(t, f, sintel.Name).Force {
		t.Error("force not cleared")
	}

	run(BatchReq{Action: "recheck", IDs: []int64{ubuntu.ID}})
	if got := byName(t, f, ubuntu.Name); got.Status != StatusRechecking || got.RecheckTo != StatusDownloading {
		t.Errorf("recheck: %s -> %s", got.Status, got.RecheckTo)
	}
	run(BatchReq{Action: "reannounce", IDs: []int64{ubuntu.ID}})
	run(BatchReq{Action: "seq", IDs: []int64{ubuntu.ID}})
	run(BatchReq{Action: "flp", IDs: []int64{ubuntu.ID}})
	if got := byName(t, f, ubuntu.Name); !got.Seq || !got.Flp {
		t.Error("seq/flp not set")
	}

	if _, err := f.svc.Batch(ctx, BatchReq{Action: "pause", IDs: []int64{999}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown id err = %v", err)
	}
	if _, err := f.svc.Batch(ctx, BatchReq{Action: "dance", IDs: []int64{ubuntu.ID}}); err == nil {
		t.Error("unknown action should fail")
	}

	run(BatchReq{Action: "remove", IDs: []int64{done.ID}, DeleteData: true})
	if all, _ := f.svc.List(ctx); len(all) != 9 {
		t.Errorf("after remove: %d torrents", len(all))
	}
	if !slices.Contains(f.bus.events, "torrent.removed") {
		t.Error("torrent.removed not emitted")
	}
}

func TestBatch_TagsMoveLimits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	a := byName(t, f, "archlinux-2026.10.01-x86_64.iso")
	on := true

	steps := []struct {
		name string
		req  BatchReq
		err  string
	}{
		{"tag toggle on", BatchReq{Action: "tags", How: "toggle", Tag: "fav", On: &on, IDs: []int64{a.ID}}, ""},
		{"tag add many", BatchReq{Action: "tags", How: "add", Tags: []string{"x", "y"}, IDs: []int64{a.ID}}, ""},
		{"tag add none", BatchReq{Action: "tags", How: "add", IDs: []int64{a.ID}}, "Enter a tag name."},
		{"move ok", BatchReq{Action: "move", Path: "/mnt/data/", IDs: []int64{a.ID}}, ""},
		{"move relative", BatchReq{Action: "move", Path: "relative", IDs: []int64{a.ID}}, "Enter a full folder path"},
		{"limits ok", BatchReq{Action: "limits", IDs: []int64{a.ID}, DlLimit: intp(100), UlLimit: intp(50), RatioMode: "custom", Ratio: 1.5}, ""},
		{"limits bad dl", BatchReq{Action: "limits", IDs: []int64{a.ID}, DlLimit: intp(-3)}, "download limit"},
		{"limits bad ratio", BatchReq{Action: "limits", IDs: []int64{a.ID}, RatioMode: "custom", Ratio: 0}, "ratio must be"},
	}
	for _, st := range steps {
		_, err := f.svc.Batch(ctx, st.req)
		if st.err == "" && err != nil {
			t.Fatalf("%s: %v", st.name, err)
		}
		if st.err != "" && (err == nil || !contains(err.Error(), st.err)) {
			t.Fatalf("%s: err = %v, want %q", st.name, err, st.err)
		}
	}
	got := byName(t, f, a.Name)
	if !slices.Equal(got.Tags, []string{"fav", "x", "y"}) || got.Path != "/mnt/data" ||
		got.DlLimit != 100 || got.UlLimit != 50 || got.RatioLimit == nil || *got.RatioLimit != 1.5 {
		t.Errorf("got %+v", got)
	}
	if !slices.Contains(f.cat.TagNames(), "fav") {
		t.Error("catalog not updated")
	}

	f.svc.Batch(ctx, BatchReq{Action: "tags", How: "toggle", Tag: "fav", IDs: []int64{a.ID}})
	f.svc.Batch(ctx, BatchReq{Action: "tags", How: "clear", IDs: []int64{a.ID}})
	if msg, _ := f.svc.Batch(ctx, BatchReq{Action: "tags", How: "prune", IDs: []int64{a.ID}}); msg != "Deleted 3 unused tags" {
		t.Errorf("prune msg = %q", msg)
	}
	f.svc.Batch(ctx, BatchReq{Action: "limits", IDs: []int64{a.ID}, RatioMode: "none"})
	f.svc.Batch(ctx, BatchReq{Action: "limits", IDs: []int64{a.ID}})
	if got := byName(t, f, a.Name); got.RatioLimit != nil {
		t.Error("global ratio mode should clear the limit")
	}
}

func intp(v int) *int { return &v }

func TestBatch_Queue(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	order := func() []string {
		items, _ := f.svc.List(ctx)
		slots := make([]string, 11)
		n := 0
		for _, v := range items {
			if v.QueuePos != nil {
				slots[*v.QueuePos] = v.Name
				n++
			}
		}
		return slots[1 : n+1]
	}
	libre := byName(t, f, "LibreOffice_26.2.1_Win_x86-64.msi")
	done := byName(t, f, "Tears of Steel (1080p)")

	if _, err := f.svc.Batch(ctx, BatchReq{Action: "queue", How: "top", IDs: []int64{done.ID}}); err == nil {
		t.Error("finished torrents have no queue place")
	}
	if _, err := f.svc.Batch(ctx, BatchReq{Action: "queue", How: "sideways", IDs: []int64{libre.ID}}); err == nil {
		t.Error("unknown move should fail")
	}
	f.svc.Batch(ctx, BatchReq{Action: "queue", How: "top", IDs: []int64{libre.ID}})
	if got := order(); got[0] != libre.Name {
		t.Errorf("top: %v", got)
	}
	f.svc.Batch(ctx, BatchReq{Action: "queue", How: "down", IDs: []int64{libre.ID}})
	if got := order(); got[1] != libre.Name {
		t.Errorf("down: %v", got)
	}
	f.svc.Batch(ctx, BatchReq{Action: "queue", How: "up", IDs: []int64{libre.ID}})
	if got := order(); got[0] != libre.Name {
		t.Errorf("up: %v", got)
	}
	f.svc.Batch(ctx, BatchReq{Action: "queue", How: "bottom", IDs: []int64{libre.ID}})
	if got := order(); got[len(got)-1] != libre.Name {
		t.Errorf("bottom: %v", got)
	}
}

func TestSetPriorityAndRetry(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	bbb := byName(t, f, "Big Buck Bunny (2160p, 60fps)")
	sintel := byName(t, f, "Sintel (Blender Open Movie)")
	prel := byName(t, f, "Prelinger Archives — selected reels")

	if err := f.svc.SetPriority(ctx, sintel.ID, 1, "skip"); err != nil {
		t.Fatal(err)
	}
	if got := byName(t, f, sintel.Name); got.Files[1].Prio != "skip" || got.Wanted() >= sintel.Wanted() {
		t.Error("skipping did not shrink the wanted size")
	}
	if err := f.svc.SetPriority(ctx, sintel.ID, 9, "high"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad index err = %v", err)
	}
	if err := f.svc.SetPriority(ctx, sintel.ID, 0, "urgent"); err == nil {
		t.Error("bad priority should fail")
	}
	if err := f.svc.SetPriority(ctx, 99, 0, "high"); !errors.Is(err, ErrNotFound) {
		t.Errorf("bad id err = %v", err)
	}
	if err := f.svc.SetPriority(ctx, bbb.ID, 0, "high"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Retry(ctx, prel.ID); err != nil {
		t.Fatal(err)
	}
	if got := byName(t, f, prel.Name); got.Status != StatusDownloading || got.Seeds != 37 || got.Err != "" {
		t.Errorf("retry: %+v", got.Status)
	}
	if err := f.svc.Retry(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("retry missing err = %v", err)
	}
}

func TestTick(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	f.set.s.MaxActiveDl = 4
	before := byName(t, f, "debian-13.2.0-amd64-DVD-1.iso")
	for i := 0; i < 3; i++ {
		if err := f.svc.Tick(ctx); err != nil {
			t.Fatal(err)
		}
	}
	after := byName(t, f, before.Name)
	if after.Done <= before.Done || len(after.Hist.Dl) == 0 {
		t.Errorf("no progress: %v -> %v", before.Done, after.Done)
	}
	if got := byName(t, f, "LibreOffice_26.2.1_Win_x86-64.msi"); got.Status == StatusQueued {
		t.Error("queued torrent should have started")
	}
	st, err := f.svc.Stats(ctx)
	if err != nil || st.Torrents != 10 || st.Active == 0 || st.SessDl <= 0 || len(st.Hist.Dl) < 60 {
		t.Errorf("stats = %+v, %v", st, err)
	}
}

func TestTick_StatesAndLimits(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, false)
	mk := func(name string, st Status, left int) int64 {
		v, err := f.svc.Add(ctx, AddSpec{Name: name, Size: 100 * MiB, Start: true})
		if err != nil {
			t.Fatal(err)
		}
		f.svc.Mutate(ctx, func(ts []Torrent) {
			for i := range ts {
				if ts[i].ID == v.ID {
					ts[i].Status, ts[i].CheckLeft, ts[i].Target = st, left, 5*MiB
				}
			}
		})
		return v.ID
	}
	meta := mk("meta", StatusChecking, 1)
	re := mk("re", StatusRechecking, 1)
	dl := mk("dl", StatusDownloading, 0)
	f.svc.Tick(ctx)
	byID := func(id int64) Torrent {
		x, _ := f.repo.Get(ctx, id)
		return x
	}
	if byID(meta).Status != StatusDownloading || !slices.Contains(f.bus.events, "torrent.metadata") {
		t.Errorf("metadata step: %s", byID(meta).Status)
	}
	if byID(re).Status != StatusDownloading || len(f.log.lines) == 0 {
		t.Errorf("recheck step: %s", byID(re).Status)
	}

	f.set.s.DlLimit = 1
	f.svc.Tick(ctx)
	if got := byID(dl).Dl; got > 1.01*KiB*float64(len(f.log.lines)+3) {
		t.Errorf("global cap ignored: %v", got)
	}
	f.svc.Mutate(ctx, func(ts []Torrent) {
		for i := range ts {
			ts[i].Done = ts[i].Size - 1
			ts[i].Dl = 10
		}
	})
	f.set.s.DlLimit = 0
	f.svc.Tick(ctx)
	if !slices.Contains(f.bus.events, "torrent.completed") {
		t.Error("completion not emitted")
	}
}

func TestTick_SeedLimits(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		action    string
		wantLeft  int
		wantState Status
		event     string
	}{
		{"pause", 1, StatusDone, "torrent.paused"},
		{"remove", 0, "", "torrent.removed"},
		{"removeData", 0, "", "torrent.removed"},
		{"superseed", 1, StatusSeeding, ""},
	} {
		t.Run(tc.action, func(t *testing.T) {
			f := newFixture(t, false)
			f.set.s.RatioOn, f.set.s.Ratio, f.set.s.SeedAction = true, 1, tc.action
			v, _ := f.svc.Add(ctx, AddSpec{Name: "s", Size: 10 * MiB, Start: true, SkipHash: true})
			f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].Uploaded = ts[0].Size * 5 })
			f.svc.Tick(ctx)
			left, _ := f.svc.List(ctx)
			if len(left) != tc.wantLeft {
				t.Fatalf("%d torrents left, want %d", len(left), tc.wantLeft)
			}
			if tc.wantLeft == 1 && (left[0].Status != tc.wantState || (tc.action == "superseed") != left[0].Superseed) {
				t.Errorf("status %s superseed %v (id %d)", left[0].Status, left[0].Superseed, v.ID)
			}
			if tc.event != "" && !slices.Contains(f.bus.events, tc.event) {
				t.Errorf("missing %s in %v", tc.event, f.bus.events)
			}
		})
	}

	f := newFixture(t, false)
	f.set.s.SeedTimeOn, f.set.s.SeedTime = true, 1
	f.svc.Add(ctx, AddSpec{Name: "t", Size: 10 * MiB, Start: true, SkipHash: true})
	f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].SeedSince = time.Now().Add(-2 * time.Minute).UnixMilli() })
	f.svc.Tick(ctx)
	if v, _ := f.svc.List(ctx); v[0].Status != StatusDone {
		t.Errorf("seed time limit ignored: %s", v[0].Status)
	}

	f = newFixture(t, false)
	f.svc.Add(ctx, AddSpec{Name: "own", Size: 10 * MiB, Start: true, SkipHash: true})
	f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].Uploaded = ts[0].Size * 3; ts[0].RatioLimit = ptr(2.0) })
	f.svc.Tick(ctx)
	if v, _ := f.svc.List(ctx); v[0].Status != StatusDone {
		t.Errorf("per-torrent ratio limit ignored: %s", v[0].Status)
	}
	f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].Status = StatusSeeding; ts[0].RatioLimit = ptr(-1.0) })
	f.svc.Tick(ctx)
	if v, _ := f.svc.List(ctx); v[0].Status != StatusSeeding {
		t.Error("ratio limit of -1 should mean no limit")
	}
}

func TestAlt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	f.svc.SetAlt(true)
	if st, _ := f.svc.Stats(ctx); !st.Alt || !st.AltOn {
		t.Errorf("alt not on: %+v", st)
	}
}

type fakeRenamer struct{ whens []string }

func (r *fakeRenamer) ApplyRenames(_ context.Context, t *Torrent, when string) {
	r.whens = append(r.whens, when)
	t.Name += "!"
}

func TestRenamerHook(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, false)
	fr := &fakeRenamer{}
	f.svc.SetRenamer(fr)
	v, err := f.svc.Add(ctx, AddSpec{Name: "x", Size: 5 * MiB, Start: true})
	if err != nil || v.Name != "x!" {
		t.Fatalf("Add() = %v, %v", v.Name, err)
	}
	f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].Status, ts[0].Done = StatusDownloading, ts[0].Size-1; ts[0].Dl = 10 })
	f.svc.Tick(ctx)
	if !slices.Equal(fr.whens, []string{"add", "complete"}) {
		t.Errorf("renamer calls = %v", fr.whens)
	}
}

func TestPathCheck_BlocksAddAndMove(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, true)
	var checked []string
	f.svc.SetPathCheck(func(p string) error {
		checked = append(checked, p)
		if strings.HasPrefix(p, "/locked") {
			return errors.New("Bitswarm can’t write to " + p + ": permission denied.")
		}
		return nil
	})

	_, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Path: "/locked/dl"})
	var inv Invalid
	if !errors.As(err, &inv) || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("Add into a locked folder err = %v", err)
	}
	if items, _ := f.svc.List(ctx); len(items) != 10 {
		t.Errorf("a failed folder check still added a torrent (%d)", len(items))
	}
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet}); err != nil {
		t.Fatalf("Add with the default folder: %v", err)
	}
	if checked[len(checked)-1] != "/downloads" {
		t.Errorf("default save path not checked: %v", checked)
	}

	a := byName(t, f, "archlinux-2026.10.01-x86_64.iso")
	if _, err := f.svc.Batch(ctx, BatchReq{Action: "move", IDs: []int64{a.ID}, Path: "/locked/x"}); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("move into a locked folder err = %v", err)
	}
	if got := byName(t, f, a.Name); got.Path == "/locked/x" {
		t.Error("a failed move changed the path")
	}
	if _, err := f.svc.Batch(ctx, BatchReq{Action: "move", IDs: []int64{a.ID}, Path: "/mnt/ok"}); err != nil {
		t.Errorf("move into a good folder: %v", err)
	}
}
