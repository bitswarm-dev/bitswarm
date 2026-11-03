package schedules

import (
	"bitswarm/server/internal/buildinfo"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/torrents"
)

const (
	hour    = int64(3600e3)
	day     = 24 * hour
	logKeep = 40
	seenCap = 2000
)

var ErrNotFound = docstore.ErrNotFound

type Invalid string

func (e Invalid) Error() string { return string(e) }

type TorrentAPI interface {
	Add(ctx context.Context, a torrents.AddSpec) (torrents.View, error)
	Snapshot(ctx context.Context) ([]torrents.Torrent, error)
}

type Fetcher func(ctx context.Context, rawURL string) (string, error)

type Service struct {
	rules *docstore.Collection[Rule]
	log   *docstore.Collection[LogEntry]
	tor   TorrentAPI
	bus   events.Emitter
	fetch Fetcher
	check func(string) error
	now   func() time.Time
	mu    sync.Mutex

	delay     func() time.Duration
	fetchMu   sync.Mutex
	lastFetch time.Time
}

func NewCollections(db docstoreDB) (*docstore.Collection[Rule], *docstore.Collection[LogEntry]) {
	return docstore.NewCollection(db, "schedules", func(r *Rule) *int64 { return &r.ID }),
		docstore.NewCollection(db, "schedlog", func(l *LogEntry) *int64 { return &l.ID })
}

func NewService(rules *docstore.Collection[Rule], log *docstore.Collection[LogEntry], tor TorrentAPI, bus events.Emitter) *Service {
	return &Service{rules: rules, log: log, tor: tor, bus: bus, fetch: HTTPFetcher(), now: time.Now}
}

func HTTPFetcher() Fetcher {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(ctx context.Context, raw string) (string, error) {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return "", fmt.Errorf("%q isn’t an http or https address", raw)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Bitswarm/"+buildinfo.Version)
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("couldn’t reach %s: %v", u.Host, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			return "", fmt.Errorf("%s answered with status %d", u.Host, resp.StatusCode)
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		return string(b), err
	}
}

func (s *Service) SetPathCheck(fn func(string) error) { s.check = fn }

func (s *Service) SetFetcher(f Fetcher) { s.fetch = f }

func (s *Service) SetDelay(fn func() time.Duration) { s.delay = fn }

func (s *Service) fetchPaced(ctx context.Context, u string) (string, error) {
	s.fetchMu.Lock()
	defer s.fetchMu.Unlock()
	if s.delay != nil && !s.lastFetch.IsZero() {
		if wait := s.delay() - time.Since(s.lastFetch); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
	}
	body, err := s.fetch(ctx, u)
	s.lastFetch = time.Now()
	return body, err
}

func intervalMs(r Rule) int64 {
	n := int64(max(1, r.Every))
	if r.Unit == "days" {
		return n * day
	}
	return n * hour
}

func (s *Service) nowMs() int64 { return s.now().UnixMilli() }

type known struct{ ts []torrents.Torrent }

func (k known) Has(btih, name string) bool {
	for _, t := range k.ts {
		if (btih != "" && t.Hash == btih) || t.Name == name {
			return true
		}
	}
	return false
}

func (s *Service) view(r Rule) View {
	v := View{Rule: r, Urls: searchURLs(r), SeenCount: len(r.Seen)}
	v.Seen = nil
	if r.EndDate > 0 && !r.Ended && r.Enabled {
		v.RunsLeft = int(max(0, (r.EndDate-r.NextRun)/intervalMs(r)+1))
	}
	if v.Urls == nil {
		v.Urls = []string{}
	}
	return v
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	rs, err := s.rules.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(rs))
	for i, r := range rs {
		out[i] = s.view(r)
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id int64) (View, error) {
	r, err := s.rules.Get(ctx, id)
	return s.view(r), err
}

func (s *Service) validate(r Rule) error {
	httpURL := func(raw string) bool {
		u, err := url.Parse(raw)
		return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
	}
	switch {
	case !httpURL(strings.ReplaceAll(r.URL, "{query}", "x")):
		return Invalid("Enter the full search URL, starting with https://")
	case strings.Contains(r.URL, "{query}") && len(termsOf(r)) == 0:
		return Invalid("Add at least one search term, or remove {query} from the search URL.")
	case strings.TrimSpace(r.Selector) == "":
		return Invalid("Enter a CSS selector for the links on the results page.")
	case !slices.Contains([]string{"href", "data-magnet", "data-href", "data-url", "text"}, r.Attr):
		return Invalid("Choose where to read the link from.")
	case !slices.Contains([]string{"any", "magnet", "torrent"}, r.LinkType):
		return Invalid("Choose which kind of link to look for.")
	case !slices.Contains([]string{"hours", "days"}, r.Unit) || r.Every < 1 || r.Every > 999:
		return Invalid("Run every 1 to 999 hours or days.")
	case r.Max < 1 || r.Max > 50:
		return Invalid("Max new per run must be from 1 to 50.")
	case r.EndDate > 0 && r.EndDate < s.nowMs():
		return Invalid("The end date is in the past. Pick today or a later day.")
	}
	if err := CheckSelector(r.Selector); err != nil {
		return Invalid("“" + r.Selector + "” isn’t a valid CSS selector. Check for unclosed brackets or quotes.")
	}
	if s.check != nil && strings.TrimSpace(r.Path) != "" {
		if err := s.check(r.Path); err != nil {
			return Invalid(err.Error())
		}
	}
	return nil
}

func normalize(r Rule) Rule {
	r.Name = strings.TrimSpace(r.Name)
	r.URL = strings.TrimSpace(r.URL)
	if r.Name == "" {
		r.Name = nameFor(r)
	}
	r.Selector = strings.TrimSpace(r.Selector)
	r.Path = strings.TrimSpace(r.Path)
	return r
}

func (s *Service) Create(ctx context.Context, r Rule) (View, error) {
	r = normalize(r)
	if err := s.validate(r); err != nil {
		return View{}, err
	}
	r.Ended, r.Status, r.LastError, r.Seen = false, "idle", "", []string{}
	r.NextRun = s.nowMs() + intervalMs(r)
	out, err := s.rules.Create(ctx, r)
	return s.view(out), err
}

func (s *Service) Update(ctx context.Context, r Rule) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r = normalize(r)
	if err := s.validate(r); err != nil {
		return View{}, err
	}
	old, err := s.rules.Get(ctx, r.ID)
	if err != nil {
		return View{}, err
	}
	reschedule := old.Every != r.Every || old.Unit != r.Unit || (!old.Enabled && r.Enabled)
	r.Seen, r.LastRun, r.LastFound, r.LastNew, r.Status, r.LastError, r.NextRun = old.Seen, old.LastRun, old.LastFound, old.LastNew, old.Status, old.LastError, old.NextRun
	r.Ended = false
	if reschedule {
		r.NextRun = s.nowMs() + intervalMs(r)
	}
	return s.view(r), s.rules.Put(ctx, r)
}

