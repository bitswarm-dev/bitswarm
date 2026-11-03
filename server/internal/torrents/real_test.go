package torrents

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

type fakeEngine struct {
	meta      Meta
	hash      string
	live      map[string]Live
	addErr    error
	restore   error
	added     []AddOptions
	unloaded  []string
	forgot    map[string]bool
	restored  []string
	names     map[string][]string
	moved     map[string]string
	prios     map[string][]string
	rates     [2]float64
	reannounc []string
	liveCalls int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{live: map[string]Live{}, forgot: map[string]bool{}, names: map[string][]string{}, moved: map[string]string{}, prios: map[string][]string{}}
}

func (f *fakeEngine) Name() string                { return "Fake" }
func (f *fakeEngine) Port() int                   { return 51413 }
func (f *fakeEngine) Hash(Source) (string, error) { return f.hash, nil }
func (f *fakeEngine) Meta(context.Context, Source) (Meta, error) {
	return f.meta, nil
}
func (f *fakeEngine) Add(_ Source, o AddOptions) (Meta, error) {
	f.added = append(f.added, o)
	if f.addErr != nil {
		return Meta{}, f.addErr
	}
	f.live[f.hash] = Live{HaveInfo: f.meta.Name != "", Meta: f.meta}
	return f.meta, nil
}
func (f *fakeEngine) Restore(hash string, _ RestoreOptions) error {
	f.restored = append(f.restored, hash)
	if f.restore != nil {
		return f.restore
	}
	if _, ok := f.live[hash]; !ok {
		f.live[hash] = Live{HaveInfo: true, Meta: f.meta}
	}
	return nil
}
func (f *fakeEngine) Live(hash string) (Live, bool) {
	f.liveCalls++
	l, ok := f.live[hash]
	return l, ok
}
func (f *fakeEngine) Loaded(hash string) bool { _, ok := f.live[hash]; return ok }
func (f *fakeEngine) Unload(hash string) {
	f.unloaded = append(f.unloaded, hash)
	delete(f.live, hash)
}
func (f *fakeEngine) Forget(hash string, del bool) error {
	f.forgot[hash] = del
	delete(f.live, hash)
	return nil
}
func (f *fakeEngine) Recheck(string) error { return nil }
func (f *fakeEngine) SetPriorities(hash string, p []string) {
	f.prios[hash] = p
}
func (f *fakeEngine) SetNames(hash string, n []string) ([]string, error) {
	f.names[hash] = n
	return n, nil
}
func (f *fakeEngine) Move(hash, dir string, _ bool) error { f.moved[hash] = dir; return nil }
func (f *fakeEngine) Reannounce(hash string)              { f.reannounc = append(f.reannounc, hash) }
func (f *fakeEngine) SetRates(dl, ul float64)             { f.rates = [2]float64{dl, ul} }
func (f *fakeEngine) Totals() EngineTotals                { return EngineTotals{Conns: 7, DHTNodes: 90} }
func (f *fakeEngine) Close() error                        { return nil }

const realHash = "0123456789abcdef0123456789abcdef01234567"

func realFixture(t *testing.T) (*fixture, *fakeEngine) {
	t.Helper()
	f := newFixture(t, false)
	eng := newFakeEngine()
	eng.hash = realHash
	eng.meta = Meta{Hash: realHash, Name: "Album", Size: 3000, PieceSize: 1000, Trackers: []string{"http://t/announce"},
		Files: []File{{Name: "Album/a.flac", Size: 2000, Prio: "normal"}, {Name: "Album/b.flac", Size: 1000, Prio: "normal"}}}
	f.svc.SetEngine(eng, func() float64 { return 1234 })
	return f, eng
}

