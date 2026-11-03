package engine

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"

	"bitswarm/server/internal/torrents"
)

func newEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(Config{DataDir: t.TempDir(), Port: 0, DHT: false, Version: "0.9.0"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

type seeded struct {
	cl      *torrent.Client
	mi      *metainfo.MetaInfo
	raw     []byte
	payload map[string][]byte
}

func seed(t *testing.T) seeded {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "Album")
	payload := map[string][]byte{}
	for _, f := range []struct {
		name string
		size int
	}{{"one.bin", 700 * 1024}, {"sub/two.bin", 300 * 1024}} {
		b := make([]byte, f.size)
		rand.Read(b)
		payload[f.name] = b
		p := filepath.Join(dir, filepath.FromSlash(f.name))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, b, 0o644)
	}
	info := metainfo.Info{PieceLength: 64 * 1024}
	if err := info.BuildFromFilePath(dir); err != nil {
		t.Fatal(err)
	}
	ib, _ := bencodeMarshal(info)
	mi := &metainfo.MetaInfo{InfoBytes: ib}
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = root
	cfg.NoDHT = true
	cfg.Seed = true
	cfg.ListenPort = 0
	cfg.DefaultStorage = storage.NewFileOpts(storage.NewFileClientOpts{ClientBaseDir: root, PieceCompletion: storage.NewMapPieceCompletion()})
	cl, err := torrent.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cl.Close() })
	st, err := cl.AddTorrent(mi)
	if err != nil {
		t.Fatal(err)
	}
	<-st.GotInfo()
	st.VerifyData()
	var buf bytes.Buffer
	mi.Write(&buf)
	return seeded{cl: cl, mi: mi, raw: buf.Bytes(), payload: payload}
}