func (s *Service) SetEnabled(ctx context.Context, id int64, on bool) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.rules.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if on && r.EndDate > 0 && s.nowMs() > r.EndDate {
		return View{}, Invalid(r.Name + "’s end date has passed. Edit it to pick a later date.")
	}
	r.Enabled = on
	if on {
		r.Ended = false
		r.NextRun = s.nowMs() + intervalMs(r)
	}
	return s.view(r), s.rules.Put(ctx, r)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.rules.Delete(ctx, id)
}

func (s *Service) ClearPreset(id int64) {
	ctx := context.Background()
	rs, err := s.rules.List(ctx)
	if err != nil {
		return
	}
	for _, r := range rs {
		if r.RenamePreset == id {
			r.RenamePreset = 0
			_ = s.rules.Put(ctx, r)
		}
	}
}

func (s *Service) UsersOf(id int64) []string {
	rs, err := s.rules.List(context.Background())
	if err != nil {
		return nil
	}
	var out []string
	for _, r := range rs {
		if r.RenamePreset == id {
			out = append(out, r.Name)
		}
	}
	return out
}

func (s *Service) addLog(ctx context.Context, e LogEntry) {
	e.T = s.nowMs()
	if e.Added == nil {
		e.Added = []string{}
	}
	_, _ = s.log.Create(ctx, e)
	if items, err := s.log.List(ctx); err == nil && len(items) > logKeep {
		for _, l := range items[:len(items)-logKeep] {
			_ = s.log.Delete(ctx, l.ID)
		}
	}
}

func (s *Service) Log(ctx context.Context) ([]LogEntry, error) {
	items, err := s.log.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	return items, nil
}

