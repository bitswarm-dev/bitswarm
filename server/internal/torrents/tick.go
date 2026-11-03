package torrents

import (
	"context"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"bitswarm/server/internal/settings"
)

func minutesOf(s string) int {
	h, m, _ := strings.Cut(s, ":")
	hh, _ := strconv.Atoi(h)
	mm, _ := strconv.Atoi(m)
	return hh*60 + mm
}

func AltActive(set settings.Settings, manual bool, at time.Time) bool {
	if manual {
		return true
	}
	if !set.AltSchedule {
		return false
	}
	day := at.Weekday()
	weekend := day == time.Saturday || day == time.Sunday
	if set.AltDays == "weekdays" && weekend || set.AltDays == "weekends" && !weekend {
		return false
	}
	m, a, b := at.Hour()*60+at.Minute(), minutesOf(set.AltFrom), minutesOf(set.AltTo)
	if a <= b {
		return m >= a && m < b
	}
	return m >= a || m < b
}

func capOf(kib int) float64 {
	if kib > 0 {
		return float64(kib) * KiB
	}
	return math.Inf(1)
}

func (s *Service) Run(ctx context.Context) {
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			if err := s.Tick(ctx); err != nil {
				s.log.Add("error", "App", "Engine tick failed: "+err.Error())
			}
		}
	}
}

func (s *Service) Tick(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	set := s.set.Get()
	ts, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	now := s.nowMs()
	s.startQueued(ts, set)

	var td, tu float64
	if s.eng != nil {
		td, tu = s.syncReal(ctx, ts, set, now)
	} else {
		td, tu = s.simulate(ctx, ts, set, now)
	}
	s.sessDl += td
	s.sessUl += tu

	var removed []int64
	if set.RatioOn || set.SeedTimeOn || anyRatioLimit(ts) {
		for i := range ts {
			if id := s.applySeedLimit(&ts[i], set, now); id != 0 {
				removed = append(removed, id)
			}
		}
	}

	s.pushHist(td, tu, chartWindow(set.SpeedWindow))

	keep := ts[:0]
	for _, t := range ts {
		if !slices.Contains(removed, t.ID) {
			keep = append(keep, t)
			continue
		}
		if s.eng != nil {
			_ = s.eng.Forget(t.Hash, set.SeedAction == "removeData")
		}
		if err := s.repo.Delete(ctx, t.ID); err != nil {
			return err
		}
	}
	return s.repo.PutAll(ctx, keep)
}

func (s *Service) startQueued(ts []Torrent, set settings.Settings) {
	active := 0
	for _, t := range ts {
		if (t.Status == StatusDownloading || t.Status == StatusChecking) && !t.Force {
			active++
		}
	}
	if set.Queueing && active >= set.MaxActiveDl {
		return
	}
	best := -1
	for i := range ts {
		if ts[i].Status == StatusQueued && (best < 0 || ts[i].Queue < ts[best].Queue) {
			best = i
		}
	}
	if best >= 0 {
		ts[best].Status = StatusDownloading
		s.log.Add("info", "Torrents", "Started "+ts[best].Name+" from the queue")
	}
}

