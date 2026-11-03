package torrents

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"bitswarm/server/internal/settings"
)

var activeStates = []Status{StatusDownloading, StatusSeeding, StatusChecking, StatusRechecking}

func isActive(t Torrent) bool { return slices.Contains(activeStates, t.Status) }

func prioList(t Torrent) []string {
	out := make([]string, len(t.Files))
	for i, f := range t.Files {
		out[i] = f.Prio
	}
	return out
}

func nameList(t Torrent) []string {
	out := make([]string, len(t.Files))
	for i, f := range t.Files {
		out[i] = f.Name
	}
	return out
}

func restoreOpts(t Torrent) RestoreOptions {
	return RestoreOptions{
		AddOptions: AddOptions{Dir: t.Path, Layout: t.Layout, Prios: prioList(t)},
		Magnet:     t.Magnet, Names: nameList(t),
	}
}

func (s *Service) addReal(ctx context.Context, a AddSpec) (View, error) {
	if a.TorrentURL != "" && len(a.FileData) == 0 {
		data, err := fetchTorrent(ctx, a.TorrentURL)
		if err != nil {
			return View{}, err
		}
		a.FileData = data
	}
	src := Source{Magnet: strings.TrimSpace(a.Magnet), Torrent: a.FileData}
	switch {
	case len(src.Torrent) > 0:
		if low := strings.ToLower(a.FileName); a.FileName != "" && !strings.HasSuffix(low, ".torrent") {
			return View{}, Invalid("That file isn’t a .torrent file. Choose a file ending in .torrent.")
		}
	case src.Magnet != "":
		if _, _, err := NameFromMagnet(src.Magnet); err != nil {
			return View{}, err
		}
	default:
		return View{}, Invalid("Paste a magnet link or choose a .torrent file first.")
	}
	if a.Seq || a.Flp {
		return View{}, Invalid("Sequential download and first-and-last-piece download aren’t supported by the BitTorrent engine yet. Turn those options off.")
	}
	hash, err := s.eng.Hash(src)
	if err != nil {
		return View{}, err
	}
	existing, err := s.repo.List(ctx)
	if err != nil {
		return View{}, err
	}
	for _, e := range existing {
		if e.Hash == hash {
			return View{}, Invalid("This torrent is already in your list.")
		}
	}
	set := s.set.Get()
	path := strings.TrimSpace(a.Path)
	if path == "" {
		path = set.SavePath
	}
	if err := s.checkPath(path); err != nil {
		return View{}, err
	}
	layout := a.Layout
	if layout == "" {
		layout = "nosub"
		if set.CreateSubfolder {
			layout = "original"
		}
	}

	var prios []string
	if a.Selected != nil {
		if len(a.Selected) == 0 {
			return View{}, Invalid("Select at least one file to download.")
		}
		if pm, perr := s.eng.Meta(ctx, src); perr == nil {
			prios = make([]string, len(pm.Files))
			for i := range prios {
				prios[i] = "skip"
			}
			for _, i := range a.Selected {
				if i >= 0 && i < len(prios) {
					prios[i] = "normal"
				}
			}
		}
	}
	meta, err := s.eng.Add(src, AddOptions{Dir: path, Layout: layout, Prios: prios})
	if err != nil {
		return View{}, err
	}

	next, minQueue := int64(1), 1<<30
	for _, e := range existing {
		next = max(next, e.ID+1)
		minQueue = min(minQueue, e.Queue)
	}
	now := s.nowMs()
	t := Torrent{
		ID: next, Hash: hash, Name: meta.Name, Size: meta.Size, PieceSize: meta.PieceSize, Files: meta.Files,
		Path: path, Layout: layout, Magnet: src.Magnet, Added: now, LastActive: now,
		Queue: int(next), Presets: []int64{}, Tags: a.Tags,
	}
	if t.Tags == nil {
		t.Tags = []string{}
	}
	t.Trackers = meta.Trackers
	if len(meta.Trackers) > 0 {
		t.Tracker = meta.Trackers[0]
	}
	if t.Name == "" {
		t.Name = firstNonEmpty(a.Name, "Magnet "+hash[:8])
	}
	for i := range t.Files {
		if prios != nil && i < len(prios) {
			t.Files[i].Prio = prios[i]
		}
	}
	if a.Preset > 0 {
		t.Presets = []int64{a.Preset}
	}
	if a.Top && minQueue != 1<<30 {
		t.Queue = minQueue - 1
	}
	downloading := 0
	for _, e := range existing {
		if (e.Status == StatusDownloading || e.Status == StatusChecking) && !e.Force {
			downloading++
		}
	}
	switch {
	case !a.Start && set.Queueing && a.Top:
		t.Status = StatusQueued
	case a.Start && set.Queueing && downloading >= set.MaxActiveDl:
		t.Status = StatusQueued
	case !a.Start:
		t.Status = StatusPaused
	case len(t.Files) == 0:
		t.Status = StatusChecking
	default:
		t.Status = StatusDownloading
	}
	if !isActive(t) {
		s.eng.Unload(hash)
	}

	created, err := s.repo.Create(ctx, t)
	if err != nil {
		_ = s.eng.Forget(hash, false)
		return View{}, err
	}
	s.cat.EnsureTags(t.Tags)

	data := map[string]any{"torrent": Info(created), "source": orDefault(a.Source, "manual"), "files_selected": len(created.Files), "files_total": len(created.Files), "skip_hash_check": false}
	if a.Schedule != nil {
		data["schedule"] = a.Schedule
		data["link"] = a.Link
	}
	s.bus.Emit("torrent.added", data)
	if s.renamer != nil && len(created.Files) > 0 {
		old := nameList(created)
		s.renamer.ApplyRenames(ctx, &created, "add")
		s.applyNames(&created, old)
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

func (s *Service) applyNames(t *Torrent, old []string) {
	if s.eng == nil || len(t.Files) == 0 || slices.Equal(old, nameList(*t)) {
		return
	}
	if !s.eng.Loaded(t.Hash) {
		opts := restoreOpts(*t)
		opts.Names = old
		if err := s.eng.Restore(t.Hash, opts); err != nil {
			s.log.Add("error", "Torrents", "Couldn’t rename the files of "+t.Name+": "+err.Error())
			return
		}
	}
	want := nameList(*t)
	final, err := s.eng.SetNames(t.Hash, want)
	if err != nil {
		s.log.Add("error", "Torrents", "Couldn’t rename the files of "+t.Name+" on disk: "+err.Error())
		return
	}
	for i := range t.Files {
		if i >= len(final) || final[i] == want[i] {
			continue
		}
		if len(t.Files) == 1 && t.Name == path.Base(want[i]) {
			t.Name = path.Base(final[i])
		}
		t.Files[i].Name = final[i]
		s.log.Add("info", "Torrents", "“"+want[i]+"” already exists, so "+t.Name+" was saved as “"+final[i]+"”")
	}
}

func (s *Service) ensureLoaded(t Torrent) error {
	if s.eng.Loaded(t.Hash) {
		return nil
	}
	return s.eng.Restore(t.Hash, restoreOpts(t))
}

func (s *Service) fail(t *Torrent, msg string) {
	t.Status, t.Err, t.Dl, t.Ul = StatusError, msg, 0, 0
	s.eng.Unload(t.Hash)
	s.bus.Emit("torrent.error", map[string]any{"torrent": Info(*t), "error": msg})
}

func (s *Service) syncReal(ctx context.Context, ts []Torrent, set settings.Settings, now int64) (td, tu float64) {
	capD, capU := set.DlLimit, set.UlLimit
	if AltActive(set, s.alt, s.now()) {
		capD, capU = set.AltDl, set.AltUl
	}
	s.eng.SetRates(float64(capD)*KiB, float64(capU)*KiB)

	for i := range ts {
		t := &ts[i]
		if !isActive(*t) {
			if s.eng.Loaded(t.Hash) {
				s.eng.Unload(t.Hash)
			}
			delete(s.rechk, t.ID)
			t.Dl, t.Ul = 0, 0
			t.Peers = nil
			continue
		}
		if !s.eng.Loaded(t.Hash) {
			if err := s.eng.Restore(t.Hash, restoreOpts(*t)); err != nil {
				s.fail(t, err.Error())
				continue
			}
		}
		l, ok := s.eng.Live(t.Hash)
		if !ok {
			continue
		}
		if l.Err != "" {
			s.fail(t, l.Err)
			continue
		}
		if !l.HaveInfo {
			t.Status, t.Dl, t.Ul = StatusChecking, 0, 0
			continue
		}
		if len(t.Files) == 0 {
			s.adoptMeta(ctx, t, l)
		}
		if t.Status == StatusRechecking {
			if !s.rechk[t.ID] {
				s.rechk[t.ID] = true
				if err := s.eng.Recheck(t.Hash); err != nil {
					s.fail(t, err.Error())
					continue
				}
				t.Dl, t.Ul = 0, 0
				continue
			}
			if l.Verifying {
				continue
			}
			delete(s.rechk, t.ID)
			t.Status = t.RecheckTo
			if t.Status == "" {
				t.Status = StatusDownloading
			}
			s.log.Add("info", "Torrents", "Recheck finished for "+t.Name)
		}

		var done float64
		for j := range t.Files {
			if j < len(l.FileDone) {
				t.Files[j].Done = l.FileDone[j]
				if t.Files[j].Prio != "skip" {
					done += l.FileDone[j]
				}
			}
		}
		t.Done = done
		t.Dl, t.Ul = l.Dl, l.Ul
		t.Seeds, t.Leech, t.Peers = l.Seeds, l.Leech, l.Peers
		t.Uploaded += l.UlDelta
		td += l.DlDelta
		tu += l.UlDelta
		if l.Dl > 1 {
			t.LastActive = now
		}
		w := t.Wanted()
		if t.Status == StatusChecking {
			t.Status = StatusDownloading
		}
		if t.Status == StatusDownloading && w > 0 && t.Done >= w {
			t.Status, t.Dl, t.SeedSince = StatusSeeding, 0, now
			if s.renamer != nil {
				old := nameList(*t)
				s.renamer.ApplyRenames(ctx, t, "complete")
				s.applyNames(t, old)
			}
			s.bus.Emit("torrent.completed", map[string]any{"torrent": Info(*t)})
		} else if t.Status == StatusSeeding && t.Done < w {
			t.Status = StatusDownloading
		}
		t.Hist.Dl = trim(append(t.Hist.Dl, t.Dl))
		t.Hist.Ul = trim(append(t.Hist.Ul, t.Ul))
	}
	return td, tu
}

func (s *Service) adoptMeta(ctx context.Context, t *Torrent, l Live) {
	t.Name = firstNonEmpty(l.Meta.Name, t.Name)
	t.Size, t.PieceSize, t.Files = l.Meta.Size, l.Meta.PieceSize, l.Meta.Files
	if len(l.Meta.Trackers) > 0 {
		t.Trackers = l.Meta.Trackers
		if t.Tracker == "" {
			t.Tracker = l.Meta.Trackers[0]
		}
	}
	s.bus.Emit("torrent.metadata", map[string]any{"torrent": Info(*t)})
	if s.renamer != nil {
		old := nameList(*t)
		s.renamer.ApplyRenames(ctx, t, "add")
		s.applyNames(t, old)
	}
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func (s *Service) Preview(ctx context.Context, src Source, timeout time.Duration) (string, Contents, error) {
	if s.eng == nil {
		return "", Contents{}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	m, err := s.eng.Meta(ctx, src)
	if err != nil {
		return "", Contents{}, err
	}
	return m.Name, Contents{Size: m.Size, Files: m.Files}, nil
}

func (s *Service) Real() bool { return s.eng != nil }

const maxTorrentFile = 8 << 20

func fetchTorrent(ctx context.Context, raw string) ([]byte, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, Invalid("The link " + raw + " isn’t a web address BitSwarm can download from.")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, Invalid("The link " + raw + " isn’t valid.")
	}
	req.Header.Set("User-Agent", "BitSwarm")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, Invalid(fmt.Sprintf("Couldn’t download the .torrent file from %s: %v", u.Host, err))
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, Invalid(fmt.Sprintf("%s answered %s when BitSwarm asked for the .torrent file.", u.Host, res.Status))
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, maxTorrentFile+1))
	if err != nil {
		return nil, Invalid(fmt.Sprintf("Couldn’t read the .torrent file from %s: %v", u.Host, err))
	}
	if len(data) > maxTorrentFile {
		return nil, Invalid("That .torrent file is larger than 8 MB, so BitSwarm won’t load it.")
	}
	return data, nil
}
