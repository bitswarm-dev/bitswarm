package schedules

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

var (
	magnetRe  = regexp.MustCompile(`(?i)^magnet:\?`)
	torrentRe = regexp.MustCompile(`(?i)\.torrent([?#]|$)`)
	btihRe    = regexp.MustCompile(`(?i)btih:([0-9a-z]{32,40})`)
	dnRe      = regexp.MustCompile(`(?i)[?&]dn=([^&]+)`)
	spaceRe   = regexp.MustCompile(`\s+`)
)

type Known interface {
	Has(btih, name string) bool
}

func words(s string) []string {
	var out []string
	for _, w := range strings.Split(s, ",") {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			out = append(out, w)
		}
	}
	return out
}

func termsOf(r Rule) []string {
	var out []string
	for _, l := range strings.Split(r.Terms, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func searchURLs(r Rule) []string {
	if r.URL == "" {
		return nil
	}
	if !strings.Contains(r.URL, "{query}") {
		return []string{r.URL}
	}
	var out []string
	for _, q := range termsOf(r) {
		out = append(out, strings.ReplaceAll(r.URL, "{query}", strings.ReplaceAll(url.QueryEscape(q), "%20", "+")))
	}
	return out
}

func baseURL(r Rule) string {
	if u := searchURLs(r); len(u) > 0 {
		return u[0]
	}
	return r.URL
}

func nameFor(r Rule) string {
	if t := termsOf(r); len(t) > 0 {
		return t[0]
	}
	u, err := url.Parse(r.URL)
	if err != nil || u.Host == "" {
		return ""
	}
	q := u.Query()
	for _, k := range []string{"q", "query", "search", "s", "keywords", "term", "k", "find"} {
		if v := strings.Join(strings.Fields(q.Get(k)), " "); v != "" {
			return v
		}
	}
	return u.Host
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func CheckSelector(sel string) error {
	_, err := cascadia.Compile(sel)
	return err
}

func Extract(r Rule, page string, known Known) Extraction {
	out := Extraction{Items: []Item{}}
	sel, err := cascadia.Compile(r.Selector)
	if err != nil {
		out.Error = "“" + r.Selector + "” isn’t a valid CSS selector."
		return out
	}
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		out.Error = "The results page couldn’t be read as HTML."
		return out
	}
	base, _ := url.Parse(baseURL(r))
	nodes := cascadia.QueryAll(doc, sel)
	out.Matched = len(nodes)

	inc, exc := words(r.Include), words(r.Exclude)
	seenKeys := map[string]bool{}
	seen := map[string]bool{}
	for _, k := range r.Seen {
		seen[k] = true
	}
	take := 0

	for _, el := range nodes {
		var raw string
		if r.Attr == "text" {
			raw = text(el)
		} else {
			raw = attr(el, r.Attr)
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		isMag := magnetRe.MatchString(raw)
		link := raw
		if !isMag {
			u, err := url.Parse(raw)
			if err != nil || base == nil {
				continue
			}
			link = base.ResolveReference(u).String()
		}
		kind := ""
		switch {
		case isMag:
			kind = "magnet"
		case torrentRe.MatchString(link):
			kind = "torrent"
		}
		if kind == "" || (r.LinkType != "any" && r.LinkType != kind) {
			continue
		}
		btih := ""
		if isMag {
			if m := btihRe.FindStringSubmatch(raw); m != nil {
				btih = strings.ToLower(m[1])
			}
		}
		key := link
		if btih != "" {
			key = btih
		}
		if seenKeys[key] {
			continue
		}
		seenKeys[key] = true

		name := ""
		if isMag {
			if m := dnRe.FindStringSubmatch(raw); m != nil {
				if d, err := url.QueryUnescape(strings.ReplaceAll(m[1], "+", " ")); err == nil {
					name = d
				}
			}
		}
		if name == "" {
			name = strings.TrimSpace(attr(el, "title"))
		}
		if name == "" {
			if t := strings.TrimSpace(text(el)); len(t) > 8 {
				name = t
			} else {
				last := link[strings.LastIndex(link, "/")+1:]
				last = regexp.MustCompile(`(?i)\.torrent.*$`).ReplaceAllString(last, "")
				if d, err := url.QueryUnescape(last); err == nil {
					last = d
				}
				name = last
			}
		}
		name = spaceRe.ReplaceAllString(name, " ")
		if len(name) > 160 {
			name = name[:160]
		}

		low := strings.ToLower(name)
		state := ""
		switch {
		case !allIn(low, inc) || anyIn(low, exc):
			state = "filtered"
		case r.SkipSeen && (seen[key] || (known != nil && known.Has(btih, name))):
			state = "have"
		case take >= r.Max:
			state = "over"
		default:
			state = "new"
			take++
		}
		out.Items = append(out.Items, Item{Name: name, Type: kind, URL: link, Key: key, Btih: btih, State: state})
	}
	return out
}

func allIn(s string, ws []string) bool {
	for _, w := range ws {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

func anyIn(s string, ws []string) bool {
	for _, w := range ws {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}
