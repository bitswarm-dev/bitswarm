package schedules

import (
	"context"
	"slices"

	"bitswarm/server/internal/catalog"
)

func (s *Service) TagUse(tag string) catalog.Use {
	u := catalog.Use{Kind: "schedules"}
	rs, err := s.rules.List(context.Background())
	if err != nil {
		return u
	}
	for _, r := range rs {
		if slices.Contains(r.Tags, tag) {
			u.Count++
			u.Names = append(u.Names, r.Name)
		}
	}
	return u
}

func swapTag(tags []string, from, to string) []string {
	out := []string{}
	for _, t := range tags {
		if t == from {
			t = to
		}
		if t != "" && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func (s *Service) RenameTag(from, to string) int {
	return s.editTags(from, to)
}

func (s *Service) RemoveTag(tag string) int {
	return s.editTags(tag, "")
}

func (s *Service) editTags(from, to string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	rs, err := s.rules.List(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, r := range rs {
		if slices.Contains(r.Tags, from) {
			r.Tags = swapTag(r.Tags, from, to)
			if s.rules.Put(ctx, r) == nil {
				n++
			}
		}
	}
	return n
}
