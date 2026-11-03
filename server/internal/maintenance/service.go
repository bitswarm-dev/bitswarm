package maintenance

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/torrents"
)

const (
	minute = int64(60e3)
	hour   = 60 * minute
	day    = 24 * hour
)

var ErrNotFound = docstore.ErrNotFound

type Invalid string

func (e Invalid) Error() string { return string(e) }

type TorrentOps interface {
	Snapshot(ctx context.Context) ([]torrents.Torrent, error)
	Mutate(ctx context.Context, fn func([]torrents.Torrent)) error
	RemoveWithReason(ctx context.Context, ids []int64, deleteData bool, reason string) (int, error)
	Free() float64
}

type Service struct {
	tasks   *docstore.Collection[Task]
	log     *docstore.Collection[LogEntry]
	tor     TorrentOps
	bus     events.Emitter
	pruners []Pruner
	now     func() time.Time
	mu      sync.Mutex
}

func NewCollections(db docstoreDB) (*docstore.Collection[Task], *docstore.Collection[LogEntry]) {
	return docstore.NewCollection(db, "maintenance", func(t *Task) *int64 { return &t.ID }),
		docstore.NewCollection(db, "maintlog", func(l *LogEntry) *int64 { return &l.ID })
}

func NewService(tasks *docstore.Collection[Task], log *docstore.Collection[LogEntry], tor TorrentOps, bus events.Emitter) *Service {
	return &Service{tasks: tasks, log: log, tor: tor, bus: bus, now: time.Now}
}

type adapter[E any] struct {
	src    string
	old    func(int64) []E
	remove func(int64)
	conv   func(E) OldEntry
}

func (a adapter[E]) Source() string  { return a.src }
func (a adapter[E]) Remove(id int64) { a.remove(id) }
func (a adapter[E]) Old(cutoff int64) []OldEntry {
	var out []OldEntry
	for _, e := range a.old(cutoff) {
		out = append(out, a.conv(e))
	}
	return out
}

func (s *Service) AddPruner(p Pruner) { s.pruners = append(s.pruners, p) }

func AdaptPruner[E any](src string, old func(int64) []E, remove func(int64), conv func(E) OldEntry) Pruner {
	return adapter[E]{src: src, old: old, remove: remove, conv: conv}
}

func (s *Service) nowMs() int64 { return s.now().UnixMilli() }

func unitMs(u string) int64 {
	switch u {
	case "minutes":
		return minute
	case "days":
		return day
	}
	return hour
}

func interval(t Task) int64 { return int64(max(1, t.Every)) * unitMs(t.Unit) }

type meta struct {
	title, desc string
	scoped      bool
}

var metas = map[string]meta{
	"retry":   {"Retry torrents with errors", "Re-announces to the trackers and resumes torrents that stopped with an error, such as a tracker timeout.", true},
	"cleanup": {"Remove finished torrents", "Removes torrents that finished a while ago so your list stays short. You choose whether their files stay on disk.", true},
	"stalled": {"Handle stalled downloads", "Finds downloads that have no seeds or haven’t received any data for a while.", true},
	"disk":    {"Pause downloads when disk space is low", "Pauses downloads before the disk fills up, and can resume them once there’s room again.", false},
	"verify":  {"Verify finished downloads", "Re-checks the pieces of finished torrents against their hashes to catch damaged files.", true},
	"logs":    {"Clear old activity", "Deletes old entries from the schedule, rename and maintenance activity lists.", false},
}

var order = []string{"retry", "cleanup", "stalled", "disk", "verify", "logs"}