func TestService_AddReal(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		spec    AddSpec
		prep    func(*fixture, *fakeEngine)
		wantErr string
		status  Status
		files   int
	}{
		{"nothing given", AddSpec{}, nil, "Paste a magnet link", "", 0},
		{"bad file type", AddSpec{FileData: []byte("x"), FileName: "x.zip"}, nil, "isn’t a .torrent file", "", 0},
		{"sequential refused", AddSpec{Magnet: magnet, Seq: true}, nil, "aren’t supported", "", 0},
		{"no files selected", AddSpec{Magnet: magnet, Selected: []int{}}, nil, "Select at least one file", "", 0},
		{"file with metadata starts downloading", AddSpec{FileData: []byte("x"), FileName: "a.torrent", Start: true}, nil, "", StatusDownloading, 2},
		{"magnet without metadata waits", AddSpec{Magnet: magnet, Start: true}, func(_ *fixture, e *fakeEngine) { e.meta = Meta{Hash: realHash} }, "", StatusChecking, 0},
		{"not started stays paused", AddSpec{Magnet: magnet, Start: false}, nil, "", StatusPaused, 2},
		{"queue is full", AddSpec{Magnet: magnet, Start: true}, func(f *fixture, _ *fakeEngine) {
			f.set.s.MaxActiveDl = 1
			if _, err := f.repo.Create(ctx, Torrent{Name: "busy", Status: StatusDownloading, Hash: "ff"}); err != nil {
				t.Fatal(err)
			}
		}, "", StatusQueued, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, eng := realFixture(t)
			if tc.prep != nil {
				tc.prep(f, eng)
			}
			tc.spec.Path = t.TempDir()
			v, err := f.svc.Add(ctx, tc.spec)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Add() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if v.Status != tc.status || len(v.Files) != tc.files || v.Hash != realHash {
				t.Errorf("Add() = status %s, %d files, hash %s; want %s, %d files", v.Status, len(v.Files), v.Hash, tc.status, tc.files)
			}
			if !slices.Contains(f.bus.events, "torrent.added") {
				t.Errorf("events = %v, want torrent.added", f.bus.events)
			}
			if tc.status == StatusPaused || tc.status == StatusQueued {
				if len(eng.unloaded) == 0 {
					t.Error("an inactive torrent stayed loaded in the engine")
				}
			}
		})
	}
}

func TestService_AddReal_Duplicate(t *testing.T) {
	ctx := context.Background()
	f, _ := realFixture(t)
	spec := AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}
	if _, err := f.svc.Add(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Add(ctx, spec); err == nil || !strings.Contains(err.Error(), "already in your list") {
		t.Errorf("second Add() error = %v, want duplicate error", err)
	}
}

func TestService_AddReal_Selected(t *testing.T) {
	f, eng := realFixture(t)
	v, err := f.svc.Add(context.Background(), AddSpec{Magnet: magnet, Start: true, Path: t.TempDir(), Selected: []int{1}})
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{v.Files[0].Prio, v.Files[1].Prio}; got[0] != "skip" || got[1] != "normal" {
		t.Errorf("priorities = %v, want [skip normal]", got)
	}
	if len(eng.added) != 1 || len(eng.added[0].Prios) != 2 {
		t.Errorf("engine got %+v", eng.added)
	}
}

