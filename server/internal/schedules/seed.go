package schedules

import (
	"context"
	"strings"
	"time"
)

func (s *Service) Seed(ctx context.Context, ubuntuPreset int64, base string) error {
	base = strings.TrimRight(base, "/")
	items, err := s.rules.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	now := s.nowMs()
	end := time.UnixMilli(now + 30*day)
	endOfDay := time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999e6, end.Location()).UnixMilli()
	for _, r := range []Rule{
		{Name: "Ubuntu point releases", URL: "https://linuxtracker.org/index.php?page=torrents&search=ubuntu+26.04",
			RenamePreset: ubuntuPreset, Selector: "a[href^='magnet:']", Attr: "href", LinkType: "magnet", Include: "ubuntu", Exclude: "server, kubuntu", Tags: []string{"keep"}, Path: base + "/linux-isos",
			Max: 5, Start: true, SkipSeen: true, Every: 6, Unit: "hours"},
		{Name: "Prelinger Archives films", URL: "https://archive.org/search?query=prelinger+civil+defense&and[]=mediatype%3A%22movies%22",
			Selector: "a.download-torrent", Attr: "href", LinkType: "torrent", Tags: []string{"large"}, Path: base + "/archives", Max: 3, SkipSeen: true, Every: 1, Unit: "days", EndDate: endOfDay},
		{Name: "Vision datasets", URL: "https://academictorrents.com/browse.php?search=imagenet",
			Selector: "td a[href*='/download/']", Attr: "href", LinkType: "torrent", Tags: []string{"large"}, Path: base + "/datasets", Max: 2, Start: true, SkipSeen: true, Every: 7, Unit: "days"},
	} {
		r.Status, r.Seen = "idle", []string{}
		r.NextRun = now + intervalMs(r)
		if _, err := s.rules.Create(ctx, r); err != nil {
			return err
		}
	}
	return nil
}