func (s *Service) pageFor(ctx context.Context, r Rule) (string, error) {
	urls := searchURLs(r)
	if len(urls) == 0 {
		return "", errors.New("The schedule has no search URL.")
	}
	var b strings.Builder
	for _, u := range urls {
		body, err := s.fetchPaced(ctx, u)
		if err != nil {
			return "", err
		}
		b.WriteString(body)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func (s *Service) extract(ctx context.Context, r Rule, page string) Extraction {
	ts, _ := s.tor.Snapshot(ctx)
	return Extract(r, page, known{ts})
}

func (s *Service) Test(ctx context.Context, r Rule) (Extraction, error) {
	r = normalize(r)
	if len(searchURLs(r)) == 0 {
		return Extraction{}, Invalid("Enter the search URL, and a search term if it has {query}.")
	}
	if r.Selector == "" {
		return Extraction{}, Invalid("Enter a CSS selector in step 3.")
	}
	if r.Max < 1 {
		r.Max = 5
	}
	if r.LinkType == "" {
		r.LinkType = "any"
	}
	if r.Attr == "" {
		r.Attr = "href"
	}
	page, err := s.pageFor(ctx, r)
	if err != nil {
		return Extraction{Error: err.Error(), Items: []Item{}}, nil
	}
	return s.extract(ctx, r, page), nil
}

func (s *Service) Preview(ctx context.Context, id int64) (PreviewResult, error) {
	r, err := s.rules.Get(ctx, id)
	if err != nil {
		return PreviewResult{}, err
	}
	res := PreviewResult{Urls: searchURLs(r)}
	page, err := s.pageFor(ctx, r)
	if err != nil {
		res.Extraction = Extraction{Error: err.Error(), Items: []Item{}}
		return res, nil
	}
	res.Extraction = s.extract(ctx, r, page)
	return res, nil
}

func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

func (s *Service) addItems(ctx context.Context, r *Rule, items []Item) []string {
	var added []string
	for _, i := range items {
		tracker := hostOf(i.URL)
		if i.Type == "magnet" {
			tracker = "DHT only"
		}
		spec := torrents.AddSpec{
			Name: i.Name, Hash: i.Btih, Size: (0.4 + rand.Float64()*5) * torrents.GiB, Path: r.Path, Tracker: tracker,
			Start: r.Start, Preset: r.RenamePreset, Source: "schedule", Link: i.URL,
			Schedule: map[string]any{"id": r.ID, "name": r.Name}, Tags: append([]string{}, r.Tags...),
		}
		if i.Type == "magnet" {
			spec.Magnet = i.URL
		} else {
			spec.TorrentURL = i.URL
		}
		_, err := s.tor.Add(ctx, spec)
		switch {
		case err == nil:
			added = append(added, i.Name)
			r.Seen = append(r.Seen, i.Key)
		case strings.Contains(err.Error(), "already in your list"):
			r.Seen = append(r.Seen, i.Key)
		default:
			s.addLog(ctx, LogEntry{Rule: r.Name, Kind: "err", Text: "Couldn’t add " + i.Name + ": " + err.Error()})
		}
	}
	if len(r.Seen) > seenCap {
		r.Seen = r.Seen[len(r.Seen)-seenCap:]
	}
	return added
}

func plural(n int, w string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", w)
	}
	if strings.HasSuffix(w, "ch") {
		return fmt.Sprintf("%d %ses", n, w)
	}
	return fmt.Sprintf("%d %ss", n, w)
}

func (s *Service) Run(ctx context.Context, id int64, how string) (View, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.rules.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	s.run(ctx, &r, how)
	return s.view(r), s.rules.Put(ctx, r)
}

func (s *Service) run(ctx context.Context, r *Rule, how string) {
	urls := searchURLs(*r)
	r.LastRun = s.nowMs()
	r.NextRun = r.LastRun + intervalMs(*r)
	sched := map[string]any{"id": r.ID, "name": r.Name}
	s.bus.Emit("schedule.search", map[string]any{"schedule": sched, "urls": urls, "trigger": how})

	fail := func(msg string) {
		r.Status, r.LastError = "error", msg
		s.addLog(ctx, LogEntry{Rule: r.Name, Kind: "err", Text: msg})
	}
	var pathErr error
	if s.check != nil && r.Path != "" {
		pathErr = s.check(r.Path)
	}
	page, err := "", pathErr
	if err == nil {
		page, err = s.pageFor(ctx, *r)
	}
	switch {
	case err != nil:
		fail(err.Error())
	default:
		res := s.extract(ctx, *r, page)
		if res.Error != "" {
			fail(res.Error)
			break
		}
		fresh := []Item{}
		filtered := 0
		for _, i := range res.Items {
			switch i.State {
			case "new":
				fresh = append(fresh, i)
			case "filtered":
				filtered++
			}
		}
		links := make([]map[string]any, len(res.Items))
		for i, it := range res.Items {
			links[i] = map[string]any{"name": it.Name, "type": it.Type, "url": it.URL, "result": it.State}
		}
		s.bus.Emit("schedule.links", map[string]any{"schedule": sched, "count": len(res.Items), "new": len(fresh), "links": links})
		added := s.addItems(ctx, r, fresh)
		r.Status, r.LastError, r.LastFound, r.LastNew = "ok", "", len(res.Items), len(added)
		text := fmt.Sprintf("%s · %s matched", plural(len(urls), "search"), plural(len(res.Items), "link"))
		if filtered > 0 {
			text += fmt.Sprintf(" · %d filtered out", filtered)
		}
		kind := "none"
		if len(added) > 0 {
			kind = "ok"
			text += fmt.Sprintf(" · %d new", len(added))
		} else {
			text += " · nothing new"
		}
		s.addLog(ctx, LogEntry{Rule: r.Name, Kind: kind, Text: text, Added: added})
	}
	data := map[string]any{"schedule": sched, "status": r.Status, "count": 0, "error": nil}
	if r.Status == "ok" {
		data["count"] = r.LastNew
	} else {
		data["error"] = r.LastError
	}
	s.bus.Emit("schedule.finished", data)
}

func (s *Service) DownloadFromPreview(ctx context.Context, id int64, keys []string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.rules.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	page, err := s.pageFor(ctx, r)
	if err != nil {
		return 0, Invalid(err.Error())
	}
	res := s.extract(ctx, r, page)
	var pick []Item
	for _, i := range res.Items {
		if slices.Contains(keys, i.Key) && i.State != "have" {
			pick = append(pick, i)
		}
	}
	added := s.addItems(ctx, &r, pick)
	kind := "none"
	if len(added) > 0 {
		kind = "ok"
	}
	s.addLog(ctx, LogEntry{Rule: r.Name, Kind: kind, Text: fmt.Sprintf("Downloaded from preview · %d added", len(added)), Added: added})
	return len(added), s.rules.Put(ctx, r)
}

func (s *Service) endRule(ctx context.Context, r *Rule) {
	r.Enabled, r.Ended = false, true
	day := time.UnixMilli(r.EndDate).Format("Jan 2, 2006")
	s.addLog(ctx, LogEntry{Rule: r.Name, Kind: "none", Text: "Reached its end date (" + day + ") and turned off"})
}

func (s *Service) Tick(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rs, err := s.rules.List(ctx)
	if err != nil {
		return
	}
	now := s.nowMs()
	for _, r := range rs {
		if !r.Enabled {
			continue
		}
		switch {
		case r.EndDate > 0 && now > r.EndDate:
			s.endRule(ctx, &r)
		case r.NextRun <= now:
			s.run(ctx, &r, "schedule")
			if r.EndDate > 0 && r.NextRun > r.EndDate {
				s.endRule(ctx, &r)
			}
		default:
			continue
		}
		_ = s.rules.Put(ctx, r)
	}
}

func (s *Service) Loop(ctx context.Context) {
	tk := time.NewTicker(time.Second)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			s.Tick(ctx)
		}
	}
}

func (s *Service) NextRun(ctx context.Context) (string, int64, bool) {
	rs, err := s.rules.List(ctx)
	if err != nil {
		return "", 0, false
	}
	var best *Rule
	for i := range rs {
		if rs[i].Enabled && (best == nil || rs[i].NextRun < best.NextRun) {
			best = &rs[i]
		}
	}
	if best == nil {
		return "", 0, false
	}
	return best.Name, best.NextRun, true
}

type OldEntry struct {
	ID    int64
	Label string
	T     int64
}

func (s *Service) Source() string { return "Schedules" }

func (s *Service) Old(cutoff int64) []OldEntry {
	items, err := s.log.List(context.Background())
	if err != nil {
		return nil
	}
	var out []OldEntry
	for _, l := range items {
		if l.T < cutoff {
			out = append(out, OldEntry{ID: l.ID, Label: l.Rule, T: l.T})
		}
	}
	return out
}

func (s *Service) Remove(id int64) { _ = s.log.Delete(context.Background(), id) }
