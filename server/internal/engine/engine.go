package engine

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/dht/v2"
	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/anacrolix/torrent/types"
	"golang.org/x/time/rate"

	"bitswarm/server/internal/torrents"
)

type Config struct {
	DataDir    string
	Port       int
	DHT        bool
	Upnp       bool
	Encryption string
	Version    string
	Logger     *slog.Logger
}

type Engine struct {
	cl      *torrent.Client
	pc      storage.PieceCompletion
	dlLimit *rate.Limiter
	ulLimit *rate.Limiter
	metaDir string
	tmpDir  string

	mu    sync.Mutex
	items map[string]*entry
	cache map[string]cached
}

type cached struct {
	raw  []byte
	meta torrents.Meta
	at   time.Time
}

type entry struct {
	hash    string
	t       *torrent.Torrent
	dir     string
	layout  string
	prios   []string
	renames map[string]string
	magnet  string

	lastAt  time.Time
	lastDl  int64
	lastUl  int64
	dl, ul  float64
	verify  bool
	errText string
	notify  sync.Once
}

const metaTTL = 15 * time.Minute

func New(cfg Config) (*Engine, error) {
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "torrents"), 0o755); err != nil {
		return nil, fmt.Errorf("creating engine data folder: %w", err)
	}
	pc, err := storage.NewBoltPieceCompletion(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("opening piece database: %w", err)
	}
	e := &Engine{
		pc: pc, items: map[string]*entry{}, cache: map[string]cached{},
		metaDir: filepath.Join(cfg.DataDir, "torrents"), tmpDir: filepath.Join(cfg.DataDir, "preview"),
		dlLimit: rate.NewLimiter(rate.Inf, 1<<20), ulLimit: rate.NewLimiter(rate.Inf, 1<<20),
	}

	c := torrent.NewDefaultClientConfig()
	c.ListenPort = cfg.Port
	c.NoDHT = !cfg.DHT
	c.NoDefaultPortForwarding = !cfg.Upnp
	c.Seed = true
	c.DefaultStorage = storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir: cfg.DataDir, PieceCompletion: keepOpen{pc}, UsePartFiles: g.Some(false),
	})
	c.DownloadRateLimiter = e.dlLimit
	c.UploadRateLimiter = e.ulLimit
	c.ExtendedHandshakeClientVersion = "Bitswarm " + cfg.Version
	c.Bep20 = peerPrefix(cfg.Version)
	c.HTTPUserAgent = "Bitswarm/" + cfg.Version
	switch cfg.Encryption {
	case "require":
		c.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{Preferred: true, RequirePreferred: true}
	case "disable":
		c.HeaderObfuscationPolicy = torrent.HeaderObfuscationPolicy{Preferred: false}
	}
	if cfg.Logger != nil {
		c.Slogger = cfg.Logger
	} else {
		c.Slogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	cl, err := torrent.NewClient(c)
	if err != nil {
		pc.Close()
		return nil, fmt.Errorf("starting the BitTorrent client on port %d: %w", cfg.Port, err)
	}
	e.cl = cl
	return e, nil
}

func peerPrefix(version string) string {
	digits := strings.ReplaceAll(version, ".", "") + "0000"
	return "-SW" + digits[:4] + "-"
}

type keepOpen struct{ storage.PieceCompletion }

func (keepOpen) Close() error { return nil }

func (e *Engine) Name() string { return "BitTorrent" }

func (e *Engine) Port() int { return e.cl.LocalPort() }

func (e *Engine) Close() error {
	errs := e.cl.Close()
	_ = e.pc.Close()
	os.RemoveAll(e.tmpDir)
	return errors.Join(errs...)
}

func hashOf(h metainfo.Hash) string { return strings.ToLower(h.HexString()) }

func parseHash(s string) (metainfo.Hash, error) {
	var h metainfo.Hash
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 20 {
		return h, fmt.Errorf("%q is not a 40-character info hash", s)
	}
	copy(h[:], b)
	return h, nil
}

func layoutPath(info *metainfo.Info, f *metainfo.FileInfo, layout string) string {
	parts := f.BestPath()
	multi := len(info.UpvertedFiles()) > 1 || len(info.Files) > 0
	name := info.BestName()
	switch {
	case multi && layout == "nosub":
		return filepath.Join(parts...)
	case !multi && layout == "subfolder" && name != metainfo.NoName:
		return filepath.Join(name, filepath.Join(parts...))
	case name != metainfo.NoName:
		return filepath.Join(append([]string{name}, parts...)...)
	}
	return filepath.Join(parts...)
}

