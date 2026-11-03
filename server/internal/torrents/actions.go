package torrents

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type BatchReq struct {
	Action     string   `json:"action"`
	IDs        []int64  `json:"ids"`
	DeleteData bool     `json:"deleteData"`
	How        string   `json:"how"`
	Tag        string   `json:"tag"`
	Tags       []string `json:"tags"`
	On         *bool    `json:"on"`
	Path       string   `json:"path"`
	MoveFiles  *bool    `json:"moveFiles"`
	DlLimit    *int     `json:"dlLimit"`
	UlLimit    *int     `json:"ulLimit"`
	RatioMode  string   `json:"ratioMode"`
	Ratio      float64  `json:"ratio"`
	Preset     int64    `json:"preset"`
}

func (s *Service) selection(ctx context.Context, ids []int64) (sel, all []Torrent, err error) {
	all, err = s.repo.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, t := range all {
		if slices.Contains(ids, t.ID) {
			sel = append(sel, t)
		}
	}
	return sel, all, nil
}

func describe(sel []Torrent) string {
	if len(sel) == 1 {
		return sel[0].Name
	}
	return Plural(len(sel), "torrent")
}

func (s *Service) Batch(ctx context.Context, r BatchReq) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sel, all, err := s.selection(ctx, r.IDs)
	if err != nil {
		return "", err
	}
	if len(sel) == 0 {
		return "", ErrNotFound
	}
	one := describe(sel)
	msg := ""

	switch r.Action {
	case "resume":
		n := 0
		for i := range sel {
			t := &sel[i]
			switch {
			case t.Status == StatusPaused || t.Status == StatusError || t.Status == StatusQueued:
				t.Status = StatusDownloading
				if t.Pct() >= 1 {
					t.Status = StatusSeeding
				}
				t.Err = ""
			case t.Status == StatusDone:
				t.Status = StatusSeeding
			default:
				continue
			}
			n++
			s.bus.Emit("torrent.resumed", map[string]any{"torrent": Info(*t)})
		}
		if n > 0 {
			msg = "Resumed " + Plural(n, "torrent")
		}
	case "pause":
		n := 0
		for i := range sel {
			t := &sel[i]
			if !t.CanPause() {
				continue
			}
			t.Status = StatusPaused
			if t.Pct() >= 1 {
				t.Status = StatusDone
			}
			t.Dl, t.Ul = 0, 0
			n++
			s.bus.Emit("torrent.paused", map[string]any{"torrent": Info(*t), "reason": "manual"})
		}
		if n > 0 {
			msg = "Paused " + Plural(n, "torrent")
		}
	case "remove":
		for _, t := range sel {
			s.bus.Emit("torrent.removed", map[string]any{"torrent": Info(t), "deleted_files": r.DeleteData, "reason": "manual"})
			if r.DeleteData {
				s.free += t.Done
			}
			if err := s.forget(t, r.DeleteData); err != nil {
				return "", err
			}
			if err := s.repo.Delete(ctx, t.ID); err != nil {
				return "", err
			}
		}
		msg = "Removed " + Plural(len(sel), "torrent")
		if r.DeleteData {
			msg += " and deleted data"
		}
		return msg, nil
	case "force":
		on := slices.ContainsFunc(sel, func(t Torrent) bool { return !t.Force })
		for i := range sel {
			t := &sel[i]
			t.Force = on
			if on && !slices.Contains([]Status{StatusDownloading, StatusSeeding, StatusChecking, StatusRechecking}, t.Status) {
				t.Status = StatusDownloading
				if t.Pct() >= 1 {
					t.Status = StatusSeeding
				}
				t.Err = ""
				s.bus.Emit("torrent.resumed", map[string]any{"torrent": Info(*t), "reason": "force_start"})
			}
		}
		if on {
			msg = "Force started " + one
			s.log.Add("info", "Torrents", "Force started "+one+". Queue limits don’t apply.")
		} else {
			msg = "Stopped forcing " + one
			s.log.Add("info", "Torrents", "Stopped forcing "+one+". Queue limits apply again.")
		}
	case "recheck":
		for i := range sel {
			t := &sel[i]
			if t.Status == StatusChecking || t.Status == StatusRechecking {
				continue
			}
			if t.CanResume() {
				t.RecheckTo = StatusPaused
				if t.Pct() >= 1 {
					t.RecheckTo = StatusDone
				}
			} else {
				t.RecheckTo = StatusDownloading
				if t.Pct() >= 1 {
					t.RecheckTo = StatusSeeding
				}
			}
			t.Status, t.CheckLeft, t.Dl, t.Ul = StatusRechecking, 4, 0, 0
		}
		msg = "Checking the files of " + one
		s.log.Add("info", "Torrents", "Started a recheck of "+one)
	case "reannounce":
		for i := range sel {
			t := &sel[i]
			if t.Status == StatusError {
				t.Status = StatusDownloading
				if t.Pct() >= 1 {
					t.Status = StatusSeeding
				}
				t.Err = ""
			}
			if s.eng != nil {
				s.eng.Reannounce(t.Hash)
				continue
			}
			t.Seeds += 3 + int(s.rnd()*15)
			t.Leech += int(s.rnd() * 5)
			t.LastActive = s.nowMs()
		}
		msg = "Asked the trackers for more peers for " + one
		if s.eng != nil {
			msg = "Asked the DHT network for more peers for " + one
		}
		their := "its"
		if len(sel) > 1 {
			their = "their"
		}
		s.log.Add("info", "Torrents", fmt.Sprintf("Reannounced %s to %s trackers", one, their))
	case "seq", "flp":
		if s.eng != nil {
			return "", Invalid("Sequential download and first-and-last-piece download aren’t supported by the BitTorrent engine yet.")
		}
		on := slices.ContainsFunc(sel, func(t Torrent) bool {
			if r.Action == "seq" {
				return !t.Seq
			}
			return !t.Flp
		})
		for i := range sel {
			if r.Action == "seq" {
				sel[i].Seq = on
			} else {
				sel[i].Flp = on
			}
		}
		what := "First and last pieces first"
		if r.Action == "seq" {
			what = "Sequential download"
		}
		state := "off"
		if on {
			state = "on"
		}
		msg = fmt.Sprintf("%s %s for %s", what, state, one)
		s.log.Add("info", "Torrents", fmt.Sprintf("%s turned %s for %s", what, state, one))
	case "queue":
		msg, err = s.moveQueue(all, sel, r.How)
		if err != nil {
			return "", err
		}
		return msg, s.repo.PutAll(ctx, all)
	case "tags":
		msg = s.applyTags(sel, all, r, one)
		if msg == "" {
			return "", Invalid("Enter a tag name.")
		}
	case "move":
		path := strings.TrimRight(strings.TrimSpace(r.Path), "/")
		if !pathRe.MatchString(path) {
			return "", Invalid("Enter a full folder path, such as /downloads/movies.")
		}
		if err := s.checkPath(path); err != nil {
			return "", err
		}
		moved := r.MoveFiles == nil || *r.MoveFiles
		for i := range sel {
			if sel[i].Path == path {
				continue
			}
			from := sel[i].Path
			if s.eng != nil {
				if err := s.ensureLoaded(sel[i]); err != nil {
					return "", Invalid("Couldn’t move " + sel[i].Name + ": " + err.Error())
				}
				if err := s.eng.Move(sel[i].Hash, path, moved); err != nil {
					return "", Invalid("Couldn’t move " + sel[i].Name + ": " + err.Error())
				}
			}
			sel[i].Path = path
			s.bus.Emit("torrent.moved", map[string]any{"torrent": Info(sel[i]), "from": from, "to": path, "moved_files": moved})
		}
		msg = fmt.Sprintf("Moved %s to %s", one, path)
	case "limits":
		if r.DlLimit != nil && (*r.DlLimit < 0 || *r.DlLimit > 10000000) {
			return "", Invalid("The download limit must be a whole number of KiB/s, 0 or more.")
		}
		if r.UlLimit != nil && (*r.UlLimit < 0 || *r.UlLimit > 10000000) {
			return "", Invalid("The upload limit must be a whole number of KiB/s, 0 or more.")
		}
		if s.eng != nil && (r.DlLimit != nil && *r.DlLimit > 0 || r.UlLimit != nil && *r.UlLimit > 0) {
			return "", Invalid("Speed limits for a single torrent aren’t supported by the BitTorrent engine yet. Set limits for everything in Settings → Speed.")
		}
		if r.RatioMode == "custom" && !(r.Ratio > 0 && r.Ratio <= 9999) {
			return "", Invalid("The ratio must be a number greater than 0, such as 1.5 or 2.")
		}
		for i := range sel {
			if r.DlLimit != nil {
				sel[i].DlLimit = *r.DlLimit
			}
			if r.UlLimit != nil {
				sel[i].UlLimit = *r.UlLimit
			}
			switch r.RatioMode {
			case "none":
				v := -1.0
				sel[i].RatioLimit = &v
			case "custom":
				v := r.Ratio
				sel[i].RatioLimit = &v
			default:
				sel[i].RatioLimit = nil
			}
		}
		msg = "Limits saved for " + one
		s.log.Add("info", "Torrents", fmt.Sprintf("Limits for %s: download %s, upload %s, ratio %s", one, limitText(r.DlLimit), limitText(r.UlLimit), ratioText(r)))
	case "preset":
		if r.Preset > 0 {
			for i := range sel {
				sel[i].Presets = []int64{r.Preset}
			}
		}
		msg = "Set the rename preset for " + one
	default:
		return "", Invalid("Unknown action: " + r.Action)
	}

	return msg, s.repo.PutAll(ctx, sel)
}

