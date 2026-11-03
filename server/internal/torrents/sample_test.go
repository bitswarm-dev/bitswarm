package torrents

import (
	"slices"
	"strings"
	"testing"
	"time"

	"bitswarm/server/internal/settings"
)

func TestFormatBytes(t *testing.T) {
	tests := map[float64]string{0: "0 B", 512: "512 B", 1024: "1.00 KB", 1536: "1.50 KB", 10 * MiB: "10.0 MB", 150 * MiB: "150 MB", 6.18 * GiB: "6.18 GB", 2000 * GiB: "1.95 TB"}
	for in, want := range tests {
		if got := FormatBytes(in); got != want {
			t.Errorf("FormatBytes(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	if Plural(1, "torrent") != "1 torrent" || Plural(2, "torrent") != "2 torrents" || Plural(0, "tag") != "0 tags" {
		t.Error("Plural() wrong")
	}
}

func TestRng_Deterministic(t *testing.T) {
	a, b := newRng(5), newRng(5)
	for i := 0; i < 5; i++ {
		x := a.next()
		if x != b.next() || x < 0 || x >= 1 {
			t.Fatal("rng is not deterministic or out of range")
		}
	}
	if newRng(0).next() == 0 || len(newRng(1).hex(40)) != 40 {
		t.Error("rng zero seed or hex length wrong")
	}
}

func TestGenContents(t *testing.T) {
	tests := []struct {
		name      string
		wantFiles int
		wantFirst string
	}{
		{"ubuntu.iso", 3, "ubuntu.iso"},
		{"Some Movie 1080p", 5, "Some Movie 1080p.mkv"},
		{"Dataset", 6, "Dataset/part-01.zip"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := GenContents(tc.name)
			if len(c.Files) != tc.wantFiles || c.Files[0].Name != tc.wantFirst || c.Size <= 0 {
				t.Errorf("got %d files, first %q, size %v", len(c.Files), c.Files[0].Name, c.Size)
			}
			if again := GenContents(tc.name); again.Size != c.Size {
				t.Error("contents are not deterministic")
			}
		})
	}
}

func TestLayoutName(t *testing.T) {
	multi := []File{{Name: "root/a.txt"}, {Name: "root/b.txt"}}
	loose := []File{{Name: "a.txt"}, {Name: "b.txt"}}
	tests := []struct {
		name, in, layout string
		files            []File
		want             string
	}{
		{"original", "root/a.txt", "original", multi, "root/a.txt"},
		{"drop shared root", "root/a.txt", "nosub", multi, "a.txt"},
		{"nosub without root", "a.txt", "nosub", loose, "a.txt"},
		{"create subfolder", "a.txt", "subfolder", loose, "My: Show/a.txt"},
		{"subfolder already there", "root/a.txt", "subfolder", multi, "root/a.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := LayoutName(tc.in, "My: Show", tc.layout, tc.files)
			if tc.name == "create subfolder" {
				tc.want = "My- Show/a.txt"
			}
			if got != tc.want {
				t.Errorf("LayoutName() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestNameFromMagnet(t *testing.T) {
	name, hash, err := NameFromMagnet("  " + magnet)
	if err != nil || name != "My Linux ISO.iso" || len(hash) != 40 {
		t.Errorf("got %q %q %v", name, hash, err)
	}
	name, _, _ = NameFromMagnet("magnet:?xt=urn:btih:ABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD")
	if !strings.HasPrefix(name, "Magnet abcdefab") {
		t.Errorf("fallback name = %q", name)
	}
	for _, bad := range []string{"", "http://x", "magnet:?dn=x", "magnet:?xt=urn:btih:short"} {
		if _, _, err := NameFromMagnet(bad); err == nil {
			t.Errorf("NameFromMagnet(%q) should fail", bad)
		}
	}
}

func TestInfoAndModel(t *testing.T) {
	tor := Torrent{ID: 3, Name: "n", Size: 100, Done: 50, Status: StatusDownloading, Tags: []string{"a"}, Uploaded: 25, Dl: 10,
		Files: []File{{Size: 60, Prio: "normal"}, {Size: 40, Prio: "skip"}}}
	if tor.Wanted() != 60 || tor.Pct() != 50.0/60 || tor.RatioValue() != .5 || !tor.Unfinished() || tor.ETA() == nil || *tor.ETA() != 1 {
		t.Errorf("model math wrong: %v %v %v", tor.Wanted(), tor.Pct(), tor.RatioValue())
	}
	info := Info(tor)
	if info["state"] != "downloading" || info["size_text"] != "100 B" {
		t.Errorf("Info() = %v", info)
	}
	tor.Tags = []string{"L"}
	if tags, _ := Info(tor)["tags"].([]string); len(tags) != 1 || tags[0] != "L" {
		t.Error("tags missing from Info()")
	}
	if (Torrent{Size: 9}).Wanted() != 9 || (Torrent{}).Pct() != 0 {
		t.Error("empty-file fallbacks wrong")
	}
	if e := (Torrent{Status: StatusChecking}).ETA(); e != nil {
		t.Error("checking torrents have no ETA")
	}
	if e := (Torrent{Status: StatusPaused}).ETA(); e == nil || *e != 0 {
		t.Error("paused ETA should be 0")
	}
	for st, want := range map[Status][2]bool{StatusPaused: {true, false}, StatusDownloading: {false, true}, StatusError: {true, false}, StatusSeeding: {false, true}, StatusChecking: {false, true}, StatusRechecking: {false, false}} {
		if (Torrent{Status: st}).CanResume() != want[0] || (Torrent{Status: st}).CanPause() != want[1] {
			t.Errorf("%s resume/pause = %v", st, want)
		}
	}
}

func TestAltActive(t *testing.T) {
	set := settings.Defaults()
	set.AltSchedule, set.AltFrom, set.AltTo = true, "08:00", "18:00"
	at := func(day time.Weekday, h, m int) time.Time {
		d := time.Date(2026, 11, 1, h, m, 0, 0, time.UTC)
		for d.Weekday() != day {
			d = d.AddDate(0, 0, 1)
		}
		return d
	}
	tests := []struct {
		name   string
		mutate func(*settings.Settings)
		manual bool
		when   time.Time
		want   bool
	}{
		{"manual wins", func(s *settings.Settings) { s.AltSchedule = false }, true, at(time.Monday, 3, 0), true},
		{"no schedule", func(s *settings.Settings) { s.AltSchedule = false }, false, at(time.Monday, 9, 0), false},
		{"inside window", func(*settings.Settings) {}, false, at(time.Monday, 9, 0), true},
		{"outside window", func(*settings.Settings) {}, false, at(time.Monday, 19, 0), false},
		{"weekday only skips weekend", func(*settings.Settings) {}, false, at(time.Saturday, 9, 0), false},
		{"weekends only skips weekday", func(s *settings.Settings) { s.AltDays = "weekends" }, false, at(time.Monday, 9, 0), false},
		{"every day on weekend", func(s *settings.Settings) { s.AltDays = "every" }, false, at(time.Sunday, 9, 0), true},
		{"overnight late", func(s *settings.Settings) { s.AltDays, s.AltFrom, s.AltTo = "every", "22:00", "06:00" }, false, at(time.Monday, 23, 0), true},
		{"overnight early", func(s *settings.Settings) { s.AltDays, s.AltFrom, s.AltTo = "every", "22:00", "06:00" }, false, at(time.Monday, 5, 0), true},
		{"overnight midday", func(s *settings.Settings) { s.AltDays, s.AltFrom, s.AltTo = "every", "22:00", "06:00" }, false, at(time.Monday, 12, 0), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := set
			tc.mutate(&s)
			if got := AltActive(s, tc.manual, tc.when); got != tc.want {
				t.Errorf("AltActive() = %v, want %v", got, tc.want)
			}
		})
	}
	if !slices.Contains([]string{"a"}, "a") {
		t.Fatal("unreachable")
	}
}