func (e *Engine) storageFor(en *entry) storage.ClientImpl {
	return storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir:   en.dir,
		PieceCompletion: keepOpen{e.pc},
		UsePartFiles:    g.Some(false),
		FilePathMaker: func(o storage.FilePathMakerOpts) string {
			base := layoutPath(o.Info, o.File, en.layout)
			if v, ok := en.renames[filepath.ToSlash(base)]; ok {
				return filepath.FromSlash(v)
			}
			return base
		},
	})
}

func metaOf(mi *metainfo.MetaInfo, info *metainfo.Info, layout string) torrents.Meta {
	files := info.UpvertedFiles()
	out := make([]torrents.File, len(files))
	for i := range files {
		out[i] = torrents.File{Name: filepath.ToSlash(layoutPath(info, &files[i], layout)), Size: float64(files[i].Length), Prio: "normal"}
	}
	m := torrents.Meta{Hash: hashOf(mi.HashInfoBytes()), Name: info.BestName(), Size: float64(info.TotalLength()), PieceSize: float64(info.PieceLength), Files: out}
	if m.Name == metainfo.NoName && len(out) > 0 {
		m.Name = out[0].Name
	}
	for _, tier := range mi.UpvertedAnnounceList() {
		m.Trackers = append(m.Trackers, tier...)
	}
	return m
}

func parseSource(src torrents.Source) (*metainfo.MetaInfo, *metainfo.Info, string, error) {
	if len(src.Torrent) > 0 {
		mi, err := metainfo.Load(bytes.NewReader(src.Torrent))
		if err != nil {
			return nil, nil, "", torrents.Invalid("That file isn’t a valid .torrent file. Choose a different file, or paste a magnet link instead.")
		}
		info, err := mi.UnmarshalInfo()
		if err != nil {
			return nil, nil, "", torrents.Invalid("That .torrent file is damaged: its file list can’t be read.")
		}
		return mi, &info, "", nil
	}
	m, err := metainfo.ParseMagnetV2Uri(strings.TrimSpace(src.Magnet))
	if err != nil || !m.InfoHash.Ok {
		return nil, nil, "", torrents.Invalid("This doesn’t look like a magnet link. It should start with “magnet:?xt=urn:btih:” followed by a 40-character hash.")
	}
	return nil, nil, hashOf(m.InfoHash.Value), nil
}

func (e *Engine) Meta(ctx context.Context, src torrents.Source) (torrents.Meta, error) {
	mi, info, hash, err := parseSource(src)
	if err != nil {
		return torrents.Meta{}, err
	}
	if mi != nil {
		m := metaOf(mi, info, "original")
		e.remember(m, src.Torrent)
		return m, nil
	}
	if c, ok := e.fromCache(hash); ok {
		return c.meta, nil
	}
	e.mu.Lock()
	if en, ok := e.items[hash]; ok && en.t.Info() != nil {
		mi := en.t.Metainfo()
		m := metaOf(&mi, en.t.Info(), "original")
		e.mu.Unlock()
		return m, nil
	}
	e.mu.Unlock()

	spec, err := torrent.TorrentSpecFromMagnetUri(strings.TrimSpace(src.Magnet))
	if err != nil {
		return torrents.Meta{}, torrents.Invalid("This doesn’t look like a magnet link.")
	}
	dir := filepath.Join(e.tmpDir, hash)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return torrents.Meta{}, err
	}
	defer os.RemoveAll(dir)
	spec.Storage = storage.NewFileOpts(storage.NewFileClientOpts{ClientBaseDir: dir, PieceCompletion: storage.NewMapPieceCompletion(), UsePartFiles: g.Some(false)})
	spec.DisallowDataDownload, spec.DisallowDataUpload = true, true
	t, _, err := e.cl.AddTorrentSpec(spec)
	if err != nil {
		return torrents.Meta{}, err
	}
	defer t.Drop()
	select {
	case <-t.GotInfo():
	case <-ctx.Done():
		return torrents.Meta{}, torrents.Invalid("No peers sent the file list in time. Check the link, make sure DHT is on in Settings, or try again later.")
	}
	mi2 := t.Metainfo()
	info2 := t.Info()
	m := metaOf(&mi2, info2, "original")
	var buf bytes.Buffer
	if err := mi2.Write(&buf); err == nil {
		e.remember(m, buf.Bytes())
	}
	return m, nil
}

func (e *Engine) remember(m torrents.Meta, raw []byte) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for k, v := range e.cache {
		if time.Since(v.at) > metaTTL {
			delete(e.cache, k)
		}
	}
	e.cache[m.Hash] = cached{raw: raw, meta: m, at: time.Now()}
}