func limitText(v *int) string {
	switch {
	case v == nil:
		return "unchanged"
	case *v > 0:
		return FormatBytes(float64(*v)*KiB) + "/s"
	}
	return "unlimited"
}

func ratioText(r BatchReq) string {
	switch r.RatioMode {
	case "none":
		return "no limit"
	case "custom":
		return fmt.Sprintf("%g", r.Ratio)
	}
	return "global setting"
}

func (s *Service) applyTags(sel, all []Torrent, r BatchReq, one string) string {
	setTags := func(fn func([]string) []string) {
		for i := range sel {
			before := strings.Join(sel[i].Tags, "\x00")
			sel[i].Tags = fn(slices.Clone(sel[i].Tags))
			if strings.Join(sel[i].Tags, "\x00") != before {
				s.bus.Emit("torrent.tagged", map[string]any{"torrent": Info(sel[i])})
			}
		}
	}
	switch r.How {
	case "toggle":
		on := r.On != nil && *r.On
		g := r.Tag
		if g == "" {
			return ""
		}
		s.cat.EnsureTags([]string{g})
		setTags(func(tags []string) []string {
			if on {
				if !slices.Contains(tags, g) {
					tags = append(tags, g)
				}
				return tags
			}
			return slices.DeleteFunc(tags, func(x string) bool { return x == g })
		})
		if on {
			return fmt.Sprintf("Tagged %s #%s", one, g)
		}
		return fmt.Sprintf("Removed #%s from %s", g, one)
	case "add":
		if len(r.Tags) == 0 {
			return ""
		}
		s.cat.EnsureTags(r.Tags)
		setTags(func(tags []string) []string {
			for _, g := range r.Tags {
				if !slices.Contains(tags, g) {
					tags = append(tags, g)
				}
			}
			return tags
		})
		hashed := make([]string, len(r.Tags))
		for i, g := range r.Tags {
			hashed[i] = "#" + g
		}
		return fmt.Sprintf("Tagged %s %s", one, strings.Join(hashed, " "))
	case "clear":
		setTags(func([]string) []string { return []string{} })
		return "Removed all tags from the selection"
	case "prune":
		used := map[string]bool{}
		merged := append([]Torrent{}, all...)
		for _, t := range merged {
			for _, g := range t.Tags {
				used[g] = true
			}
		}
		unused := []string{}
		for _, g := range s.cat.TagNames() {
			if !used[g] {
				unused = append(unused, g)
			}
		}
		s.cat.RemoveTags(unused)
		s.log.Add("info", "Torrents", "Deleted unused tags: "+strings.Join(unused, ", "))
		return "Deleted " + Plural(len(unused), "unused tag")
	}
	return ""
}

