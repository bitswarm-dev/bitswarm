package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"bitswarm/server/internal/catalog"
	"bitswarm/server/internal/hooks"
	"bitswarm/server/internal/maintenance"
	"bitswarm/server/internal/renames"
	"bitswarm/server/internal/schedules"
	"bitswarm/server/internal/settings"
)

type Settings interface {
	Get() settings.Settings
	Import(ctx context.Context, partial map[string]any) ([]string, error)
}

type Catalog interface {
	Snapshot() catalog.Snapshot
	EnsureTags(names []string)
}

type Renames interface {
	List(ctx context.Context) ([]renames.View, error)
	Create(ctx context.Context, p renames.Preset) (renames.Preset, error)
	Delete(ctx context.Context, id int64) error
}

type Schedules interface {
	List(ctx context.Context) ([]schedules.View, error)
	Create(ctx context.Context, r schedules.Rule) (schedules.View, error)
	Delete(ctx context.Context, id int64) error
}

type Maintenance interface {
	List(ctx context.Context) ([]maintenance.View, error)
	Update(ctx context.Context, t maintenance.Task) (maintenance.View, error)
	SetEnabled(ctx context.Context, id int64, on bool) (maintenance.View, error)
}

type Hooks interface {
	Export(ctx context.Context) ([]hooks.Hook, error)
	List(ctx context.Context) ([]hooks.View, error)
	Import(ctx context.Context, h hooks.Hook) error
	Delete(ctx context.Context, id int64) error
}

type Logger interface {
	Add(level, src, msg string)
}

type Service struct {
	Settings    Settings
	Catalog     Catalog
	Renames     Renames
	Schedules   Schedules
	Maintenance Maintenance
	Hooks       Hooks
	Log         Logger
	now         func() time.Time
}

func NewService() *Service { return &Service{now: time.Now} }

func plural(n int, w string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", w)
	}
	return fmt.Sprintf("%d %ss", n, w)
}

func has(list []string, k string) bool { return slices.Contains(list, k) }

func (s *Service) Counts(ctx context.Context) map[string]string {
	out := map[string]string{}
	c := s.Catalog.Snapshot()
	out["tags"] = plural(len(c.Tags), "tag")
	if v, err := s.Schedules.List(ctx); err == nil {
		out["schedules"] = plural(len(v), "schedule")
	}
	if v, err := s.Renames.List(ctx); err == nil {
		out["presets"] = plural(len(v), "preset")
	}
	if v, err := s.Maintenance.List(ctx); err == nil {
		out["maintenance"] = plural(len(v), "task")
	}
	if v, err := s.Hooks.List(ctx); err == nil {
		out["hooks"] = plural(len(v), "hook")
	}
	return out
}