func (s *Service) simulate(ctx context.Context, ts []Torrent, set settings.Settings, now int64) (td, tu float64) {
	for i := range ts {
		t := &ts[i]
		switch t.Status {
		case StatusRechecking:
			t.Dl, t.Ul = 0, 0
			if t.CheckLeft--; t.CheckLeft <= 0 {
				t.Status = t.RecheckTo
				if t.Status == "" {
					t.Status = StatusDownloading
					if t.Pct() >= 1 {
						t.Status = StatusSeeding
					}
				}
				s.log.Add("info", "Torrents", "Recheck finished for "+t.Name+": all "+strconv.Itoa(int(math.Ceil(t.Size/t.PieceSize)))+" pieces match")
			}
		case StatusChecking:
			t.Dl, t.Ul = 0, 0
			if t.CheckLeft--; t.CheckLeft <= 0 {
				t.Status = StatusDownloading
				s.bus.Emit("torrent.metadata", map[string]any{"torrent": Info(*t)})
			}
		case StatusDownloading:
			s.stepSpeeds(t)
			if t.Dl > 1 {
				t.LastActive = now
			}
		case StatusSeeding:
			t.Dl = 0
			s.stepSpeeds(t)
		default:
			t.Dl, t.Ul = 0, 0
		}
	}

	for i := range ts {
		if ts[i].DlLimit > 0 {
			ts[i].Dl = math.Min(ts[i].Dl, float64(ts[i].DlLimit)*KiB)
		}
		if ts[i].UlLimit > 0 {
			ts[i].Ul = math.Min(ts[i].Ul, float64(ts[i].UlLimit)*KiB)
		}
	}
	alt := AltActive(set, s.alt, s.now())
	capD, capU := capOf(set.DlLimit), capOf(set.UlLimit)
	if alt {
		capD, capU = capOf(set.AltDl), capOf(set.AltUl)
	}
	var sd, su float64
	for _, t := range ts {
		sd += t.Dl
		su += t.Ul
	}
	for i := range ts {
		if sd > capD {
			ts[i].Dl *= capD / sd
		}
		if su > capU {
			ts[i].Ul *= capU / su
		}
	}

	for i := range ts {
		t := &ts[i]
		if t.Status == StatusDownloading {
			w := t.Wanted()
			t.Done = math.Min(w, t.Done+t.Dl)
			if t.Done >= w {
				t.Status, t.Dl, t.SeedSince = StatusSeeding, 0, now
				if s.renamer != nil {
					s.renamer.ApplyRenames(ctx, t, "complete")
				}
				s.bus.Emit("torrent.completed", map[string]any{"torrent": Info(*t)})
			}
		}
		t.Uploaded += t.Ul
		td += t.Dl
		tu += t.Ul
		s.free -= t.Dl
		t.Hist.Dl = trim(append(t.Hist.Dl, t.Dl))
		t.Hist.Ul = trim(append(t.Hist.Ul, t.Ul))
		var ws float64
		for _, p := range t.Peers {
			ws += p.W
		}
		for j := range t.Peers {
			p := &t.Peers[j]
			p.W = math.Max(.05, p.W+(s.rnd()-.5)*.25)
			p.Dl, p.Ul = 0, 0
			if p.Prog > 0 && ws > 0 {
				p.Dl = t.Dl * p.W / ws
			}
			if p.Prog < 1 && ws > 0 {
				p.Ul = t.Ul * p.W / ws
			}
			if p.Prog < 1 {
				p.Prog = math.Min(1, p.Prog+s.rnd()*.002)
			}
		}
	}

	s.dht = max(300, s.dht+int(math.Round((s.rnd()-.45)*6)))
	return td, tu
}

func trim(a []float64) []float64 { return trimTo(a, 90) }

func trimTo(a []float64, n int) []float64 {
	if len(a) > n {
		return a[len(a)-n:]
	}
	return a
}

const (
	DefaultChartWindow = 90
	chartPoints        = 90
)

type chartAcc struct {
	dl, ul float64
	n      int
}

func chartWindow(v int) int {
	if v < 30 || v > 3600 {
		return DefaultChartWindow
	}
	return v
}

func chartStep(window int) int { return max(1, (window+chartPoints-1)/chartPoints) }

func chartSlots(window int) int {
	step := chartStep(window)
	return (window + step - 1) / step
}

func (s *Service) pushHist(td, tu float64, window int) {
	if window != s.chartW {
		s.chartW, s.hist, s.acc = window, History{}, chartAcc{}
	}
	s.acc.dl += td
	s.acc.ul += tu
	s.acc.n++
	if s.acc.n < chartStep(window) {
		return
	}
	n := float64(s.acc.n)
	slots := chartSlots(window)
	s.hist.Dl = trimTo(append(s.hist.Dl, s.acc.dl/n), slots)
	s.hist.Ul = trimTo(append(s.hist.Ul, s.acc.ul/n), slots)
	s.acc = chartAcc{}
}

func (s *Service) chartHist(window int) History {
	h := History{Dl: []float64{}, Ul: []float64{}}
	if s.chartW != window {
		return h
	}
	h.Dl = append(h.Dl, s.hist.Dl...)
	h.Ul = append(h.Ul, s.hist.Ul...)
	if s.acc.n > 0 {
		n := float64(s.acc.n)
		slots := chartSlots(window)
		h.Dl = trimTo(append(h.Dl, s.acc.dl/n), slots)
		h.Ul = trimTo(append(h.Ul, s.acc.ul/n), slots)
	}
	return h
}

func anyRatioLimit(ts []Torrent) bool {
	for _, t := range ts {
		if t.RatioLimit != nil {
			return true
		}
	}
	return false
}