func (s *Service) moveQueue(all, sel []Torrent, how string) (string, error) {
	inc := []int{}
	for i := range all {
		if all[i].Unfinished() {
			inc = append(inc, i)
		}
	}
	slices.SortStableFunc(inc, func(a, b int) int { return all[a].Queue - all[b].Queue })
	selected := map[int64]bool{}
	for _, t := range sel {
		if t.Unfinished() {
			selected[t.ID] = true
		}
	}
	if len(selected) == 0 {
		return "", Invalid("Only unfinished torrents have a place in the queue")
	}

	var order []int
	var chosen []int
	for _, i := range inc {
		if selected[all[i].ID] {
			chosen = append(chosen, i)
		} else {
			order = append(order, i)
		}
	}
	switch how {
	case "top":
		order = append(chosen, order...)
	case "bottom":
		order = append(order, chosen...)
	case "up", "down":
		order = slices.Clone(inc)
		step := 1
		walkSel := slices.Clone(chosen)
		if how == "up" {
			step = -1
		} else {
			slices.Reverse(walkSel)
		}
		for _, t := range walkSel {
			i := slices.Index(order, t)
			j := i + step
			if j < 0 || j >= len(order) || selected[all[order[j]].ID] {
				continue
			}
			order[i], order[j] = order[j], order[i]
		}
	default:
		return "", Invalid("Unknown queue move: " + how)
	}
	for pos, i := range order {
		all[i].Queue = pos + 1
	}
	one := describe(sel)
	where := map[string]string{"top": "to the top of", "bottom": "to the bottom of", "up": "up in", "down": "down in"}[how]
	msg := fmt.Sprintf("Moved %s %s the queue", one, where)
	s.log.Add("info", "Torrents", msg)
	return msg, nil
}