func summary(t Task) string {
	p := t.P
	switch t.Type {
	case "retry":
		return "Re-announce and resume every torrent with an error"
	case "cleanup":
		ratio := ""
		if p.Ratio > 0 {
			ratio = fmt.Sprintf(", ratio %g+", p.Ratio)
		}
		files := "keep files"
		if p.Files == "delete" {
			files = "delete files"
		}
		return fmt.Sprintf("Finished %d+ days ago%s · %s", p.Days, ratio, files)
	case "stalled":
		act := map[string]string{"pause": "pause it", "reannounce": "re-announce", "remove": "remove it"}[p.Action]
		return fmt.Sprintf("No seeds, or no data for %d+ hours · %s", p.Hours, act)
	case "disk":
		resume := ""
		if p.Resume {
			resume = " · resume when space frees up"
		}
		return fmt.Sprintf("Pause below %d GB free%s", p.GB, resume)
	case "verify":
		return "Re-check every piece of each finished torrent"
	case "logs":
		return fmt.Sprintf("Entries older than %d days", p.Days)
	}
	return ""
}

func (s *Service) view(t Task) View {
	m := metas[t.Type]
	return View{Task: t, Title: m.title, Desc: m.desc, Scoped: m.scoped, Summary: summary(t)}
}

func (s *Service) Seed(ctx context.Context) error {
	items, err := s.tasks.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	now := s.nowMs()
	defaults := map[string]Task{
		"retry":   {Enabled: true, Every: 1, Unit: "hours"},
		"cleanup": {Enabled: true, Every: 1, Unit: "days", P: Params{Days: 7, Ratio: 1, Files: "keep"}},
		"stalled": {Enabled: true, Every: 2, Unit: "hours", P: Params{Hours: 6, Action: "pause"}},
		"disk":    {Enabled: true, Every: 15, Unit: "minutes", P: Params{GB: 20, Resume: true}},
		"verify":  {Enabled: false, Every: 7, Unit: "days"},
		"logs":    {Enabled: true, Every: 1, Unit: "days", P: Params{Days: 30}},
	}
	for _, typ := range order {
		t := defaults[typ]
		t.Type = typ
		t.NextRun = now + interval(t)
		if _, err := s.tasks.Create(ctx, t); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	ts, err := s.tasks.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(ts))
	for i, t := range ts {
		out[i] = s.view(t)
	}
	return out, nil
}

func inScope(t Task, x torrents.Torrent) bool { return t.Tag == "" || slices.Contains(x.Tags, t.Tag) }

func completedAt(t torrents.Torrent) int64 {
	switch {
	case t.SeedSince > 0:
		return t.SeedSince
	case t.Pct() >= 1:
		return t.Added
	}
	return 0
}

func ago(ms int64) string {
	d := ms / day
	return fmt.Sprintf("%d days ago", d)
}

func plural(n int, w string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", w)
	}
	return fmt.Sprintf("%d %ss", n, w)
}