func (s *Service) Export(ctx context.Context, r ExportReq) (ExportResult, error) {
	if len(r.Parts) == 0 {
		return ExportResult{}, Invalid("Choose at least one part to export")
	}
	out := map[string]any{}
	P := map[string]any{}
	if has(r.Parts, "settings") {
		m := toMap(s.Settings.Get())
		delete(m, "webPass")
		if !r.Secrets {
			delete(m, "proxyPass")
		}
		P["settings"] = m
	}
	if has(r.Parts, "tags") {
		P["tags"] = s.Catalog.Snapshot().Tags
	}
	presets, err := s.Renames.List(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	nameOf := map[int64]string{}
	for _, p := range presets {
		nameOf[p.ID] = p.Name
	}
	if has(r.Parts, "presets") {
		items := []map[string]any{}
		for _, p := range presets {
			items = append(items, map[string]any{"name": p.Name, "find": p.Find, "replace": p.Replace, "i": p.I, "g": p.G, "keepExt": p.KeepExt, "target": p.Target, "tag": p.Tag, "when": p.When, "enabled": p.Enabled})
		}
		P["rename_presets"] = items
	}
	if has(r.Parts, "schedules") {
		rules, err := s.Schedules.List(ctx)
		if err != nil {
			return ExportResult{}, err
		}
		items := []map[string]any{}
		for _, x := range rules {
			var end any
			if x.EndDate > 0 {
				end = time.UnixMilli(x.EndDate).UTC().Format(time.RFC3339)
			}
			var preset any
			if n, ok := nameOf[x.RenamePreset]; ok {
				preset = n
			}
			items = append(items, map[string]any{"name": x.Name, "url": x.URL, "terms": x.Terms, "selector": x.Selector, "attr": x.Attr, "linkType": x.LinkType,
				"include": x.Include, "exclude": x.Exclude, "tags": x.Tags, "path": x.Path, "max": x.Max, "start": x.Start, "skipSeen": x.SkipSeen, "every": x.Every, "unit": x.Unit,
				"enabled": x.Enabled, "endDate": end, "renamePreset": preset})
		}
		P["schedules"] = items
	}
	if has(r.Parts, "maintenance") {
		tasks, err := s.Maintenance.List(ctx)
		if err != nil {
			return ExportResult{}, err
		}
		items := []map[string]any{}
		for _, t := range tasks {
			items = append(items, map[string]any{"type": t.Type, "enabled": t.Enabled, "every": t.Every, "unit": t.Unit, "tag": t.Tag, "settings": toMap(t.P)})
		}
		P["maintenance"] = items
	}
	if has(r.Parts, "hooks") {
		hs, err := s.Hooks.Export(ctx)
		if err != nil {
			return ExportResult{}, err
		}
		items := []map[string]any{}
		for _, h := range hs {
			o := map[string]any{"name": h.Name, "kind": h.Kind, "events": h.Events, "tag": h.Tag, "retries": h.Retries, "enabled": h.Enabled}
			if h.Kind == "cmd" {
				o["command"], o["cwd"], o["timeout"], o["stdin"] = h.Cmd, h.Cwd, h.Timeout, h.Stdin
			} else {
				headers := h.Headers
				if !r.Secrets {
					lines := strings.Split(headers, "\n")
					for i, l := range lines {
						if j := strings.Index(l, ":"); j > 0 && hooks.IsSecretHeader(l[:j]) && !hooks.HasSecretRef(l[j+1:]) {
							lines[i] = l[:j] + ": <removed>"
						}
					}
					headers = strings.Join(lines, "\n")
				}
				o["method"], o["url"], o["headers"], o["body"] = h.Method, h.URL, headers, h.Body
			}
			if r.Secrets && h.Secret != "" {
				o["secret"] = h.Secret
			}
			items = append(items, o)
		}
		P["hooks"] = items
	}
	out["swarm_backup"], out["app_version"], out["exported_at"], out["parts"] = 1, Version, s.now().UTC().Format("2006-01-02T15:04:05.000Z"), P

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return ExportResult{}, err
	}
	text := strings.TrimRight(buf.String(), "\n")
	names := []string{}
	for _, k := range Parts {
		if has(r.Parts, k) {
			names = append(names, strings.ToLower(PartNames[k]))
		}
	}
	secrets := ""
	if r.Secrets {
		secrets = ", including secrets"
	}
	s.Log.Add("info", "Backup", "Created an export of "+strings.Join(names, ", ")+secrets)
	return ExportResult{JSON: text, Size: len(text)}, nil
}

type parsed struct {
	from, at string
	parts    map[string]any
}

func parse(text string) (*parsed, error) {
	var j map[string]any
	if err := json.Unmarshal([]byte(text), &j); err != nil {
		return nil, Invalid("This isn’t valid JSON. Paste the whole export, from the first { to the last }.")
	}
	parts, _ := j["parts"].(map[string]any)
	if v, _ := j["swarm_backup"].(float64); v != 1 || parts == nil {
		return nil, Invalid("This JSON isn’t a Bitswarm export. Bitswarm exports start with \"swarm_backup\": 1.")
	}
	from, _ := j["app_version"].(string)
	if from == "" {
		from = "unknown"
	}
	at, _ := j["exported_at"].(string)
	return &parsed{from: from, at: at, parts: parts}, nil
}

