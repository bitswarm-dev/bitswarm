package torrents

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/settings"
)

var (
	ErrNotFound = docstore.ErrNotFound
	pathRe      = regexp.MustCompile(`^(/|~|[A-Za-z]:[\\/])`)
)

type Invalid string

func (e Invalid) Error() string { return string(e) }

type SettingsSource interface {
	Get() settings.Settings
}

type Catalog interface {
	EnsureTags(names []string)
	RemoveTags(names []string)
	TagNames() []string
}

type Logger interface {
	Add(level, src, msg string)
}

type Renamer interface {
	ApplyRenames(ctx context.Context, t *Torrent, when string)
}

type Service struct {
	repo Repository
	set  SettingsSource
	cat  Catalog
	bus  events.Emitter
	log  Logger

	mu      sync.Mutex
	renamer Renamer
	eng     Engine
	freeFn  func() float64
	rechk   map[int64]bool
	check   func(string) error
	free    float64
	dht     int
	sessDl  float64
	sessUl  float64
	alt     bool
	hist    History
	started time.Time
	now     func() time.Time
	rnd     func() float64
}

func NewService(repo Repository, set SettingsSource, cat Catalog, bus events.Emitter, log Logger) *Service {
	s := &Service{repo: repo, set: set, cat: cat, bus: bus, log: log, free: 318.4 * GiB, dht: 412,
		started: time.Now(), now: time.Now, rnd: newRng(time.Now().UnixNano()).next}
	s.prefill()
	return s
}

func (s *Service) SetEngine(e Engine, free func() float64) {
	s.mu.Lock()
	s.eng, s.freeFn, s.rechk = e, free, map[int64]bool{}
	s.hist = History{Dl: make([]float64, 60), Ul: make([]float64, 60)}
	s.dht = 0
	s.mu.Unlock()
}

func (s *Service) EngineName() string {
	if s.eng == nil {
		return "Simulated engine"
	}
	return s.eng.Name()
}

func (s *Service) SetPathCheck(fn func(string) error) { s.check = fn }

func (s *Service) checkPath(p string) error {
	if s.check == nil {
		return nil
	}
	if err := s.check(p); err != nil {
		return Invalid(err.Error())
	}
	return nil
}

func (s *Service) SetRenamer(r Renamer) {
	s.mu.Lock()
	s.renamer = r
	s.mu.Unlock()
}

func (s *Service) nowMs() int64 { return s.now().UnixMilli() }

func (s *Service) Seed(ctx context.Context) error {
	items, err := s.repo.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	for _, o := range samples() {
		id := int64(len(items) + 1)
		t := newTorrent(id, o, s.nowMs())
		for i := 0; i < 60; i++ {
			s.stepSpeeds(&t)
			t.Hist.Dl = append(t.Hist.Dl, t.Dl)
			t.Hist.Ul = append(t.Hist.Ul, t.Ul)
		}
		if _, err := s.repo.Create(ctx, t); err != nil {
			return err
		}
		items = append(items, t)
	}
	return nil
}

func (s *Service) prefill() {
	for i := 0; i < 60; i++ {
		s.hist.Dl = append(s.hist.Dl, 2*MiB+s.rnd()*MiB)
		s.hist.Ul = append(s.hist.Ul, .4*MiB+s.rnd()*.3*MiB)
	}
}

func (s *Service) stepSpeeds(t *Torrent) {
	switch t.Status {
	case StatusDownloading:
		base := t.Dl
		if base == 0 {
			base = t.Target * .6
		}
		t.Dl = walk(base, t.Target, .5, s.rnd)
		t.Ul = walk(t.Ul, t.UpTarget, .6, s.rnd)
	case StatusSeeding:
		base := t.Ul
		if base == 0 {
			base = t.UpTarget * .5
		}
		t.Ul = walk(base, t.UpTarget, .8, s.rnd)
	}
}

