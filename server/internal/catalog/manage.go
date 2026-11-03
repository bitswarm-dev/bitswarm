package catalog

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type Invalid string

func (e Invalid) Error() string { return string(e) }

type Use struct {
	Kind  string   `json:"kind"`
	Count int      `json:"count"`
	Names []string `json:"names"`
}

type Owner interface {
	TagUse(tag string) Use
	RenameTag(from, to string) int
	RemoveTag(tag string) int
}

func (s *Service) AddOwner(o Owner) { s.owners = append(s.owners, o) }

func (s *Service) SetLog(fn func(level, src, msg string)) { s.logf = fn }

func (s *Service) note(msg string) {
	if s.logf != nil {
		s.logf("info", "Tags", msg)
	}
}

func oneTag(raw string) (string, error) {
	if strings.Contains(raw, ",") {
		return "", Invalid("Enter one tag name, without commas.")
	}
	tags := ParseTags(raw)
	if len(tags) != 1 {
		return "", Invalid("Enter a tag name.")
	}
	return tags[0], nil
}

func (s *Service) Usage() map[string][]Use {
	out := map[string][]Use{}
	for _, t := range s.TagNames() {
		uses := []Use{}
		for _, o := range s.owners {
			if u := o.TagUse(t); u.Count > 0 {
				if len(u.Names) > 5 {
					u.Names = u.Names[:5]
				}
				uses = append(uses, u)
			}
		}
		out[t] = uses
	}
	return out
}

func (s *Service) Add(_ context.Context, raw string) (string, error) {
	name, err := oneTag(raw)
	if err != nil {
		return "", err
	}
	if slices.Contains(s.TagNames(), name) {
		return "", Invalid("The tag “" + name + "” already exists.")
	}
	s.EnsureTags([]string{name})
	s.note("Added the tag “" + name + "”")
	return name, nil
}

func (s *Service) Rename(_ context.Context, from, raw string) (string, error) {
	to, err := oneTag(raw)
	if err != nil {
		return "", err
	}
	if !slices.Contains(s.TagNames(), from) {
		return "", Invalid("The tag “" + from + "” doesn’t exist.")
	}
	if to == from {
		return to, nil
	}
	merged := slices.Contains(s.TagNames(), to)
	changed := 0
	for _, o := range s.owners {
		changed += o.RenameTag(from, to)
	}
	s.mu.Lock()
	if merged {
		s.snap.Tags = slices.DeleteFunc(s.snap.Tags, func(t string) bool { return t == from })
	} else {
		for i, t := range s.snap.Tags {
			if t == from {
				s.snap.Tags[i] = to
			}
		}
	}
	err = s.save()
	s.mu.Unlock()
	verb := "Renamed"
	if merged {
		verb = "Merged"
	}
	s.note(fmt.Sprintf("%s the tag “%s” into “%s” (%d places updated)", verb, from, to, changed))
	return to, err
}

func (s *Service) Delete(_ context.Context, name string) error {
	if !slices.Contains(s.TagNames(), name) {
		return Invalid("The tag “" + name + "” doesn’t exist.")
	}
	changed := 0
	for _, o := range s.owners {
		changed += o.RemoveTag(name)
	}
	s.RemoveTags([]string{name})
	s.note(fmt.Sprintf("Deleted the tag “%s” (removed from %d places)", name, changed))
	return nil
}