func TestService_AddReal_EngineError(t *testing.T) {
	f, eng := realFixture(t)
	eng.addErr = errors.New("boom")
	if _, err := f.svc.Add(context.Background(), AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err == nil {
		t.Error("Add() expected the engine error")
	}
	if items, _ := f.svc.List(context.Background()); len(items) != 0 {
		t.Errorf("a failed add left %d torrents behind", len(items))
	}
}

func TestService_TickReal(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	eng.live[realHash] = Live{HaveInfo: true, Meta: eng.meta, FileDone: []float64{1000, 0}, Dl: 500, DlDelta: 500, Seeds: 3, Leech: 1}
	if err := f.svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	v, _ := f.svc.List(ctx)
	if v[0].Status != StatusDownloading || v[0].Done != 1000 || v[0].Seeds != 3 || v[0].Dl != 500 {
		t.Errorf("mid-download = %s done %v seeds %d dl %v", v[0].Status, v[0].Done, v[0].Seeds, v[0].Dl)
	}
	if v[0].Files[0].Done != 1000 {
		t.Errorf("per-file done = %v, want 1000", v[0].Files[0].Done)
	}

	eng.live[realHash] = Live{HaveInfo: true, Meta: eng.meta, FileDone: []float64{2000, 1000}, UlDelta: 40}
	if err := f.svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	v, _ = f.svc.List(ctx)
	if v[0].Status != StatusSeeding || v[0].Uploaded != 40 {
		t.Errorf("finished = %s uploaded %v, want seeding 40", v[0].Status, v[0].Uploaded)
	}
	if !slices.Contains(f.bus.events, "torrent.completed") {
		t.Errorf("events = %v, want torrent.completed", f.bus.events)
	}
}

func TestService_TickReal_Metadata(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	full := eng.meta
	eng.meta = Meta{Hash: realHash}
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	eng.live[realHash] = Live{}
	_ = f.svc.Tick(ctx)
	if v, _ := f.svc.List(ctx); v[0].Status != StatusChecking {
		t.Fatalf("status before metadata = %s, want checking", v[0].Status)
	}
	eng.live[realHash] = Live{HaveInfo: true, Meta: full}
	_ = f.svc.Tick(ctx)
	v, _ := f.svc.List(ctx)
	if v[0].Name != "Album" || len(v[0].Files) != 2 || v[0].Status != StatusDownloading || v[0].Size != 3000 {
		t.Errorf("after metadata = %s %q %d files size %v", v[0].Status, v[0].Name, len(v[0].Files), v[0].Size)
	}
	if !slices.Contains(f.bus.events, "torrent.metadata") {
		t.Errorf("events = %v, want torrent.metadata", f.bus.events)
	}
}

func TestService_TickReal_Lifecycle(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	v, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}

	delete(eng.live, realHash)
	_ = f.svc.Tick(ctx)
	if !slices.Contains(eng.restored, realHash) {
		t.Error("an active torrent that wasn't loaded was not restored into the engine")
	}

	if _, err := f.svc.Batch(ctx, BatchReq{Action: "pause", IDs: []int64{v.ID}}); err != nil {
		t.Fatal(err)
	}
	eng.unloaded = nil
	_ = f.svc.Tick(ctx)
	if !slices.Contains(eng.unloaded, realHash) {
		t.Error("a paused torrent stayed loaded in the engine")
	}

	eng.restore = errors.New("folder is missing")
	if _, err := f.svc.Batch(ctx, BatchReq{Action: "resume", IDs: []int64{v.ID}}); err != nil {
		t.Fatal(err)
	}
	_ = f.svc.Tick(ctx)
	items, _ := f.svc.List(ctx)
	if items[0].Status != StatusError || !strings.Contains(items[0].Err, "folder is missing") {
		t.Errorf("failed restore = %s %q, want error status with message", items[0].Status, items[0].Err)
	}
	if !slices.Contains(f.bus.events, "torrent.error") {
		t.Errorf("events = %v, want torrent.error", f.bus.events)
	}
}

func TestService_TickReal_DiskError(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	eng.live[realHash] = Live{HaveInfo: true, Meta: eng.meta, Err: "Writing to the download folder failed: no space left"}
	_ = f.svc.Tick(ctx)
	items, _ := f.svc.List(ctx)
	if items[0].Status != StatusError || !strings.Contains(items[0].Err, "no space left") {
		t.Errorf("disk error = %s %q", items[0].Status, items[0].Err)
	}
}

func TestService_TickReal_Rates(t *testing.T) {
	f, eng := realFixture(t)
	f.set.s.DlLimit, f.set.s.UlLimit = 100, 50
	_ = f.svc.Tick(context.Background())
	if eng.rates != [2]float64{100 * KiB, 50 * KiB} {
		t.Errorf("rates = %v, want 100 KiB/s down and 50 KiB/s up", eng.rates)
	}
	f.svc.SetAlt(true)
	_ = f.svc.Tick(context.Background())
	if eng.rates[0] != float64(f.set.s.AltDl)*KiB {
		t.Errorf("alt rates = %v, want alt download limit", eng.rates)
	}
}

