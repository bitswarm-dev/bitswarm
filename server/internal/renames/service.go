package renames

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/torrents"
)

type Invalid string

func (e Invalid) Error() string { return string(e) }

var ErrNotFound = docstore.ErrNotFound

type TorrentStore interface {
	Snapshot(ctx context.Context) ([]torrents.Torrent, error)
	Mutate(ctx context.Context, fn func([]torrents.Torrent)) error
}

type Toaster interface {
	Add(level, src, msg string)
}

type Service struct {
	presets *docstore.Collection[Preset]
	log     *docstore.Collection[LogEntry]
	store   TorrentStore
	bus     events.Emitter
	now     func() time.Time
	users   func(id int64) []string
	onGone  []func(id int64)
}

func NewCollections(db docstoreDB) (*docstore.Collection[Preset], *docstore.Collection[LogEntry]) {
	return docstore.NewCollection(db, "renames", func(p *Preset) *int64 { return &p.ID }),
		docstore.NewCollection(db, "renamelog", func(l *LogEntry) *int64 { return &l.ID })
}

func NewService(presets *docstore.Collection[Preset], log *docstore.Collection[LogEntry], store TorrentStore, bus events.Emitter) *Service {
	return &Service{presets: presets, log: log, store: store, bus: bus, now: time.Now, users: func(int64) []string { return nil }}
}

func (s *Service) SetUsers(fn func(id int64) []string) { s.users = fn }

func (s *Service) OnDelete(fn func(id int64)) { s.onGone = append(s.onGone, fn) }

func (s *Service) Seed(ctx context.Context) error {
	items, err := s.presets.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	for _, p := range []Preset{
		{Name: "Tidy Ubuntu ISO names", Find: `^ubuntu-(\d+\.\d+(?:\.\d+)?)-desktop-(\w+)$`, Replace: "Ubuntu $1 Desktop ($2)", I: true, KeepExt: true, Target: "files", Tag: "keep", When: "complete", Enabled: true},
		{Name: "Film title and year", Find: `^([A-Za-z]+(?:[._ ][A-Za-z]+)*?)[._ ](\d{4})\b.*$`, Replace: "$1 ($2)", KeepExt: true, Target: "files", Tag: "4K", When: "complete", Enabled: true},
		{Name: "Drop quality tags", Find: `\s*\(\d{3,4}p[^)]*\)`, G: true, I: true, KeepExt: true, Target: "torrent", When: "complete"},
	} {
		if _, err := s.presets.Create(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) validate(p Preset) error {
	switch {
	case strings.TrimSpace(p.Name) == "":
		return Invalid("Give the preset a name.")
	case p.Find == "":
		return Invalid("Enter a pattern to find.")
	case !slices.Contains([]string{"files", "torrent", "both"}, p.Target):
		return Invalid("Choose what to rename: file names, the torrent name, or both.")
	case !slices.Contains([]string{"complete", "add"}, p.When):
		return Invalid("Choose when the preset runs.")
	}
	if _, err := Compile(p); err != nil {
		return Invalid("The “Find” pattern isn’t a valid regular expression (" + strings.TrimPrefix(err.Error(), "error parsing regexp: ") + "). Check brackets and escape special characters like . ( ) with \\.")
	}
	return nil
}

func (s *Service) example(ctx context.Context, p Preset, ts []torrents.Torrent) *Example {
	for _, t := range ts {
		if p.Tag != "" && !slices.Contains(t.Tags, p.Tag) {
			continue
		}
		for _, it := range plan(p, t) {
			if it.To != "" {
				return &Example{From: it.From, To: it.To}
			}
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	ps, err := s.presets.List(ctx)
	if err != nil {
		return nil, err
	}
	ts, err := s.store.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(ps))
	for i, p := range ps {
		out[i] = View{Preset: p, Example: s.example(ctx, p, ts), UsedBy: s.users(p.ID)}
		if out[i].UsedBy == nil {
			out[i].UsedBy = []string{}
		}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, p Preset) (Preset, error) {
	p.Name = strings.TrimSpace(p.Name)
	if err := s.validate(p); err != nil {
		return Preset{}, err
	}
	p.Count = 0
	return s.presets.Create(ctx, p)
}

func (s *Service) Update(ctx context.Context, p Preset) (Preset, error) {
	p.Name = strings.TrimSpace(p.Name)
	if err := s.validate(p); err != nil {
		return Preset{}, err
	}
	old, err := s.presets.Get(ctx, p.ID)
	if err != nil {
		return Preset{}, err
	}
	p.Count = old.Count
	return p, s.presets.Put(ctx, p)
}

func (s *Service) SetEnabled(ctx context.Context, id int64, on bool) (Preset, error) {
	p, err := s.presets.Get(ctx, id)
	if err != nil {
		return p, err
	}
	p.Enabled = on
	return p, s.presets.Put(ctx, p)
}

func (s *Service) Duplicate(ctx context.Context, id int64) (Preset, error) {
	p, err := s.presets.Get(ctx, id)
	if err != nil {
		return p, err
	}
	p.Name += " copy"
	p.Enabled, p.Count = false, 0
	return s.presets.Create(ctx, p)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.presets.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.store.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			ts[i].Presets = slices.DeleteFunc(ts[i].Presets, func(x int64) bool { return x == id })
		}
	})
	for _, fn := range s.onGone {
		fn(id)
	}
	return nil
}

func (s *Service) applyRule(ctx context.Context, p Preset, t *torrents.Torrent) int {
	n := 0
	for _, it := range plan(p, *t) {
		if it.To == "" {
			continue
		}
		kind := "torrent"
		if it.File >= 0 {
			kind = "file"
			f := &t.Files[it.File]
			if f.Orig == "" {
				f.Orig = it.From
			}
			f.Name = it.To
		}
		if it.File < 0 {
			t.Name = it.To
		} else if it.AlsoTorrent {
			t.Name = path.Base(it.To)
		}
		_, _ = s.log.Create(ctx, LogEntry{T: s.now().UnixMilli(), Rule: p.Name, From: it.From, To: it.To, TorrentID: t.ID, Kind: kind, Also: it.AlsoTorrent})
		s.bus.Emit("torrent.renamed", map[string]any{"torrent": torrents.Info(*t), "preset": p.Name, "target": kind, "from": it.From, "to": it.To})
		n++
	}
	if n > 0 {
		cur, err := s.presets.Get(ctx, p.ID)
		if err == nil {
			cur.Count += n
			_ = s.presets.Put(ctx, cur)
		}
		s.trimLog(ctx)
	}
	return n
}

func (s *Service) trimLog(ctx context.Context) {
	items, err := s.log.List(ctx)
	if err != nil || len(items) <= 60 {
		return
	}
	for _, l := range items[:len(items)-60] {
		_ = s.log.Delete(ctx, l.ID)
	}
}

func (s *Service) ApplyRenames(ctx context.Context, t *torrents.Torrent, when string) {
	ps, err := s.presets.List(ctx)
	if err != nil {
		return
	}
	total := 0
	for _, p := range ps {
		auto := p.Enabled && (p.Tag == "" || slices.Contains(t.Tags, p.Tag))
		if p.When == when && (auto || slices.Contains(t.Presets, p.ID)) {
			total += s.applyRule(ctx, p, t)
		}
	}
	_ = total
}

type ApplyResult struct {
	Names    int `json:"names"`
	Torrents int `json:"torrents"`
}

func (s *Service) Apply(ctx context.Context, id int64, ids []int64) (ApplyResult, error) {
	p, err := s.presets.Get(ctx, id)
	if err != nil {
		return ApplyResult{}, err
	}
	var res ApplyResult
	err = s.store.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			if len(ids) > 0 && !slices.Contains(ids, ts[i].ID) {
				continue
			}
			if len(ids) == 0 && p.Tag != "" && !slices.Contains(ts[i].Tags, p.Tag) {
				continue
			}
			if n := s.applyRule(ctx, p, &ts[i]); n > 0 {
				res.Names += n
				res.Torrents++
			}
		}
	})
	return res, err
}