func (s *Service) find(ctx context.Context, t Task) ([]Item, error) {
	ts, err := s.tor.Snapshot(ctx)
	if err != nil {
		return nil, err
	}
	now := s.nowMs()
	free := s.tor.Free()
	var items []Item
	switch t.Type {
	case "retry":
		for _, x := range ts {
			if x.Status == torrents.StatusError && inScope(t, x) {
				items = append(items, Item{ID: x.ID, Name: x.Name, What: "Re-announce and resume", Why: x.Err})
			}
		}
	case "cleanup":
		what := "Remove, keep files"
		if t.P.Files == "delete" {
			what = "Remove and delete files"
		}
		for _, x := range ts {
			c := completedAt(x)
			if inScope(t, x) && x.Pct() >= 1 && c > 0 && now-c >= int64(t.P.Days)*day && x.RatioValue() >= t.P.Ratio {
				items = append(items, Item{ID: x.ID, Name: x.Name, What: what, Why: fmt.Sprintf("Finished %s · ratio %.2f", ago(now-c), x.RatioValue())})
			}
		}
	case "stalled":
		what := map[string]string{"pause": "Pause", "reannounce": "Re-announce", "remove": "Remove, keep files"}[t.P.Action]
		for _, x := range ts {
			if !inScope(t, x) || x.Status != torrents.StatusDownloading {
				continue
			}
			idle := now - x.LastActive
			if x.Seeds == 0 || idle >= int64(t.P.Hours)*hour {
				why := "No seeds"
				if x.Seeds != 0 {
					why = "No data for " + torrentsEta(float64(idle)/1000)
				}
				items = append(items, Item{ID: x.ID, Name: x.Name, What: what, Why: why})
			}
		}
	case "disk":
		why := torrents.FormatBytes(free) + " free"
		if free < float64(t.P.GB)*torrents.GiB {
			for _, x := range ts {
				if x.Status == torrents.StatusDownloading {
					items = append(items, Item{ID: x.ID, Name: x.Name, What: "Pause", Why: why})
				}
			}
		} else if t.P.Resume {
			for _, x := range ts {
				if x.PausedBy == t.ID && x.Status == torrents.StatusPaused {
					items = append(items, Item{ID: x.ID, Name: x.Name, What: "Resume", Why: why})
				}
			}
		}
	case "verify":
		for _, x := range ts {
			if inScope(t, x) && x.Pct() >= 1 {
				items = append(items, Item{ID: x.ID, Name: x.Name, What: "Re-check pieces", Why: fmt.Sprintf("%d pieces", int(x.Size/x.PieceSize)+1)})
			}
		}
	case "logs":
		cut := now - int64(t.P.Days)*day
		for pi, p := range s.pruners {
			for _, e := range p.Old(cut) {
				items = append(items, Item{ID: e.ID, Ref: int64(pi), Name: p.Source() + ": " + e.Label, What: "Delete entry", Why: time.UnixMilli(e.T).Format("Jan 2, 3:04 PM")})
			}
		}
	}
	return items, nil
}

func torrentsEta(sec float64) string {
	switch {
	case sec < 60:
		return fmt.Sprintf("%ds", int(sec))
	case sec < 3600:
		return fmt.Sprintf("%dm", int(sec/60))
	case sec < 86400:
		return fmt.Sprintf("%dh %02dm", int(sec/3600), int(sec/60)%60)
	}
	return fmt.Sprintf("%dd %dh", int(sec/86400), int(sec/3600)%24)
}

