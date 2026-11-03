package torrents

import (
	"fmt"
	"math"
	"strings"
)

type rng struct{ s int64 }

func newRng(seed int64) *rng {
	s := seed % 2147483647
	if s == 0 {
		s = 1
	}
	return &rng{s: s}
}

func (r *rng) next() float64 {
	r.s = r.s * 16807 % 2147483647
	return float64(r.s) / 2147483647
}

func (r *rng) hex(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteByte("0123456789abcdef"[int(r.next()*16)])
	}
	return b.String()
}

func FormatBytes(b float64) string {
	if b < KiB {
		return fmt.Sprintf("%d B", int(math.Round(b)))
	}
	units := []string{"KB", "MB", "GB", "TB"}
	i := -1
	for {
		b /= 1024
		i++
		if !(b >= 1000 && i < 3) {
			break
		}
	}
	switch {
	case b >= 100:
		return fmt.Sprintf("%.0f %s", b, units[i])
	case b >= 10:
		return fmt.Sprintf("%.1f %s", b, units[i])
	}
	return fmt.Sprintf("%.2f %s", b, units[i])
}

func Plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func walk(cur, target, spread float64, rnd func() float64) float64 {
	next := cur + (target-cur)*.35 + (rnd()-.5)*target*spread
	return math.Max(0, next)
}

func Info(t Torrent) map[string]any {
	tags := append([]string{}, t.Tags...)
	return map[string]any{
		"id": t.ID, "hash": t.Hash, "name": t.Name, "tags": tags,
		"size": t.Size, "size_text": FormatBytes(t.Size), "progress": math.Round(t.Pct()*10000) / 10000,
		"ratio": math.Round(t.RatioValue()*1000) / 1000, "state": string(t.Status), "save_path": t.Path,
	}
}
