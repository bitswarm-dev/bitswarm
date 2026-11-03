package renames

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"bitswarm/server/internal/torrents"
)

var (
	extRe     = regexp.MustCompile(`^(.+?)(\.[A-Za-z0-9]{1,5})$`)
	bumpRe    = regexp.MustCompile(`^(.*?)(\.[A-Za-z0-9]{1,5})?$`)
	badChars  = regexp.MustCompile(`[\\/:*?"<>|]`)
	spaceRuns = regexp.MustCompile(`\s{2,}`)
)

func Compile(p Preset) (*regexp.Regexp, error) {
	pattern := p.Find
	if p.I {
		pattern = "(?i)" + pattern
	}
	return regexp.Compile(pattern)
}

func expand(repl, src string, loc []int) string {
	var b strings.Builder
	for i := 0; i < len(repl); i++ {
		c := repl[i]
		if c != '$' || i+1 >= len(repl) {
			b.WriteByte(c)
			continue
		}
		n := repl[i+1]
		switch {
		case n == '$':
			b.WriteByte('$')
			i++
		case n == '&':
			b.WriteString(src[loc[0]:loc[1]])
			i++
		case n == '`':
			b.WriteString(src[:loc[0]])
			i++
		case n == '\'':
			b.WriteString(src[loc[1]:])
			i++
		case n >= '0' && n <= '9':
			end := i + 2
			if end < len(repl) && repl[end] >= '0' && repl[end] <= '9' && (int(n-'0')*10+int(repl[end]-'0')) <= len(loc)/2-1 {
				end++
			}
			idx, _ := strconv.Atoi(repl[i+1 : end])
			if idx >= 1 && idx <= len(loc)/2-1 {
				if loc[2*idx] >= 0 {
					b.WriteString(src[loc[2*idx]:loc[2*idx+1]])
				}
				i = end - 1
			} else {
				b.WriteByte('$')
			}
		default:
			b.WriteByte('$')
		}
	}
	return b.String()
}

func replace(re *regexp.Regexp, src, repl string, all bool) string {
	var b strings.Builder
	last := 0
	for _, loc := range re.FindAllStringSubmatchIndex(src, -1) {
		b.WriteString(src[last:loc[0]])
		b.WriteString(expand(repl, src, loc))
		last = loc[1]
		if !all {
			break
		}
	}
	b.WriteString(src[last:])
	return b.String()
}

func testedPart(p Preset, name string) string {
	base := name[strings.LastIndex(name, "/")+1:]
	if p.KeepExt {
		if m := extRe.FindStringSubmatch(base); m != nil {
			return m[1]
		}
	}
	return base
}

func cleanName(s string) string {
	return strings.TrimSpace(spaceRuns.ReplaceAllString(badChars.ReplaceAllString(s, "-"), " "))
}

func cleanPath(s string, dirs bool) string {
	if !dirs {
		return cleanName(s)
	}
	var segs []string
	for _, part := range strings.Split(s, "/") {
		if part = cleanName(part); part != "" && part != "." && part != ".." {
			segs = append(segs, part)
		}
	}
	return strings.Join(segs, "/")
}

func renameOne(p Preset, name string, re *regexp.Regexp, dirs bool) (string, bool) {
	slash := strings.LastIndex(name, "/")
	dir, base := name[:slash+1], name[slash+1:]
	stem, ext := base, ""
	if p.KeepExt {
		if m := extRe.FindStringSubmatch(base); m != nil {
			stem, ext = m[1], m[2]
		}
	}
	if !re.MatchString(stem) {
		return "", false
	}
	out := replace(re, stem, p.Replace, p.G)
	out = cleanPath(out, dirs)
	to := ""
	if out != "" {
		to = dir + out + ext
	}
	if to == name {
		return "", false
	}
	return to, true
}

func bump(name string, taken map[string]bool) string {
	m := bumpRe.FindStringSubmatch(name)
	for i := 2; ; i++ {
		n := fmt.Sprintf("%s (%d)%s", m[1], i, m[2])
		if !taken[strings.ToLower(n)] {
			return n
		}
	}
}

type planItem struct {
	File        int
	From, To    string
	Empty       bool
	Clash       bool
	AlsoTorrent bool
}

func plan(p Preset, t torrents.Torrent) []planItem {
	if p.Find == "" {
		return nil
	}
	re, err := Compile(p)
	if err != nil {
		return nil
	}
	var out []planItem
	single := len(t.Files) == 1 && t.Files[0].Name == t.Name
	if p.Target != "torrent" {
		taken := map[string]bool{}
		for _, f := range t.Files {
			taken[strings.ToLower(f.Name)] = true
		}
		for i, f := range t.Files {
			to, ok := renameOne(p, f.Name, re, true)
			if !ok {
				continue
			}
			if to == "" {
				out = append(out, planItem{File: i, From: f.Name, Empty: true})
				continue
			}
			delete(taken, strings.ToLower(f.Name))
			clash := false
			if taken[strings.ToLower(to)] {
				to, clash = bump(to, taken), true
			}
			taken[strings.ToLower(to)] = true
			out = append(out, planItem{File: i, From: f.Name, To: to, Clash: clash, AlsoTorrent: single})
		}
	}
	if p.Target != "files" && !(single && p.Target == "both" && len(out) > 0) {
		if to, ok := renameOne(p, t.Name, re, false); ok {
			out = append(out, planItem{File: -1, From: t.Name, To: to, Empty: to == ""})
		}
	}
	return out
}