func (e *Engine) fromCache(hash string) (cached, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.cache[hash]
	if ok && time.Since(c.at) > metaTTL {
		delete(e.cache, hash)
		return cached{}, false
	}
	return c, ok
}

func (e *Engine) Add(src torrents.Source, o torrents.AddOptions) (torrents.Meta, error) {
	mi, info, hash, err := parseSource(src)
	if err != nil {
		return torrents.Meta{}, err
	}
	var raw []byte = src.Torrent
	if mi == nil {
		if c, ok := e.fromCache(hash); ok {
			raw = c.raw
			if loaded, lerr := metainfo.Load(bytes.NewReader(raw)); lerr == nil {
				mi = loaded
				if inf, ierr := mi.UnmarshalInfo(); ierr == nil {
					info = &inf
				}
			}
		}
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	en := &entry{layout: o.Layout, dir: o.Dir, prios: o.Prios, renames: map[string]string{}, magnet: strings.TrimSpace(src.Magnet)}
	var meta torrents.Meta
	var spec *torrent.TorrentSpec
	if mi != nil && info != nil {
		meta = metaOf(mi, info, o.Layout)
		en.hash = meta.Hash
		spec, err = torrent.TorrentSpecFromMetaInfoErr(mi)
		if err != nil {
			return torrents.Meta{}, torrents.Invalid("That torrent’s file list can’t be read.")
		}
		_ = os.WriteFile(e.metaPath(meta.Hash), rawOf(mi, raw), 0o644)
	} else {
		spec, err = torrent.TorrentSpecFromMagnetUri(en.magnet)
		if err != nil {
			return torrents.Meta{}, torrents.Invalid("This doesn’t look like a magnet link.")
		}
		en.hash = hash
		meta = torrents.Meta{Hash: hash, Name: spec.DisplayName}
	}
	if _, dup := e.items[en.hash]; dup {
		return torrents.Meta{}, torrents.Invalid("This torrent is already in your list.")
	}
	if err := e.start(en, spec); err != nil {
		return torrents.Meta{}, err
	}
	return meta, nil
}

func rawOf(mi *metainfo.MetaInfo, raw []byte) []byte {
	if len(raw) > 0 {
		return raw
	}
	var buf bytes.Buffer
	_ = mi.Write(&buf)
	return buf.Bytes()
}

func (e *Engine) metaPath(hash string) string { return filepath.Join(e.metaDir, hash+".torrent") }

func (e *Engine) start(en *entry, spec *torrent.TorrentSpec) error {
	if err := os.MkdirAll(en.dir, 0o755); err != nil {
		return torrents.Invalid("Bitswarm can’t use the folder " + en.dir + ": " + err.Error())
	}
	spec.Storage = e.storageFor(en)
	t, _, err := e.cl.AddTorrentSpec(spec)
	if err != nil {
		return err
	}
	en.t = t
	t.SetOnWriteChunkError(func(err error) {
		e.mu.Lock()
		en.errText = "Writing to the download folder failed: " + err.Error()
		e.mu.Unlock()
	})
	e.items[en.hash] = en
	go e.afterInfo(en)
	return nil
}

func (e *Engine) afterInfo(en *entry) {
	select {
	case <-en.t.GotInfo():
	case <-en.t.Closed():
		return
	}
	e.mu.Lock()
	if cur, ok := e.items[en.hash]; !ok || cur != en {
		e.mu.Unlock()
		return
	}
	mi := en.t.Metainfo()
	if _, err := os.Stat(e.metaPath(en.hash)); err != nil {
		var buf bytes.Buffer
		if mi.Write(&buf) == nil {
			_ = os.WriteFile(e.metaPath(en.hash), buf.Bytes(), 0o644)
		}
	}
	e.applyPrios(en)
	e.mu.Unlock()
}

func (e *Engine) applyPrios(en *entry) {
	if en.t.Info() == nil {
		return
	}
	for i, f := range en.t.Files() {
		p := "normal"
		if i < len(en.prios) && en.prios[i] != "" {
			p = en.prios[i]
		}
		switch p {
		case "skip":
			f.SetPriority(types.PiecePriorityNone)
		case "high":
			f.SetPriority(types.PiecePriorityHigh)
		default:
			f.SetPriority(types.PiecePriorityNormal)
		}
	}
}

func (e *Engine) Restore(hash string, o torrents.RestoreOptions) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.items[hash]; ok {
		return nil
	}
	h, err := parseHash(hash)
	if err != nil {
		return err
	}
	en := &entry{hash: hash, dir: o.Dir, layout: o.Layout, prios: o.Prios, renames: map[string]string{}, magnet: o.Magnet}
	var spec *torrent.TorrentSpec
	if mi, lerr := metainfo.LoadFromFile(e.metaPath(hash)); lerr == nil {
		spec, err = torrent.TorrentSpecFromMetaInfoErr(mi)
		if err != nil {
			return err
		}
		if info, ierr := mi.UnmarshalInfo(); ierr == nil {
			files := info.UpvertedFiles()
			for i := range files {
				if i < len(o.Names) && o.Names[i] != "" {
					base := filepath.ToSlash(layoutPath(&info, &files[i], o.Layout))
					if o.Names[i] != base {
						en.renames[base] = o.Names[i]
					}
				}
			}
		}
	} else if o.Magnet != "" {
		spec, err = torrent.TorrentSpecFromMagnetUri(o.Magnet)
		if err != nil {
			return err
		}
	} else {
		spec = &torrent.TorrentSpec{}
		spec.InfoHash = h
	}
	return e.start(en, spec)
}

