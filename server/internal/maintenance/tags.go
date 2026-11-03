package maintenance

import (
	"context"

	"bitswarm/server/internal/catalog"
)

func (s *Service) TagUse(tag string) catalog.Use {
	u := catalog.Use{Kind: "maintenance tasks"}
	ts, err := s.tasks.List(context.Background())
	if err != nil {
		return u
	}
	for _, t := range ts {
		if t.Tag == tag {
			u.Count++
			u.Names = append(u.Names, metas[t.Type].title)
		}
	}
	return u
}

func (s *Service) RenameTag(from, to string) int { return s.editTag(from, to) }

func (s *Service) RemoveTag(tag string) int { return s.editTag(tag, "") }

func (s *Service) editTag(from, to string) int {
	ctx := context.Background()
	ts, err := s.tasks.List(ctx)
	if err != nil {
		return 0
	}
	n := 0
	for _, t := range ts {
		if t.Tag == from {
			t.Tag = to
			if s.tasks.Put(ctx, t) == nil {
				n++
			}
		}
	}
	return n
}