func TestService_BatchReal(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	v, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{v.ID}
	tests := []struct {
		name    string
		req     BatchReq
		wantErr string
	}{
		{"per-torrent speed limit refused", BatchReq{Action: "limits", IDs: ids, DlLimit: ptr(500)}, "aren’t supported"},
		{"clearing a limit is fine", BatchReq{Action: "limits", IDs: ids, DlLimit: ptr(0)}, ""},
		{"sequential refused", BatchReq{Action: "seq", IDs: ids}, "aren’t supported"},
		{"reannounce", BatchReq{Action: "reannounce", IDs: ids}, ""},
		{"move", BatchReq{Action: "move", IDs: ids, Path: t.TempDir()}, ""},
		{"remove deletes data", BatchReq{Action: "remove", IDs: ids, DeleteData: true}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.svc.Batch(ctx, tc.req)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Batch() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Batch() error = %v", err)
			}
		})
	}
	if len(eng.reannounc) != 1 || eng.moved[realHash] == "" || !eng.forgot[realHash] {
		t.Errorf("engine calls: reannounce %v, moved %q, forgot-with-delete %v", eng.reannounc, eng.moved[realHash], eng.forgot[realHash])
	}
}

func TestService_SetPriorityReal(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	v, _ := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()})
	if err := f.svc.SetPriority(ctx, v.ID, 0, "skip"); err != nil {
		t.Fatal(err)
	}
	if got := eng.prios[realHash]; len(got) != 2 || got[0] != "skip" {
		t.Errorf("engine priorities = %v, want [skip normal]", got)
	}
}

func TestService_MutateRenamesFiles(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	err := f.svc.Mutate(ctx, func(ts []Torrent) { ts[0].Files[0].Name = "Album/first.flac" })
	if err != nil {
		t.Fatal(err)
	}
	if got := eng.names[realHash]; len(got) != 2 || got[0] != "Album/first.flac" {
		t.Errorf("engine names = %v, want the renamed first file", got)
	}
}

func TestService_StatsReal(t *testing.T) {
	f, _ := realFixture(t)
	st, err := f.svc.Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Real || st.Engine != "Fake" || st.Port != 51413 || st.Conns != 7 || st.Dht != 90 || st.Free != 1234 {
		t.Errorf("Stats() = %+v", st)
	}
	if f.svc.Free() != 1234 {
		t.Errorf("Free() = %v, want 1234", f.svc.Free())
	}
	if f.svc.EngineName() != "Fake" {
		t.Errorf("EngineName() = %q", f.svc.EngineName())
	}
}

func TestService_EngineName_Simulated(t *testing.T) {
	f := newFixture(t, false)
	if got := f.svc.EngineName(); got != "Simulated engine" {
		t.Errorf("EngineName() = %q", got)
	}
	if f.svc.Real() {
		t.Error("Real() = true without an engine")
	}
}

func TestFetchTorrent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok.torrent":
			_, _ = w.Write([]byte("d4:infod4:name1:xee"))
		case "/big.torrent":
			_, _ = w.Write([]byte(strings.Repeat("x", maxTorrentFile+10)))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{"ok", srv.URL + "/ok.torrent", ""},
		{"not found", srv.URL + "/missing.torrent", "404"},
		{"too large", srv.URL + "/big.torrent", "larger than 8 MB"},
		{"not http", "ftp://example.com/a.torrent", "isn’t a web address"},
		{"unreachable", "http://127.0.0.1:1/a.torrent", "Couldn’t download"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, err := fetchTorrent(context.Background(), tc.url)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("fetchTorrent() error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || len(data) == 0 {
				t.Fatalf("fetchTorrent() = %d bytes, err %v", len(data), err)
			}
		})
	}
}

func TestService_TickReal_ReadsLiveOncePerTorrent(t *testing.T) {
	ctx := context.Background()
	f, eng := realFixture(t)
	if _, err := f.svc.Add(ctx, AddSpec{Magnet: magnet, Start: true, Path: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	eng.live[realHash] = Live{HaveInfo: true, Meta: eng.meta, FileDone: []float64{1000, 0}, Dl: 500, DlDelta: 500}
	eng.liveCalls = 0
	if err := f.svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if eng.liveCalls != 1 {
		t.Errorf("Live called %d times in one tick, want 1 (each call consumes the byte counters)", eng.liveCalls)
	}
	st, _ := f.svc.Stats(ctx)
	if st.SessDl != 500 || st.Hist.Dl[len(st.Hist.Dl)-1] != 500 {
		t.Errorf("sessDl = %v, last sample = %v, want 500", st.SessDl, st.Hist.Dl[len(st.Hist.Dl)-1])
	}
}