func (s *Service) Log(ctx context.Context) ([]LogEntry, error) {
	items, err := s.log.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	return items, nil
}

func (s *Service) Undo(ctx context.Context, id int64) error {
	l, err := s.log.Get(ctx, id)
	if err != nil {
		return err
	}
	if l.Undone {
		return nil
	}
	err = s.store.Mutate(ctx, func(ts []torrents.Torrent) {
		for i := range ts {
			if ts[i].ID != l.TorrentID {
				continue
			}
			for j := range ts[i].Files {
				if ts[i].Files[j].Name == l.To && l.Kind == "file" {
					ts[i].Files[j].Name = l.From
					break
				}
			}
			shown := l.To
			if l.Also {
				shown = path.Base(l.To)
			}
			if ts[i].Name == shown && (l.Kind == "torrent" || l.Also) {
				ts[i].Name = l.From
			}
		}
	})
	if err != nil {
		return err
	}
	l.Undone = true
	return s.log.Put(ctx, l)
}

type PreviewReq struct {
	Preset
	Samples string `json:"samples"`
}

func (s *Service) Preview(ctx context.Context, r PreviewReq) (Preview, error) {
	out := Preview{Rows: []Row{}}
	if r.Find == "" {
		return out, nil
	}
	re, err := Compile(r.Preset)
	if err != nil {
		out.Error = "That pattern isn’t valid: " + strings.TrimPrefix(err.Error(), "error parsing regexp: ")
		return out, nil
	}
	ts, err := s.store.Snapshot(ctx)
	if err != nil {
		return out, err
	}
	rows := []Row{}
	for _, t := range ts {
		if r.Tag != "" && !slices.Contains(t.Tags, r.Tag) {
			continue
		}
		if r.Target != "torrent" {
			out.Total += len(t.Files)
		}
		if r.Target != "files" {
			out.Total++
		}
		for _, it := range plan(r.Preset, t) {
			rows = append(rows, Row{From: it.From, To: it.To, Empty: it.Empty, Clash: it.Clash, Src: t.Name, Match: true})
		}
	}
	for _, name := range strings.Split(r.Samples, "\n") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out.Total++
		tested := testedPart(r.Preset, name)
		if to, ok := renameOne(r.Preset, name, re, r.Target != "torrent"); ok {
			rows = append(rows, Row{From: name, To: to, Empty: to == "", Src: "Your names", Match: true, Tested: tested})
		} else {
			rows = append(rows, Row{From: name, To: name, Src: "Your names", Match: re.MatchString(tested), Same: true, Tested: tested})
		}
	}
	for _, row := range rows {
		if !row.Empty && !row.Same {
			out.OK++
		}
	}
	if len(rows) > 40 {
		rows = rows[:40]
	}
	out.Rows = rows
	return out, nil
}

type OldEntry struct {
	ID    int64
	Label string
	T     int64
}

func (s *Service) Source() string { return "Renames" }

func (s *Service) Old(cutoff int64) []OldEntry {
	items, err := s.log.List(context.Background())
	if err != nil {
		return nil
	}
	var out []OldEntry
	for _, l := range items {
		if l.T < cutoff {
			out = append(out, OldEntry{ID: l.ID, Label: l.Rule, T: l.T})
		}
	}
	return out
}

func (s *Service) Remove(id int64) { _ = s.log.Delete(context.Background(), id) }