func (s *Service) SetPriority(ctx context.Context, id int64, idx int, prio string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains([]string{"high", "normal", "skip"}, prio) {
		return Invalid("Priority must be high, normal or skip.")
	}
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if idx < 0 || idx >= len(t.Files) {
		return ErrNotFound
	}
	wasDone := t.Pct() >= 1
	t.Files[idx].Prio = prio
	if s.eng != nil {
		s.eng.SetPriorities(t.Hash, prioList(t))
	}
	if t.Done > t.Wanted() {
		t.Done = t.Wanted()
	}
	if !wasDone && t.Pct() >= 1 && t.Status == StatusDownloading {
		t.Status, t.SeedSince = StatusSeeding, s.nowMs()
	} else if wasDone && t.Pct() < 1 && t.Status == StatusSeeding {
		t.Status = StatusDownloading
	}
	return s.repo.Put(ctx, t)
}

func (s *Service) Retry(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	t.Status, t.Err = StatusDownloading, ""
	if s.eng == nil {
		t.Seeds = 37
	}
	return s.repo.Put(ctx, t)
}

func (s *Service) forget(t Torrent, deleteData bool) error {
	if s.eng == nil {
		return nil
	}
	if deleteData {
		_ = s.ensureLoaded(t)
	}
	if err := s.eng.Forget(t.Hash, deleteData); err != nil {
		return Invalid("Couldn’t delete the files of " + t.Name + ": " + err.Error())
	}
	return nil
}