func (s *Service) applySeedLimit(t *Torrent, set settings.Settings, now int64) int64 {
	if t.Status != StatusSeeding {
		return 0
	}
	var rl *float64
	switch {
	case t.RatioLimit != nil && *t.RatioLimit >= 0:
		rl = t.RatioLimit
	case t.RatioLimit == nil && set.RatioOn:
		v := set.Ratio
		rl = &v
	}
	hitR := rl != nil && t.RatioValue() >= *rl
	hitT := set.SeedTimeOn && t.SeedSince > 0 && now-t.SeedSince >= int64(set.SeedTime)*60e3
	if !hitR && !hitT {
		return 0
	}
	reason, why := "seeding_time_limit", "reached its seeding time limit"
	if hitR {
		reason, why = "ratio_limit", "reached ratio "+strconv.FormatFloat(*rl, 'f', -1, 64)
	}
	switch set.SeedAction {
	case "pause":
		t.Status, t.Ul = StatusDone, 0
		s.bus.Emit("torrent.paused", map[string]any{"torrent": Info(*t), "reason": reason})
	case "superseed":
		if !t.Superseed {
			t.Superseed = true
			s.log.Add("info", "Torrents", t.Name+" "+why+", so super seeding is on")
		}
	default:
		s.bus.Emit("torrent.removed", map[string]any{"torrent": Info(*t), "deleted_files": set.SeedAction == "removeData", "reason": reason})
		if set.SeedAction == "removeData" {
			s.free += t.Done
		}
		return t.ID
	}
	return 0
}

type Stats struct {
	Free     float64 `json:"free"`
	Dht      int     `json:"dht"`
	SessDl   float64 `json:"sessDl"`
	SessUl   float64 `json:"sessUl"`
	Alt      bool    `json:"alt"`
	AltOn    bool    `json:"altActive"`
	Hist     History `json:"hist"`
	HistWin  int     `json:"histWindow"`
	HistStep int     `json:"histStep"`
	Uptime   int64   `json:"uptime"`
	Torrents int     `json:"torrents"`
	Active   int     `json:"active"`
	Conns    int     `json:"conns"`
	Real     bool    `json:"real"`
	Engine   string  `json:"engine"`
	Port     int     `json:"port"`
	TotalDl  float64 `json:"totalDl"`
	TotalUl  float64 `json:"totalUl"`
}

func (s *Service) Stats(ctx context.Context) (Stats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.repo.List(ctx)
	if err != nil {
		return Stats{}, err
	}
	set := s.set.Get()
	st := Stats{Free: s.free, Dht: s.dht, SessDl: s.sessDl, SessUl: s.sessUl, Alt: s.alt, AltOn: AltActive(set, s.alt, s.now()),
		Hist: s.chartHist(chartWindow(set.SpeedWindow)), HistWin: chartWindow(set.SpeedWindow), HistStep: chartStep(chartWindow(set.SpeedWindow)),
		Uptime: int64(time.Since(s.started).Seconds()), Torrents: len(ts)}
	st.Engine = s.EngineName()
	for _, t := range ts {
		st.TotalDl += t.Dl
		st.TotalUl += t.Ul
		if t.Status == StatusDownloading || t.Status == StatusSeeding {
			st.Active++
			if s.eng == nil {
				st.Conns += 20
			}
		}
	}
	if s.eng != nil {
		tot := s.eng.Totals()
		st.Conns, st.Dht = tot.Conns, tot.DHTNodes
		st.Real, st.Port = true, s.eng.Port()
		if s.freeFn != nil {
			st.Free = s.freeFn()
		}
	}
	if set.MaxConn > 0 && st.Conns > set.MaxConn {
		st.Conns = set.MaxConn
	}
	return st, nil
}

func (s *Service) SetAlt(on bool) {
	s.mu.Lock()
	s.alt = on
	s.mu.Unlock()
}

func (s *Service) Snapshot(ctx context.Context) ([]Torrent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.List(ctx)
}

func (s *Service) Mutate(ctx context.Context, fn func([]Torrent)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ts, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	old := map[int64][]string{}
	if s.eng != nil {
		for _, t := range ts {
			old[t.ID] = nameList(t)
		}
	}
	fn(ts)
	for i := range ts {
		if o, ok := old[ts[i].ID]; ok {
			s.applyNames(&ts[i], o)
		}
	}
	return s.repo.PutAll(ctx, ts)
}

func (s *Service) Free() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.eng != nil && s.freeFn != nil {
		return s.freeFn()
	}
	return s.free
}

func (s *Service) RemoveWithReason(ctx context.Context, ids []int64, deleteData bool, reason string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sel, _, err := s.selection(ctx, ids)
	if err != nil {
		return 0, err
	}
	for _, t := range sel {
		s.bus.Emit("torrent.removed", map[string]any{"torrent": Info(t), "deleted_files": deleteData, "reason": reason})
		if deleteData {
			s.free += t.Done
		}
		if err := s.forget(t, deleteData); err != nil {
			return 0, err
		}
		if err := s.repo.Delete(ctx, t.ID); err != nil {
			return 0, err
		}
	}
	return len(sel), nil
}

func (s *Service) Emit(event string, data map[string]any) {
	s.bus.Emit(event, data)
}
