package renames

import (
	"context"

	"bitswarm/server/internal/catalog"
)

func (s *Service) TagUse(tag string) catalog.Use {
	u := catalog.Use{Kind: "rename presets"}
	ps, err := s.presets.List(context.Background())
	if err != nil {
		return u
	}
	for _, p := range ps {
		if p.Tag == tag {
			u.Count++
			u.Names = append(u.Names, p.Name)
		}
	}
	return u
}

func (s *Service) RenameTag(from, to string) int { return s.editTag(from, to) }

func (s *Service) RemoveTag(tag string) int { return s.editTag(tag, "") }

func (s *Service) editTag(from, to string) int {
	ctx := context.Background()
	ps, err := s.presets.List(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, p := range ps {
		if p.Tag == from {
			p.Tag = to
			if s.presets.Put(ctx, p) == nil {
				n++
			}
		}
	}
	return n
}