func waitFor(t *testing.T, what string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestEngine_DownloadFromPeer(t *testing.T) {
	s := seed(t)
	e := newEngine(t)
	dir := t.TempDir()

	meta, err := e.Meta(context.Background(), torrents.Source{Torrent: s.raw})
	if err != nil {
		t.Fatalf("Meta() error = %v", err)
	}
	if len(meta.Files) != 2 || meta.Name != "Album" {
		t.Fatalf("Meta() = %+v, want 2 files named Album", meta)
	}
	if _, err := e.Add(torrents.Source{Torrent: s.raw}, torrents.AddOptions{Dir: dir, Layout: "original"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := e.Add(torrents.Source{Torrent: s.raw}, torrents.AddOptions{Dir: dir}); err == nil {
		t.Error("Add() of the same torrent twice expected an error")
	}
	en, _ := e.get(meta.Hash)
	en.t.AddPeers([]torrent.PeerInfo{{Addr: ipPort(s.cl.LocalPort()), Trusted: true}})

	waitFor(t, "download to finish", func() bool {
		l, ok := e.Live(meta.Hash)
		if !ok || !l.HaveInfo {
			return false
		}
		var sum float64
		for _, d := range l.FileDone {
			sum += d
		}
		return sum >= meta.Size
	})
	for name, want := range s.payload {
		got, err := os.ReadFile(filepath.Join(dir, "Album", filepath.FromSlash(name)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("file %s mismatch (err=%v)", name, err)
		}
	}
	if _, err := e.SetNames(meta.Hash, []string{"Album/first.bin", "Album/sub/second.bin"}); err != nil {
		t.Fatalf("SetNames() error = %v", err)
	}
	for _, n := range []string{"first.bin", "sub/second.bin"} {
		if _, err := os.Stat(filepath.Join(dir, "Album", filepath.FromSlash(n))); err != nil {
			t.Errorf("renamed file %s missing: %v", n, err)
		}
	}
	final, err := e.SetNames(meta.Hash, []string{"Album/first.bin", "Album/first.bin"})
	if err != nil || final[0] != "Album/first.bin" || final[1] != "Album/first (2).bin" {
		t.Fatalf("SetNames() onto an existing file = %v, %v", final, err)
	}
	for n, want := range map[string][]byte{"first.bin": s.payload["one.bin"], "first (2).bin": s.payload["sub/two.bin"]} {
		got, err := os.ReadFile(filepath.Join(dir, "Album", n))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s after the numbered rename: err=%v, overwritten=%v", n, err, !bytes.Equal(got, want))
		}
	}
	if _, err := e.SetNames(meta.Hash, []string{"Album/first.bin", "Album/sub/second.bin"}); err != nil {
		t.Fatalf("SetNames() back error = %v", err)
	}
	waitFor(t, "complete state after rename", func() bool {
		l, ok := e.Live(meta.Hash)
		if !ok || !l.HaveInfo {
			return false
		}
		return l.FileDone[0]+l.FileDone[1] >= meta.Size
	})

	newDir := t.TempDir()
	if err := e.Move(meta.Hash, newDir, true); err != nil {
		t.Fatalf("Move() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "Album", "first.bin")); err != nil {
		t.Errorf("moved file missing: %v", err)
	}
	if err := e.Forget(meta.Hash, true); err != nil {
		t.Fatalf("Forget() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(newDir, "Album", "first.bin")); err == nil {
		t.Error("Forget(deleteData) left the file behind")
	}
}

func TestEngine_SkipFile(t *testing.T) {
	s := seed(t)
	e := newEngine(t)
	dir := t.TempDir()
	meta, err := e.Add(torrents.Source{Torrent: s.raw}, torrents.AddOptions{Dir: dir, Layout: "original", Prios: []string{"skip", "normal"}})
	if err != nil {
		t.Fatal(err)
	}
	en, _ := e.get(meta.Hash)
	en.t.AddPeers([]torrent.PeerInfo{{Addr: ipPort(s.cl.LocalPort()), Trusted: true}})
	waitFor(t, "second file", func() bool {
		l, ok := e.Live(meta.Hash)
		return ok && l.HaveInfo && l.FileDone[1] >= meta.Files[1].Size
	})
	l, _ := e.Live(meta.Hash)
	if l.FileDone[0] > 64*1024 {
		t.Errorf("skipped file downloaded %v bytes", l.FileDone[0])
	}
}

func TestEngine_Restore(t *testing.T) {
	s := seed(t)
	e := newEngine(t)
	dir := t.TempDir()
	meta, _ := e.Add(torrents.Source{Torrent: s.raw}, torrents.AddOptions{Dir: dir, Layout: "original"})
	en, _ := e.get(meta.Hash)
	en.t.AddPeers([]torrent.PeerInfo{{Addr: ipPort(s.cl.LocalPort()), Trusted: true}})
	waitFor(t, "download", func() bool {
		l, ok := e.Live(meta.Hash)
		return ok && l.HaveInfo && l.FileDone[0]+l.FileDone[1] >= meta.Size
	})
	e.Unload(meta.Hash)
	if _, ok := e.Live(meta.Hash); ok {
		t.Fatal("Unload() left the torrent running")
	}
	if err := e.Restore(meta.Hash, torrents.RestoreOptions{AddOptions: torrents.AddOptions{Dir: dir, Layout: "original"}}); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	waitFor(t, "restored torrent to know its files are complete", func() bool {
		l, ok := e.Live(meta.Hash)
		return ok && l.HaveInfo && l.FileDone[0]+l.FileDone[1] >= meta.Size
	})
}

func TestEngine_Meta_Invalid(t *testing.T) {
	e := newEngine(t)
	tests := []struct {
		name string
		src  torrents.Source
	}{
		{"bad magnet", torrents.Source{Magnet: "magnet:?dn=x"}},
		{"bad file", torrents.Source{Torrent: []byte("not a torrent")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := e.Meta(context.Background(), tc.src); err == nil {
				t.Error("Meta() expected error, got nil")
			}
		})
	}
}

func TestPruneDirs_StaysInsideBase(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "downloads")
	nested := filepath.Join(base, "Show", "S01")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	pruneDirs([]string{filepath.Join(nested, "ep1.mkv")}, base)
	if _, err := os.Stat(filepath.Join(base, "Show")); err == nil {
		t.Error("pruneDirs() left the empty Show folder")
	}
	if _, err := os.Stat(base); err != nil {
		t.Errorf("pruneDirs() removed the save folder itself: %v", err)
	}
	if _, err := os.Stat(root); err != nil {
		t.Errorf("pruneDirs() removed a parent of the save folder: %v", err)
	}
}

func TestPeerPrefix(t *testing.T) {
	tests := map[string]string{"0.9.0": "-SW0900-", "1.2.3": "-SW1230-", "10": "-SW1000-", "0.9.1-rc.1": "-SW0910-", "dev": "-SW0000-"}
	for in, want := range tests {
		if got := peerPrefix(in); got != want {
			t.Errorf("peerPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func ipPort(port int) torrentAddr { return torrentAddr(fmt.Sprintf("127.0.0.1:%d", port)) }
