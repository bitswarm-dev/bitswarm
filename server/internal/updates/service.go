package updates

import (
	"bitswarm/server/internal/buildinfo"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"bitswarm/server/internal/events"
)

type Release struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Notes   []string `json:"notes"`
}

type State struct {
	State      string   `json:"state"`
	Current    string   `json:"current"`
	Latest     *Release `json:"latest"`
	Checked    int64    `json:"checked"`
	Configured bool     `json:"configured"`
	Sample     bool     `json:"sample"`
	Error      string   `json:"error"`
}

type Feed func(ctx context.Context) ([]Release, error)

type Service struct {
	current string
	feed    Feed
	sample  bool
	bus     events.Emitter
	now     func() time.Time
	mu      sync.Mutex
	st      State
	told    string
}

func NewService(current string, feed Feed, sample bool, bus events.Emitter) *Service {
	return &Service{current: current, feed: feed, sample: sample, bus: bus, now: time.Now,
		st: State{State: "idle", Current: current, Configured: feed != nil, Sample: sample}}
}

func SampleFeed() Feed {
	return func(context.Context) ([]Release, error) {
		return []Release{
			{Version: "1.0.0", Date: "2026-10-28T12:00:00Z", Notes: []string{
				"Connects to a real qBittorrent or Transmission backend",
				"Scheduled downloads run on the server, so they keep going with the page closed",
				"Several tags per torrent",
				"Faster torrent list with thousands of torrents",
				"Fixes the search box shrinking when many torrents are selected",
			}},
			{Version: "0.9.0", Date: "2026-09-14T12:00:00Z", Notes: []string{}},
		}, nil
	}
}

func URLFeed(raw string) Feed {
	client := &http.Client{Timeout: 15 * time.Second}
	return func(ctx context.Context) ([]Release, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Bitswarm/"+buildinfo.Version)
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("couldn’t reach the release feed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return nil, fmt.Errorf("the release feed answered with status %d", resp.StatusCode)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return nil, err
		}
		var rs []Release
		if err := json.Unmarshal(body, &rs); err != nil {
			return nil, errors.New("the release feed isn’t a list of releases")
		}
		return rs, nil
	}
}

func Compare(a, b string) int {
	x, y := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var p, q int
		if i < len(x) {
			p, _ = strconv.Atoi(x[i])
		}
		if i < len(y) {
			q, _ = strconv.Atoi(y[i])
		}
		if p != q {
			return p - q
		}
	}
	return 0
}

func (s *Service) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.st
}

func (s *Service) Check(ctx context.Context) State {
	s.mu.Lock()
	if s.feed == nil {
		s.st.State, s.st.Error = "unconfigured", ""
		out := s.st
		s.mu.Unlock()
		return out
	}
	s.st.State = "checking"
	s.mu.Unlock()

	rs, err := s.feed(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.st.Checked = s.now().UnixMilli()
	if err != nil || len(rs) == 0 {
		s.st.State, s.st.Error = "error", "No release information came back."
		if err != nil {
			s.st.Error = err.Error()
		}
		return s.st
	}
	latest := rs[0]
	for _, r := range rs[1:] {
		if Compare(r.Version, latest.Version) > 0 {
			latest = r
		}
	}
	if latest.Notes == nil {
		latest.Notes = []string{}
	}
	s.st.Latest, s.st.Error = &latest, ""
	if Compare(latest.Version, s.current) > 0 {
		s.st.State = "available"
		if s.told != latest.Version {
			s.told = latest.Version
			s.bus.Emit("update.available", map[string]any{"current": s.current, "latest": latest.Version, "released": latest.Date})
		}
	} else {
		s.st.State = "current"
	}
	return s.st
}
