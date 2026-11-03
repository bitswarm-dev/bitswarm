package renames

import (
	"slices"
	"testing"

	"bitswarm/server/internal/torrents"
)

func TestExpand(t *testing.T) {
	re := mustCompile(t, Preset{Find: `^(\w+)-(\d+)$`})
	src := "ubuntu-26"
	loc := re.FindStringSubmatchIndex(src)
	tests := []struct{ in, want string }{
		{"$1 v$2", "ubuntu v26"},
		{"$&!", "ubuntu-26!"},
		{"$$1", "$1"},
		{"[$`|$']", "[|]"},
		{"$3", "$3"},
		{"$0", "$0"},
		{"end$", "end$"},
		{"$x", "$x"},
		{"$1$2", "ubuntu26"},
	}
	for _, tc := range tests {
		if got := expand(tc.in, src, loc); got != tc.want {
			t.Errorf("expand(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func mustCompile(t *testing.T, p Preset) interface {
	FindStringSubmatchIndex(string) []int
	MatchString(string) bool
	FindAllStringSubmatchIndex(string, int) [][]int
} {
	t.Helper()
	re, err := Compile(p)
	if err != nil {
		t.Fatal(err)
	}
	return re
}

func TestRenameOne(t *testing.T) {
	tests := []struct {
		name   string
		p      Preset
		in     string
		dirs   bool
		want   string
		wantOK bool
	}{
		{"keeps extension", Preset{Find: `^ubuntu-(\d+)-(\w+)$`, Replace: "Ubuntu $1 ($2)", I: true, KeepExt: true}, "ubuntu-26-amd64.iso", false, "Ubuntu 26 (amd64).iso", true},
		{"no match", Preset{Find: `^zzz$`, Replace: "x", KeepExt: true}, "ubuntu.iso", false, "", false},
		{"first match only", Preset{Find: `a`, Replace: "b"}, "banana", false, "bbnana", true},
		{"every match", Preset{Find: `a`, Replace: "b", G: true}, "banana", false, "bbnbnb", true},
		{"ignore case", Preset{Find: `PRO`, Replace: "x", I: true}, "pro.txt", false, "x.txt", true},
		{"directory preserved", Preset{Find: `^a$`, Replace: "b", KeepExt: true}, "dir/a.txt", false, "dir/b.txt", true},
		{"unsafe characters", Preset{Find: `^a$`, Replace: "x:y?z"}, "a", false, "x-y-z", true},
		{"collapses spaces", Preset{Find: `-`, Replace: "  ", G: true}, "a-b", false, "a b", true},
		{"deleting everything", Preset{Find: `.+`, Replace: ""}, "gone", false, "", true},
		{"unchanged", Preset{Find: `a`, Replace: "a"}, "a", false, "", false},
		{"folder in replacement", Preset{Find: `^\[.+?\] (.+?) - (\d+) .*$`, Replace: "$1/$1 - $2", KeepExt: true}, "[G] Show Name - 06 (1080p) [AB12].mkv", true, "Show Name/Show Name - 06.mkv", true},
		{"folder kept with dir prefix", Preset{Find: `^a$`, Replace: "x/y"}, "d/a", true, "d/x/y", true},
		{"slash flattened without dirs", Preset{Find: `^a$`, Replace: "x/y"}, "a", false, "x-y", true},
		{"dot segments dropped", Preset{Find: `^a$`, Replace: "../../etc/./x"}, "a", true, "etc/x", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			re, err := Compile(tc.p)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := renameOne(tc.p, tc.in, re, tc.dirs)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("renameOne() = %q, %v; want %q, %v", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestBump(t *testing.T) {
	taken := map[string]bool{"a (2).txt": true}
	if got := bump("a.txt", taken); got != "a (3).txt" {
		t.Errorf("bump() = %q", got)
	}
	if got := bump("noext", map[string]bool{}); got != "noext (2)" {
		t.Errorf("bump(noext) = %q", got)
	}
}

func TestPlan(t *testing.T) {
	multi := torrents.Torrent{Name: "Show", Files: []torrents.File{{Name: "Show.2010.mkv"}, {Name: "Show.2011.mkv"}, {Name: "Show (2010).mkv"}}}
	single := torrents.Torrent{Name: "a.iso", Files: []torrents.File{{Name: "a.iso"}}}

	files := plan(Preset{Find: `^Show\.(\d{4})$`, Replace: "Show ($1)", Target: "files", KeepExt: true}, multi)
	if len(files) != 2 || files[0].To != "Show (2010) (2).mkv" || !files[0].Clash || files[1].To != "Show (2011).mkv" {
		t.Errorf("clash handling wrong: %+v", files)
	}

	both := plan(Preset{Find: `^a$`, Replace: "b", Target: "both", KeepExt: true}, single)
	if len(both) != 1 || !both[0].AlsoTorrent || both[0].To != "b.iso" {
		t.Errorf("single-file torrent should rename once: %+v", both)
	}
	if got := plan(Preset{Find: `^a$`, Replace: "b", Target: "torrent", KeepExt: true}, single); len(got) != 1 || got[0].File != -1 {
		t.Errorf("torrent target = %+v", got)
	}
	empty := plan(Preset{Find: `.+`, Replace: "", Target: "files"}, single)
	if len(empty) != 1 || !empty[0].Empty {
		t.Errorf("empty result should be flagged: %+v", empty)
	}
	if plan(Preset{}, single) != nil || plan(Preset{Find: "("}, single) != nil {
		t.Error("blank or invalid pattern should plan nothing")
	}
	if !slices.Equal([]string{"x"}, []string{"x"}) {
		t.Fatal("unreachable")
	}
}