func lower(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func (s *Service) done(ctx context.Context, t Task, n int, items []Item) string {
	free := torrents.FormatBytes(s.tor.Free())
	switch t.Type {
	case "retry":
		if n > 0 {
			return "Retried " + plural(n, "torrent")
		}
		return "No torrents had errors"
	case "cleanup":
		if n > 0 {
			return "Removed " + plural(n, "finished torrent")
		}
		return "No finished torrents to remove"
	case "stalled":
		if n > 0 {
			return "Handled " + plural(n, "stalled download")
		}
		return "No stalled downloads"
	case "disk":
		switch {
		case n == 0:
			return free + " free, nothing to do"
		case items != nil && items[0].What == "Resume":
			return fmt.Sprintf("Resumed %s, %s free", plural(n, "download"), free)
		}
		return fmt.Sprintf("Paused %s, only %s free", plural(n, "download"), free)
	case "verify":
		if n > 0 {
			return "Verified " + plural(n, "torrent") + ", all pieces match"
		}
		return "No finished torrents to verify"
	case "logs":
		switch n {
		case 0:
			return "No old entries"
		case 1:
			return "Deleted 1 old entry"
		}
		return fmt.Sprintf("Deleted %d old entries", n)
	}
	return ""
}

func (s *Service) Preview(ctx context.Context, id int64) (Preview, error) {
	t, err := s.tasks.Get(ctx, id)
	if err != nil {
		return Preview{}, err
	}
	items, err := s.find(ctx, t)
	if err != nil {
		return Preview{}, err
	}
	if items == nil {
		items = []Item{}
	}
	return Preview{Items: items, Done: s.done(ctx, t, 0, nil)}, nil
}

func (s *Service) apply(ctx context.Context, t Task, items []Item) error {
	ids := make([]int64, 0, len(items))
	for _, i := range items {
		if t.Type != "logs" {
			ids = append(ids, i.ID)
		}
	}
	now := s.nowMs()
	switch t.Type {
	case "retry":
		return s.tor.Mutate(ctx, func(ts []torrents.Torrent) {
			for i := range ts {
				if slices.Contains(ids, ts[i].ID) {
					ts[i].Status, ts[i].Err, ts[i].LastActive = torrents.StatusDownloading, "", now
					if ts[i].Seeds == 0 {
						ts[i].Seeds = 12 + rand.IntN(40)
					}
					s.bus.Emit("torrent.resumed", map[string]any{"torrent": torrents.Info(ts[i]), "reason": "maintenance"})
				}
			}
		})
	case "cleanup":
		_, err := s.tor.RemoveWithReason(ctx, ids, t.P.Files == "delete", "maintenance")
		return err
	case "stalled":
		if t.P.Action == "remove" {
			_, err := s.tor.RemoveWithReason(ctx, ids, false, "maintenance")
			return err
		}
		return s.tor.Mutate(ctx, func(ts []torrents.Torrent) {
			for i := range ts {
				if !slices.Contains(ids, ts[i].ID) {
					continue
				}
				if t.P.Action == "pause" {
					ts[i].Status, ts[i].Dl = torrents.StatusPaused, 0
					s.bus.Emit("torrent.paused", map[string]any{"torrent": torrents.Info(ts[i]), "reason": "maintenance"})
				} else {
					ts[i].Seeds += 5 + rand.IntN(20)
					ts[i].LastActive = now
				}
			}
		})
	case "disk":
		return s.tor.Mutate(ctx, func(ts []torrents.Torrent) {
			for _, it := range items {
				for i := range ts {
					if ts[i].ID != it.ID {
						continue
					}
					if it.What == "Pause" {
						ts[i].Status, ts[i].Dl, ts[i].PausedBy = torrents.StatusPaused, 0, t.ID
						s.bus.Emit("torrent.paused", map[string]any{"torrent": torrents.Info(ts[i]), "reason": "low_disk_space"})
					} else {
						ts[i].Status, ts[i].PausedBy = torrents.StatusDownloading, 0
						s.bus.Emit("torrent.resumed", map[string]any{"torrent": torrents.Info(ts[i]), "reason": "disk_space_freed"})
					}
				}
			}
		})
	case "logs":
		for _, it := range items {
			s.pruners[it.Ref].Remove(it.ID)
		}
	}
	return nil
}

func (s *Service) run(ctx context.Context, t *Task) (int, error) {
	items, err := s.find(ctx, *t)
	if err != nil {
		return 0, err
	}
	if err := s.apply(ctx, *t, items); err != nil {
		return 0, err
	}
	now := s.nowMs()
	t.LastRun, t.NextRun = now, now+interval(*t)
	t.LastText = s.done(ctx, *t, len(items), items)
	names := make([]string, 0, 8)
	for i, it := range items {
		if i < 8 {
			names = append(names, it.Name)
		}
	}
	kind := "none"
	if len(items) > 0 {
		kind = "ok"
	}
	_, _ = s.log.Create(ctx, LogEntry{T: now, Task: metas[t.Type].title, Kind: kind, Text: t.LastText, Added: names, More: max(0, len(items)-8)})
	if all, err := s.log.List(ctx); err == nil && len(all) > 60 {
		for _, l := range all[:len(all)-60] {
			_ = s.log.Delete(ctx, l.ID)
		}
	}
	all := make([]string, len(items))
	for i, it := range items {
		all[i] = it.Name
	}
	s.bus.Emit("maintenance.run", map[string]any{"task": map[string]any{"id": t.ID, "name": metas[t.Type].title}, "count": len(items), "result": t.LastText, "items": all})
	return len(items), nil
}

type RunResult struct {
	Count  int    `json:"count"`
	Result string `json:"result"`
}

func (s *Service) Run(ctx context.Context, id int64) (RunResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.tasks.Get(ctx, id)
	if err != nil {
		return RunResult{}, err
	}
	n, err := s.run(ctx, &t)
	if err != nil {
		return RunResult{}, err
	}
	return RunResult{Count: n, Result: t.LastText}, s.tasks.Put(ctx, t)
}

func (s *Service) SetEnabled(ctx context.Context, id int64, on bool) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.tasks.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	t.Enabled = on
	if on {
		t.NextRun = s.nowMs() + interval(t)
	}
	return s.view(t), s.tasks.Put(ctx, t)
}