func (s *Service) views(items []Torrent) []View {
	unfinished := []Torrent{}
	for _, t := range items {
		if t.Unfinished() {
			unfinished = append(unfinished, t)
		}
	}
	sort.SliceStable(unfinished, func(i, j int) bool { return unfinished[i].Queue < unfinished[j].Queue })
	pos := map[int64]int{}
	for i, t := range unfinished {
		pos[t.ID] = i + 1
	}
	out := make([]View, len(items))
	for i, t := range items {
		if t.Peers == nil {
			t.Peers = []Peer{}
		}
		if t.Files == nil {
			t.Files = []File{}
		}
		if t.Tags == nil {
			t.Tags = []string{}
		}
		if t.Trackers == nil {
			t.Trackers = []string{}
		}
		if t.Hist.Dl == nil {
			t.Hist.Dl = []float64{}
		}
		if t.Hist.Ul == nil {
			t.Hist.Ul = []float64{}
		}
		v := View{Torrent: t, Wanted: t.Wanted(), Progress: t.Pct(), Ratio: t.RatioValue(), ETA: t.ETA()}
		if p, ok := pos[t.ID]; ok {
			v.QueuePos = &p
		}
		out[i] = v
	}
	return out
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	return s.views(items), nil
}

var (
	magnetRe = regexp.MustCompile(`(?i)^magnet:\?`)
	btihRe   = regexp.MustCompile(`(?i)xt=urn:btih:([0-9a-z]{32,40})`)
	dnRe     = regexp.MustCompile(`(?i)[?&]dn=([^&]+)`)
)

func NameFromMagnet(m string) (name, hash string, err error) {
	m = strings.TrimSpace(m)
	if !magnetRe.MatchString(m) || !btihRe.MatchString(m) {
		return "", "", Invalid("This doesn’t look like a magnet link. It should start with “magnet:?xt=urn:btih:” followed by a 40-character hash.")
	}
	hash = strings.ToLower(btihRe.FindStringSubmatch(m)[1])
	name = "Magnet " + hash[:8]
	if dn := dnRe.FindStringSubmatch(m); dn != nil {
		if d, derr := url.QueryUnescape(dn[1]); derr == nil {
			name = d
		}
	}
	return name, hash, nil
}

type AddSpec struct {
	Magnet     string         `json:"magnet"`
	FileName   string         `json:"fileName"`
	FileData   []byte         `json:"fileData"`
	Path       string         `json:"path"`
	Tags       []string       `json:"tags"`
	Preset     int64          `json:"preset"`
	Start      bool           `json:"start"`
	Layout     string         `json:"layout"`
	SkipHash   bool           `json:"skipHash"`
	Seq        bool           `json:"seq"`
	Flp        bool           `json:"flp"`
	Top        bool           `json:"top"`
	Selected   []int          `json:"selected"`
	Name       string         `json:"-"`
	Hash       string         `json:"-"`
	Size       float64        `json:"-"`
	Tracker    string         `json:"-"`
	Source     string         `json:"-"`
	Schedule   map[string]any `json:"-"`
	Link       string         `json:"-"`
	TorrentURL string         `json:"-"`
	Contents   *Contents      `json:"-"`
	NoContents bool           `json:"-"`
}

