package hooks

import (
	"context"

	"bitswarm/server/internal/catalog"
)

func (s *Service) TagUse(tag string) catalog.Use {
	u := catalog.Use{Kind: "hooks"}
	hs, err := s.hooks.List(context.Background())
	if err != nil {
		return u
	}
	for _, h := range hs {
		if h.Tag == tag {
			u.Count++
			u.Names = append(u.Names, h.Name)
		}
	}
	return u
}

func (s *Service) RenameTag(from, to string) int { return s.editTag(from, to) }

func (s *Service) RemoveTag(tag string) int { return s.editTag(tag, "") }

func (s *Service) editTag(from, to string) int {
	ctx := context.Background()
	hs, err := s.hooks.List(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, h := range hs {
		if h.Tag == from {
			h.Tag = to
			if s.hooks.Put(ctx, h) == nil {
				n++
			}
		}
	}
	return n
}