func whole(label string, v, lo, hi int) error {
	if v < lo || v > hi {
		return Invalid(fmt.Sprintf("Enter a whole number from %d to %d for “%s”.", lo, hi, label))
	}
	return nil
}

func validate(t Task) error {
	if err := whole("Run every", t.Every, 1, 999); err != nil {
		return err
	}
	if !slices.Contains([]string{"minutes", "hours", "days"}, t.Unit) {
		return Invalid("Choose minutes, hours or days.")
	}
	p := t.P
	switch t.Type {
	case "cleanup":
		if err := whole("Finished at least", p.Days, 0, 3650); err != nil {
			return err
		}
		if p.Ratio < 0 || p.Ratio > 9999 {
			return Invalid("Enter a number from 0 to 9999 for “With a share ratio of at least”.")
		}
		if !slices.Contains([]string{"keep", "delete"}, p.Files) {
			return Invalid("Choose whether to keep or delete the files.")
		}
	case "stalled":
		if err := whole("No data received for", p.Hours, 1, 720); err != nil {
			return err
		}
		if !slices.Contains([]string{"pause", "reannounce", "remove"}, p.Action) {
			return Invalid("Choose what to do with stalled downloads.")
		}
	case "disk":
		return whole("Free space below", p.GB, 1, 100000)
	case "logs":
		return whole("Entries older than", p.Days, 0, 3650)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, in Task) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.tasks.Get(ctx, in.ID)
	if err != nil {
		return View{}, err
	}
	next := t
	next.Every, next.Unit, next.Tag, next.P = in.Every, in.Unit, in.Tag, in.P
	if !metas[t.Type].scoped {
		next.Tag = ""
	}
	if err := validate(next); err != nil {
		return View{}, err
	}
	if next.Every != t.Every || next.Unit != t.Unit {
		next.NextRun = s.nowMs() + interval(next)
	}
	return s.view(next), s.tasks.Put(ctx, next)
}

func (s *Service) Log(ctx context.Context) ([]LogEntry, error) {
	items, err := s.log.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	for i := range items {
		if items[i].Added == nil {
			items[i].Added = []string{}
		}
	}
	return items, nil
}

func (s *Service) Source() string { return "Maintenance" }

func (s *Service) Old(cutoff int64) []OldEntry {
	items, err := s.log.List(context.Background())
	if err != nil {
		return nil
	}
	var out []OldEntry
	for _, l := range items {
		if l.T < cutoff {
			out = append(out, OldEntry{ID: l.ID, Label: l.Task, T: l.T})
		}
	}
	return out
}

func (s *Service) Remove(id int64) { _ = s.log.Delete(context.Background(), id) }

func (s *Service) Tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.tasks.List(ctx)
	if err != nil {
		return
	}
	now := s.nowMs()
	for _, t := range ts {
		if t.Enabled && t.NextRun <= now {
			if _, err := s.run(ctx, &t); err == nil {
				_ = s.tasks.Put(ctx, t)
			}
		}
	}
}

func (s *Service) Loop(ctx context.Context) {
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			s.Tick(ctx)
		}
	}
}