func (s *Service) Add(ctx context.Context, a AddSpec) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng != nil {
		return s.addReal(ctx, a)
	}

	if a.Name == "" {
		switch {
		case strings.TrimSpace(a.Magnet) != "":
			name, hash, err := NameFromMagnet(a.Magnet)
			if err != nil {
				return View{}, err
			}
			a.Name, a.Hash = name, hash
		case a.FileName != "":
			a.Name = strings.TrimSuffix(a.FileName, ".torrent")
			if low := strings.ToLower(a.FileName); !strings.HasSuffix(low, ".torrent") {
				return View{}, Invalid("That file isn’t a .torrent file. Choose a file ending in .torrent.")
			}
			a.Name = a.FileName[:len(a.FileName)-len(".torrent")]
		default:
			return View{}, Invalid("Paste a magnet link or choose a .torrent file first.")
		}
	}

	existing, err := s.repo.List(ctx)
	if err != nil {
		return View{}, err
	}
	if a.Hash != "" {
		for _, e := range existing {
			if e.Hash == a.Hash {
				return View{}, Invalid("This torrent is already in your list.")
			}
		}
	}

	set := s.set.Get()
	c := Contents{}
	switch {
	case a.Contents != nil:
		c = *a.Contents
	case a.Size > 0:
		c = Contents{Size: a.Size, Files: []File{{Name: a.Name, Size: a.Size, Prio: "normal"}}}
	default:
		c = GenContents(a.Name)
	}
	selected := map[int]bool{}
	if a.Selected == nil {
		for i := range c.Files {
			selected[i] = true
		}
	} else {
		for _, i := range a.Selected {
			selected[i] = true
		}
	}
	if len(selected) == 0 || (a.Selected != nil && len(a.Selected) == 0) {
		return View{}, Invalid("Select at least one file to download.")
	}

	r := s.rnd()
	status := StatusPaused
	switch {
	case a.SkipHash && a.Start:
		status = StatusSeeding
	case a.SkipHash:
		status = StatusDone
	case a.Start:
		status = StatusChecking
	}
	tracker := a.Tracker
	if tracker == "" {
		tracker = "udp://tracker.opentrackr.org:1337/announce"
	}
	specFiles := make([]fileSpec, len(c.Files))
	for i, f := range c.Files {
		specFiles[i] = fileSpec{LayoutName(f.Name, a.Name, a.Layout, c.Files), f.Size}
	}
	next := int64(1)
	minQueue := 1 << 30
	for _, e := range existing {
		if e.ID >= next {
			next = e.ID + 1
		}
		if e.Queue < minQueue {
			minQueue = e.Queue
		}
	}
	t := newTorrent(next, spec{
		Name: a.Name, Size: c.Size, Status: status, Tracker: tracker, Target: (2 + r*7) * MiB, UpTarget: .3 * MiB,
		Seeds: int(20 + r*400), Leech: int(5 + r*60), AgeHours: 0.0001, CheckLeft: 3, Uploaded: ptr(0.0), Files: specFiles,
	}, s.nowMs())
	for i := range t.Files {
		if !selected[i] {
			t.Files[i].Prio = "skip"
		}
	}
	if a.SkipHash {
		t.Done = t.Wanted()
		t.SeedSince = s.nowMs()
	}
	if a.Hash != "" {
		t.Hash = (a.Hash + strings.Repeat("0", 40))[:40]
	}
	t.Path = strings.TrimSpace(a.Path)
	if t.Path == "" {
		t.Path = set.SavePath
	}
	if err := s.checkPath(t.Path); err != nil {
		return View{}, err
	}
	if a.Preset > 0 {
		t.Presets = []int64{a.Preset}
	}
	t.Tags = a.Tags
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.Seq, t.Flp = a.Seq, a.Flp
	if a.Top && minQueue != 1<<30 {
		t.Queue = minQueue - 1
	}
	if !a.Start && !a.SkipHash && set.Queueing && a.Top {
		t.Status = StatusQueued
	}

	created, err := s.repo.Create(ctx, t)
	if err != nil {
		return View{}, err
	}
	s.cat.EnsureTags(t.Tags)

	data := map[string]any{"torrent": Info(created), "source": orDefault(a.Source, "manual"), "files_selected": len(selected), "files_total": len(c.Files), "skip_hash_check": a.SkipHash}
	if a.Schedule != nil {
		data["schedule"] = a.Schedule
		data["link"] = a.Link
	}
	s.bus.Emit("torrent.added", data)
	if s.renamer != nil {
		s.renamer.ApplyRenames(ctx, &created, "add")
		_ = s.repo.Put(ctx, created)
	}
	all, _ := s.repo.List(ctx)
	for _, v := range s.views(all) {
		if v.ID == created.ID {
			return v, nil
		}
	}
	return View{Torrent: created}, nil
}

func ptr[T any](v T) *T { return &v }

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