func arr(v any) []any {
	a, _ := v.([]any)
	return a
}

func obj(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func text(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func (s *Service) valid(part string, item any) bool {
	m := obj(item)
	if m == nil {
		return false
	}
	switch part {
	case "presets":
		if text(m["name"]) == "" || text(m["find"]) == "" {
			return false
		}
		_, err := regexp.Compile(fmt.Sprint(m["find"]))
		return err == nil
	case "schedules":
		return text(m["name"]) != "" && (text(m["url"]) != "" || text(m["template"]) != "") && text(m["selector"]) != ""
	case "maintenance":
		return slices.Contains([]string{"retry", "cleanup", "stalled", "disk", "verify", "logs"}, text(m["type"]))
	case "hooks":
		_, hasEvents := m["events"].([]any)
		if text(m["name"]) == "" || !hasEvents {
			return false
		}
		if text(m["kind"]) == "cmd" {
			return text(m["command"]) != ""
		}
		return text(m["url"]) != ""
	}
	return false
}

func (s *Service) Check(ctx context.Context, in string) (Plan, error) {
	p, err := parse(in)
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{From: p.from, At: p.at, Parts: []PlanPart{}}
	add := func(key, summary string) {
		plan.Parts = append(plan.Parts, PlanPart{Key: key, Name: PartNames[key], Summary: summary})
	}
	counted := func(key, jsonKey, noun string) {
		list := arr(p.parts[jsonKey])
		if len(list) == 0 {
			return
		}
		good := 0
		for _, it := range list {
			if s.valid(key, it) {
				good++
			}
		}
		sum := plural(good, noun)
		if good < len(list) {
			sum += fmt.Sprintf(" (%d invalid, skipped)", len(list)-good)
		}
		add(key, sum)
	}
	if m := obj(p.parts["settings"]); m != nil {
		cur := toMap(s.Settings.Get())
		n := 0
		for k := range m {
			if _, ok := cur[k]; ok {
				n++
			}
		}
		if n > 0 {
			add("settings", plural(n, "setting"))
		}
	}
	if t := obj(p.parts["theme"]); t != nil {
		plan.Theme = t
		add("theme", "mode, accent color and background")
	}
	if p.parts["tags"] != nil {
		g := 0
		for _, x := range arr(p.parts["tags"]) {
			if text(x) != "" {
				g++
			}
		}
		add("tags", plural(g, "tag"))
	}
	counted("presets", "rename_presets", "rename preset")
	counted("schedules", "schedules", "schedule")
	counted("maintenance", "maintenance", "maintenance task")
	counted("hooks", "hooks", "hook")
	if len(plan.Parts) == 0 {
		return Plan{}, Invalid("This export doesn’t contain anything Bitswarm can import.")
	}
	order := map[string]int{}
	for i, k := range Parts {
		order[k] = i
	}
	slices.SortFunc(plan.Parts, func(a, b PlanPart) int { return order[a.Key] - order[b.Key] })
	return plan, nil
}

func toMap(v any) map[string]any {
	raw, _ := json.Marshal(v)
	m := map[string]any{}
	_ = json.Unmarshal(raw, &m)
	return m
}

func intOf(v any, lo, hi, def int) int {
	f, ok := v.(float64)
	if !ok || f != float64(int(f)) || int(f) < lo || int(f) > hi {
		return def
	}
	return int(f)
}

func oneOf(v any, allowed []string, def string) string {
	if s, ok := v.(string); ok && slices.Contains(allowed, s) {
		return s
	}
	return def
}

func (s *Service) tagsFrom(v any) []string {
	tags := []string{}
	for _, x := range arr(v) {
		if t := text(x); t != "" {
			tags = append(tags, t)
		}
	}
	s.Catalog.EnsureTags(tags)
	return tags
}

func (s *Service) ensureTag(v any) string {
	name := text(v)
	if name != "" {
		s.Catalog.EnsureTags([]string{name})
	}
	return name
}

func (s *Service) Apply(ctx context.Context, r ApplyReq) (Result, error) {
	p, err := parse(r.Text)
	if err != nil {
		return Result{}, err
	}
	if len(r.Use) == 0 {
		return Result{}, Invalid("Choose at least one part to import.")
	}
	replace := r.Mode == "replace"
	res := Result{Done: []string{}, Skipped: []Skipped{}}
	skip := func(part, name string, err error) {
		res.Skipped = append(res.Skipped, Skipped{Part: PartNames[part], Name: name, Reason: err.Error()})
	}

	if has(r.Use, "settings") {
		kept, err := s.Settings.Import(ctx, obj(p.parts["settings"]))
		if err != nil {
			return res, err
		}
		note := ""
		if len(kept) > 0 {
			note = fmt.Sprintf(" (%s kept as before)", plural(len(kept), "invalid value"))
		}
		res.Done = append(res.Done, "settings"+note)
	}
	if has(r.Use, "tags") {
		s.tagsFrom(p.parts["tags"])
		res.Done = append(res.Done, "tags")
	}
	if has(r.Use, "presets") {
		existing, _ := s.Renames.List(ctx)
		if replace {
			for _, e := range existing {
				_ = s.Renames.Delete(ctx, e.ID)
			}
			existing = nil
		}
		n := 0
		for _, x := range arr(p.parts["rename_presets"]) {
			m := obj(x)
			if !s.valid("presets", x) {
				continue
			}
			name := text(m["name"])
			if !replace && slices.ContainsFunc(existing, func(e renames.View) bool { return e.Name == name }) {
				continue
			}
			pr := renames.Preset{Name: name, Find: fmt.Sprint(m["find"]), Replace: fmt.Sprint(orEmpty(m["replace"])), I: m["i"] == true, G: m["g"] == true, KeepExt: m["keepExt"] != false,
				Target: oneOf(m["target"], []string{"files", "torrent", "both"}, "files"), Tag: s.ensureTag(m["tag"]), When: oneOf(m["when"], []string{"add"}, "complete"), Enabled: m["enabled"] == true}
			if _, err := s.Renames.Create(ctx, pr); err != nil {
				skip("presets", name, err)
				continue
			}
			n++
		}
		res.Done = append(res.Done, plural(n, "rename preset"))
	}
	if has(r.Use, "schedules") {
		if replace {
			cur, _ := s.Schedules.List(ctx)
			for _, e := range cur {
				_ = s.Schedules.Delete(ctx, e.ID)
			}
		}
		existing, _ := s.Schedules.List(ctx)
		presets, _ := s.Renames.List(ctx)
		n := 0
		for _, x := range arr(p.parts["schedules"]) {
			m := obj(x)
			if !s.valid("schedules", x) {
				continue
			}
			name := text(m["name"])
			if !replace && slices.ContainsFunc(existing, func(e schedules.View) bool { return e.Name == name }) {
				continue
			}
			var preset int64
			for _, pr := range presets {
				if pr.Name == text(m["renamePreset"]) {
					preset = pr.ID
				}
			}
			var end int64
			enabled := m["enabled"] == true
			if t, err := time.Parse(time.RFC3339, text(m["endDate"])); err == nil {
				end = t.UnixMilli()
				if end < s.now().UnixMilli() {
					enabled = false
					end = 0
				}
			}
			rule := schedules.Rule{Name: name, URL: orDefault(text(m["url"]), text(m["template"])), Terms: fmt.Sprint(orEmpty(m["terms"])), Selector: text(m["selector"]),
				Attr: oneOf(m["attr"], []string{"href", "data-magnet", "data-href", "data-url", "text"}, "href"), LinkType: oneOf(m["linkType"], []string{"magnet", "torrent"}, "any"),
				Include: fmt.Sprint(orEmpty(m["include"])), Exclude: fmt.Sprint(orEmpty(m["exclude"])), Tags: s.tagsFrom(m["tags"]), Path: orDefault(text(m["path"]), s.Settings.Get().SavePath),
				Max: intOf(m["max"], 1, 50, 5), Start: m["start"] != false, SkipSeen: m["skipSeen"] != false, Every: intOf(m["every"], 1, 999, 6), Unit: oneOf(m["unit"], []string{"days"}, "hours"),
				Enabled: enabled, EndDate: end, RenamePreset: preset}
			if _, err := s.Schedules.Create(ctx, rule); err != nil {
				skip("schedules", name, err)
				continue
			}
			n++
		}
		res.Done = append(res.Done, plural(n, "schedule"))
	}
	if has(r.Use, "maintenance") {
		tasks, _ := s.Maintenance.List(ctx)
		n := 0
		for _, x := range arr(p.parts["maintenance"]) {
			m := obj(x)
			if !s.valid("maintenance", x) {
				continue
			}
			for _, t := range tasks {
				if t.Type != text(m["type"]) {
					continue
				}
				next := t.Task
				next.Every = intOf(m["every"], 1, 999, t.Every)
				next.Unit = oneOf(m["unit"], []string{"minutes", "hours", "days"}, t.Unit)
				next.Tag = ""
				if t.Scoped {
					next.Tag = s.ensureTag(m["tag"])
				}
				merged := toMap(t.P)
				for k, v := range obj(m["settings"]) {
					if old, ok := merged[k]; ok && fmt.Sprintf("%T", old) == fmt.Sprintf("%T", v) {
						merged[k] = v
					}
				}
				raw, _ := json.Marshal(merged)
				_ = json.Unmarshal(raw, &next.P)
				if _, err := s.Maintenance.Update(ctx, next); err != nil {
					skip("maintenance", t.Title, err)
					break
				}
				_, _ = s.Maintenance.SetEnabled(ctx, t.ID, m["enabled"] == true)
				n++
			}
		}
		res.Done = append(res.Done, plural(n, "maintenance task"))
	}
	if has(r.Use, "hooks") {
		if replace {
			cur, _ := s.Hooks.List(ctx)
			for _, e := range cur {
				_ = s.Hooks.Delete(ctx, e.ID)
			}
		}
		existing, _ := s.Hooks.List(ctx)
		n := 0
		for _, x := range arr(p.parts["hooks"]) {
			m := obj(x)
			if !s.valid("hooks", x) {
				continue
			}
			name := text(m["name"])
			if !replace && slices.ContainsFunc(existing, func(e hooks.View) bool { return e.Name == name }) {
				continue
			}
			events := []string{}
			for _, e := range arr(m["events"]) {
				if ev, ok := e.(string); ok && slices.Contains(hooks.Events, ev) {
					events = append(events, ev)
				}
			}
			h := hooks.Hook{Name: name, Kind: oneOf(m["kind"], []string{"cmd"}, "api"), Method: oneOf(m["method"], []string{"PUT", "PATCH", "GET"}, "POST"), URL: text(m["url"]),
				Headers: fmt.Sprint(orEmpty(m["headers"])), Body: fmt.Sprint(orEmpty(m["body"])), Cmd: text(m["command"]), Cwd: text(m["cwd"]), Timeout: intOf(m["timeout"], 1, 3600, 60),
				Stdin: m["stdin"] == true, Events: events, Tag: s.ensureTag(m["tag"]), Retries: intOf(m["retries"], 0, 10, 3), Secret: text(m["secret"])}
			if err := s.Hooks.Import(ctx, h); err != nil {
				skip("hooks", name, err)
				continue
			}
			n++
		}
		res.Done = append(res.Done, plural(n, "hook")+", turned off")
	}

	mode := ""
	if replace {
		mode = " (replace mode)"
	}
	res.Summary = "Imported " + strings.Join(res.Done, ", ")
	s.Log.Add("info", "Backup", "Imported "+strings.Join(res.Done, "; ")+mode)
	return res, nil
}

func orEmpty(v any) any {
	if v == nil {
		return ""
	}
	return v
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
