package eventlog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"bitswarm/server/internal/docstore"
)

const keep = 1000

type Entry struct {
	ID    int64  `json:"id"`
	T     int64  `json:"t"`
	Level string `json:"level"`
	Src   string `json:"src"`
	Msg   string `json:"msg"`
}

type Filter struct {
	Levels map[string]bool
	Src    string
	Q      string
	Limit  int
}

type Result struct {
	Entries []Entry        `json:"entries"`
	Counts  map[string]int `json:"counts"`
	Sources []string       `json:"sources"`
	Total   int            `json:"total"`
}

type Service struct {
	col *docstore.Collection[Entry]
	now func() time.Time
}

func NewService(col *docstore.Collection[Entry]) *Service {
	return &Service{col: col, now: time.Now}
}

func NewCollection(db docstoreDB) *docstore.Collection[Entry] {
	return docstore.NewCollection(db, "log", func(e *Entry) *int64 { return &e.ID })
}

func (s *Service) Add(level, src, msg string) {
	ctx := context.Background()
	_, _ = s.col.Create(ctx, Entry{T: s.now().UnixMilli(), Level: level, Src: src, Msg: msg})
	if items, err := s.col.List(ctx); err == nil && len(items) > keep+100 {
		for _, e := range items[:len(items)-keep] {
			_ = s.col.Delete(ctx, e.ID)
		}
	}
}

func (s *Service) Addf(level, src, format string, args ...any) {
	s.Add(level, src, fmt.Sprintf(format, args...))
}

func (s *Service) Emit(event string, data map[string]any) {
	level, msg := Describe(event, data)
	s.Add(level, SourceOf(event), msg)
}

func (s *Service) Query(ctx context.Context, f Filter) (Result, error) {
	items, err := s.col.List(ctx)
	if err != nil {
		return Result{}, err
	}
	if f.Limit <= 0 {
		f.Limit = 300
	}

	res := Result{Entries: []Entry{}, Counts: map[string]int{"error": 0, "warn": 0, "info": 0, "debug": 0}}
	seen := map[string]bool{}
	q := strings.ToLower(f.Q)
	for i := len(items) - 1; i >= 0; i-- {
		e := items[i]
		res.Counts[e.Level]++
		if !seen[e.Src] {
			seen[e.Src] = true
			res.Sources = append(res.Sources, e.Src)
		}
		if len(f.Levels) > 0 && !f.Levels[e.Level] {
			continue
		}
		if f.Src != "" && e.Src != f.Src {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(e.Msg+" "+e.Src), q) {
			continue
		}
		res.Total++
		if len(res.Entries) < f.Limit {
			res.Entries = append(res.Entries, e)
		}
	}
	return res, nil
}

func (s *Service) Clear(ctx context.Context) error {
	return s.col.Clear(ctx)
}

func (s *Service) Count(ctx context.Context) int {
	items, _ := s.col.List(ctx)
	return len(items)
}
