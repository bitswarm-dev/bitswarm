package torrents

import (
	"context"
	"slices"

	"bitswarm/server/internal/catalog"
)

func (s *Service) TagUse(tag string) catalog.Use {
	u := catalog.Use{Kind: "torrents"}
	ts, err := s.Snapshot(context.Background())
	if err != nil {
		return u
	}
	for _, t := range ts {
		if slices.Contains(t.Tags, tag) {
			u.Count++
			u.Names = append(u.Names, t.Name)
		}
	}
	return u
}

func (s *Service) RenameTag(from, to string) int { return s.editTag(from, to) }

func (s *Service) RemoveTag(tag string) int { return s.editTag(tag, "") }

func (s *Service) editTag(from, to string) int {
	n := 0
	_ = s.Mutate(context.Background(), func(ts []Torrent) {
		for i := range ts {
			if !slices.Contains(ts[i].Tags, from) {
				continue
			}
			next := []string{}
			for _, t := range ts[i].Tags {
				if t == from {
					t = to
				}
				if t != "" && !slices.Contains(next, t) {
					next = append(next, t)
				}
			}
			ts[i].Tags = next
			n++
		}
	})
	return n
}