func (e *Engine) Unload(hash string) {
	e.mu.Lock()
	en, ok := e.items[hash]
	delete(e.items, hash)
	e.mu.Unlock()
	if ok {
		en.t.Drop()
	}
}

func (e *Engine) Forget(hash string, deleteData bool) error {
	e.mu.Lock()
	en, ok := e.items[hash]
	delete(e.items, hash)
	e.mu.Unlock()
	var paths []string
	base := ""
	if ok {
		base = en.dir
		if info := en.t.Info(); info != nil {
			files := info.UpvertedFiles()
			for i := range files {
				p := filepath.ToSlash(layoutPath(info, &files[i], en.layout))
				if v, has := en.renames[p]; has {
					p = v
				}
				paths = append(paths, filepath.Join(en.dir, filepath.FromSlash(p)))
			}
		}
		en.t.Drop()
	}
	_ = os.Remove(e.metaPath(hash))
	if deleteData {
		return removeFiles(paths, base)
	}
	return nil
}

func removeFiles(paths []string, base string) error {
	var first error
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) && first == nil {
			first = err
		}
	}
	pruneDirs(paths, base)
	return first
}

func (e *Engine) get(hash string) (*entry, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	en, ok := e.items[hash]
	return en, ok
}

func (e *Engine) Recheck(hash string) error {
	en, ok := e.get(hash)
	if !ok {
		return errors.New("the torrent isn’t running, so it can’t be checked")
	}
	e.mu.Lock()
	if en.verify {
		e.mu.Unlock()
		return nil
	}
	en.verify = true
	e.mu.Unlock()
	go func() {
		select {
		case <-en.t.GotInfo():
		case <-en.t.Closed():
			return
		}
		err := en.t.VerifyData()
		e.mu.Lock()
		en.verify = false
		if err != nil {
			en.errText = "Checking the files failed: " + err.Error()
		}
		e.mu.Unlock()
	}()
	return nil
}

func (e *Engine) SetPriorities(hash string, prios []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if en, ok := e.items[hash]; ok {
		en.prios = append([]string{}, prios...)
		e.applyPrios(en)
	}
}

func (e *Engine) Hash(src torrents.Source) (string, error) {
	mi, _, hash, err := parseSource(src)
	if err != nil {
		return "", err
	}
	if mi != nil {
		return hashOf(mi.HashInfoBytes()), nil
	}
	return hash, nil
}

func (e *Engine) Reannounce(hash string) {
	en, ok := e.get(hash)
	if !ok {
		return
	}
	for _, s := range e.cl.DhtServers() {
		_, _ = s.Announce(en.t.InfoHash(), e.cl.LocalPort(), true)
	}
}

func (e *Engine) SetRates(dl, ul float64) {
	set := func(l *rate.Limiter, v float64) {
		if v <= 0 {
			l.SetLimit(rate.Inf)
			return
		}
		l.SetLimit(rate.Limit(v))
	}
	set(e.dlLimit, dl)
	set(e.ulLimit, ul)
}

func (e *Engine) Totals() torrents.EngineTotals {
	e.mu.Lock()
	defer e.mu.Unlock()
	var tot torrents.EngineTotals
	for _, en := range e.items {
		tot.Conns += en.t.Stats().ActivePeers
		tot.Dl += en.dl
		tot.Ul += en.ul
	}
	for _, s := range e.cl.DhtServers() {
		if st, ok := s.Stats().(dht.ServerStats); ok {
			tot.DHTNodes += st.GoodNodes
		}
	}
	return tot
}
