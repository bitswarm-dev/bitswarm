package catalog

import (
	"context"
	"slices"
	"strings"
	"sync"

	"bitswarm/server/internal/docstore"
)

const kvKey = "catalog"

type Snapshot struct {
	Tags []string `json:"tags"`
}

type Service struct {
	kv   *docstore.KV
	mu   sync.Mutex
	snap Snapshot

	owners []Owner
	logf   func(level, src, msg string)
}

func NewService(kv *docstore.KV) (*Service, error) {
	s := &Service{kv: kv, snap: Snapshot{Tags: []string{"4K", "keep", "large"}}}
	var saved Snapshot
	ok, err := kv.Load(context.Background(), kvKey, &saved)
	if err != nil {
		return nil, err
	}
	if ok {
		s.snap = saved
	}
	if s.snap.Tags == nil {
		s.snap.Tags = []string{}
	}
	return s, s.save()
}

func (s *Service) save() error {
	return s.kv.Save(context.Background(), kvKey, s.snap)
}

func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Snapshot{Tags: slices.Clone(s.snap.Tags)}
}

func (s *Service) EnsureTags(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := false
	for _, n := range names {
		if n != "" && !slices.Contains(s.snap.Tags, n) {
			s.snap.Tags = append(s.snap.Tags, n)
			changed = true
		}
	}
	if changed {
		_ = s.save()
	}
}

func (s *Service) RemoveTags(names []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap.Tags = slices.DeleteFunc(s.snap.Tags, func(t string) bool { return slices.Contains(names, t) })
	_ = s.save()
}

func (s *Service) TagNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.snap.Tags)
}

func (s *Service) Replace(snap Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snap = snap
	if s.snap.Tags == nil {
		s.snap.Tags = []string{}
	}
	return s.save()
}

func ParseTags(s string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		t := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "#"))
		t = strings.Join(strings.Fields(t), " ")
		if len(t) > 40 {
			t = t[:40]
		}
		if t != "" && !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}
