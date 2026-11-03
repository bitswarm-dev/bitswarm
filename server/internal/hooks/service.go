package hooks

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/jsonutil"
	"bitswarm/server/internal/redact"
)

var ErrNotFound = docstore.ErrNotFound

type Invalid string

func (e Invalid) Error() string { return string(e) }

type Logger interface {
	Add(level, src, msg string)
}

type Samples struct {
	Torrent  func(ctx context.Context) map[string]any
	Schedule func(ctx context.Context) (id int64, name string, urls []string)
}

type Service struct {
	hooks      *docstore.Collection[Hook]
	deliveries *docstore.Collection[Delivery]
	log        Logger
	samples    Samples
	commands   bool
	client     *http.Client
	now        func() time.Time
	sleep      func(time.Duration)
	sem        chan struct{}
	wg         sync.WaitGroup
}

func NewCollections(db docstoreDB) (*docstore.Collection[Hook], *docstore.Collection[Delivery]) {
	return docstore.NewCollection(db, "hooks", func(h *Hook) *int64 { return &h.ID }),
		docstore.NewCollection(db, "deliveries", func(d *Delivery) *int64 { return &d.ID })
}

func NewService(hooks *docstore.Collection[Hook], deliveries *docstore.Collection[Delivery], log Logger, allowCommands bool) *Service {
	return &Service{hooks: hooks, deliveries: deliveries, log: log, commands: allowCommands, client: &http.Client{Timeout: 20 * time.Second},
		now: time.Now, sleep: time.Sleep, sem: make(chan struct{}, 8)}
}

func (s *Service) SetSamples(sm Samples) { s.samples = sm }

func (s *Service) Config() Config { return Config{CommandsEnabled: s.commands} }

func (s *Service) Wait() { s.wg.Wait() }

func (s *Service) view(h Hook) View {
	v := View{Hook: h, HasSecret: h.Secret != ""}
	v.Secret, v.ClearSecret = "", false
	v.Headers = maskHeaderText(h.Headers)
	return v
}

func (s *Service) List(ctx context.Context) ([]View, error) {
	hs, err := s.hooks.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]View, len(hs))
	for i, h := range hs {
		out[i] = s.view(h)
	}
	return out, nil
}

var cwdRe = regexp.MustCompile(`^(/|~|[A-Za-z]:[\\/])`)

func (s *Service) sample(ctx context.Context, event string) map[string]any {
	var t map[string]any
	if s.samples.Torrent != nil {
		t = s.samples.Torrent(ctx)
	}
	if t == nil {
		t = map[string]any{"id": 0, "hash": strings.Repeat("0", 40), "name": "example.iso", "tags": []string{}, "size": 1, "size_text": "1 B", "progress": 1, "ratio": 0, "state": "seeding", "save_path": "/downloads"}
	}
	switch {
	case strings.HasPrefix(event, "torrent."):
		d := map[string]any{"torrent": t}
		switch event {
		case "torrent.added":
			d["source"] = "manual"
		case "torrent.renamed":
			d["preset"], d["from"], d["to"] = "Tidy Ubuntu ISO names", "old-name.iso", t["name"]
		}
		return d
	case strings.HasPrefix(event, "schedule."):
		id, name, urls := int64(1), "My schedule", []string{}
		if s.samples.Schedule != nil {
			id, name, urls = s.samples.Schedule(ctx)
		}
		d := map[string]any{"schedule": map[string]any{"id": id, "name": name}}
		switch event {
		case "schedule.search":
			d["urls"], d["trigger"] = urls, "schedule"
		case "schedule.links":
			d["count"], d["links"] = 2, []any{}
		default:
			d["status"], d["count"] = "ok", 2
		}
		return d
	case event == "maintenance.run":
		return map[string]any{"task": map[string]any{"id": 1, "name": "Retry torrents with errors"}, "count": 1, "result": "Retried 1 torrent", "items": []string{fmt.Sprint(t["name"])}}
	case event == "update.available":
		return map[string]any{"current": Version, "latest": "1.0.0"}
	}
	return map[string]any{}
}

func (s *Service) validate(ctx context.Context, h Hook) error {
	if strings.TrimSpace(h.Name) == "" {
		return Invalid("Give the hook a name.")
	}
	if h.Kind != "api" && h.Kind != "cmd" {
		return Invalid("Choose whether to call an API or run a command.")
	}
	for _, e := range h.Events {
		if !slices.Contains(Events, e) {
			return Invalid("“" + e + "” isn’t an event Bitswarm knows.")
		}
	}
	retries := Invalid("Retries must be a whole number from 0 to 10.")
	if h.Kind == "cmd" {
		if secretRe.MatchString(h.Cmd) {
			return Invalid("Command hooks can’t use {{secret.NAME}}. API hooks can. In a command, read the key from your own file or environment instead.")
		}
		switch {
		case strings.TrimSpace(h.Cmd) == "":
			return Invalid("Enter the command to run.")
		case h.Cwd != "" && !cwdRe.MatchString(h.Cwd):
			return Invalid("The working folder must be a full path, such as /home/me/scripts.")
		case len(h.Events) == 0:
			return Invalid("Pick at least one event in step 2.")
		case h.Timeout < 1 || h.Timeout > 3600:
			return Invalid("The time limit must be a whole number of seconds from 1 to 3600.")
		case h.Retries < 0 || h.Retries > 10:
			return retries
		}
		return nil
	}
	if !slices.Contains([]string{"POST", "PUT", "PATCH", "GET"}, h.Method) {
		return Invalid("Choose POST, PUT, PATCH or GET.")
	}
	for _, n := range secretNames(h) {
		if !secretNameRe.MatchString(n) {
			return Invalid("Secret names use capital letters, digits and underscores, like {{secret.RESEND_KEY}}. “" + n + "” doesn’t.")
		}
	}
	raw := Fill(h.URL, Vars("torrent.completed", s.sample(ctx, "torrent.completed"), s.now()), "url")
	if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Invalid("Enter the full URL to call, starting with https:// or http://.")
	}
	if len(h.Events) == 0 {
		return Invalid("Pick at least one event in step 2.")
	}
	for _, kv := range ParseHeaders(h.Headers) {
		if kv[1] == "\x00" || !headerNameRe.MatchString(kv[0]) {
			shown := kv[0]
			if kv[1] != "\x00" {
				shown += ": " + kv[1]
			}
			return Invalid("“" + shown + "” isn’t a valid header. Write each one as Name: value.")
		}
	}
	if h.Retries < 0 || h.Retries > 10 {
		return retries
	}
	if h.Method != "GET" && strings.TrimSpace(h.Body) != "" {
		for _, kv := range ParseHeaders(h.Headers) {
			if strings.EqualFold(kv[0], "content-type") && jsonTypeRe.MatchString(kv[1]) {
				for _, ev := range h.Events {
					d := Build(h, ev, s.sample(ctx, ev), s.now())
					var v any
					if err := looseUnmarshal([]byte(d.Body), &v); err != nil {
						return Invalid("With a sample " + ev + " event the body isn’t valid JSON. Check the quotes and commas around your placeholders.")
					}
				}
			}
		}
	}
	return nil
}

func (s *Service) Create(ctx context.Context, h Hook) (View, error) {
	h.Name = strings.TrimSpace(h.Name)
	if err := s.validate(ctx, h); err != nil {
		return View{}, err
	}
	h.Count, h.Last, h.LastEvent, h.ClearSecret = 0, 0, "", false
	out, err := s.hooks.Create(ctx, h)
	return s.view(out), err
}

func (s *Service) Update(ctx context.Context, h Hook) (View, error) {
	h.Name = strings.TrimSpace(h.Name)
	if err := s.validate(ctx, h); err != nil {
		return View{}, err
	}
	old, err := s.hooks.Get(ctx, h.ID)
	if err != nil {
		return View{}, err
	}
	h.Count, h.Last, h.LastEvent = old.Count, old.Last, old.LastEvent
	h.Headers = restoreHeaders(h.Headers, old.Headers)
	switch {
	case h.ClearSecret:
		h.Secret = ""
	case h.Secret == "":
		h.Secret = old.Secret
	}
	h.ClearSecret = false
	return s.view(h), s.hooks.Put(ctx, h)
}

func (s *Service) SetEnabled(ctx context.Context, id int64, on bool) (View, error) {
	h, err := s.hooks.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	h.Enabled = on
	return s.view(h), s.hooks.Put(ctx, h)
}

func (s *Service) Duplicate(ctx context.Context, id int64) (View, error) {
	h, err := s.hooks.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	h.Name += " copy"
	h.Events = slices.Clone(h.Events)
	h.Enabled, h.Count, h.Last, h.LastEvent = false, 0, 0, ""
	out, err := s.hooks.Create(ctx, h)
	return s.view(out), err
}

func (s *Service) Delete(ctx context.Context, id int64) error { return s.hooks.Delete(ctx, id) }

func (s *Service) Deliveries(ctx context.Context) ([]Delivery, error) {
	items, err := s.deliveries.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.Reverse(items)
	return items, nil
}

func (s *Service) Preview(ctx context.Context, h Hook) Preview {
	event := "torrent.completed"
	if len(h.Events) > 0 {
		event = h.Events[0]
	}
	data := s.sample(ctx, event)
	d := Build(h, event, data, s.now())
	vars := Vars(event, data, s.now())
	p := Preview{Event: event, Text: maskDelivery(d).Text, Unknown: []string{}, Empty: []string{}, Signed: h.Kind != "cmd" && h.Secret != "", MissingSecrets: []string{}}
	if h.Kind != "cmd" {
		p.MissingSecrets = MissingSecrets(h)
	}
	if h.Kind != "cmd" && h.Secret != "" {
		p.Text += "\n(X-Swarm-Signature: sha256=… is added when sent)"
	}
	for _, name := range Placeholders_(h) {
		switch _, has := vars[name]; {
		case !slices.Contains(Placeholders, name):
			p.Unknown = append(p.Unknown, name)
		case !has:
			p.Empty = append(p.Empty, name)
		}
	}
	return p
}

func (s *Service) Emit(event string, data map[string]any) {
	ctx := context.Background()
	hs, err := s.hooks.List(ctx)
	if err != nil {
		return
	}
	var tags []string
	if t, ok := data["torrent"].(map[string]any); ok {
		tags, _ = t["tags"].([]string)
	}
	for _, h := range hs {
		if !h.Enabled || !slices.Contains(h.Events, event) {
			continue
		}
		if h.Tag != "" {
			if _, isTorrent := data["torrent"]; isTorrent && !slices.Contains(tags, h.Tag) {
				continue
			}
		}
		s.wg.Add(1)
		go func(h Hook) {
			defer s.wg.Done()
			s.sem <- struct{}{}
			defer func() { <-s.sem }()
			s.deliver(context.Background(), h, event, data, false)
		}(h)
	}
}

func (s *Service) Test(ctx context.Context, id int64) (Delivery, error) {
	h, err := s.hooks.Get(ctx, id)
	if err != nil {
		return Delivery{}, err
	}
	event := "torrent.completed"
	if len(h.Events) > 0 {
		event = h.Events[0]
	}
	h.Retries = 0
	return s.deliver(ctx, h, event, s.sample(ctx, event), true), nil
}

func (s *Service) deliver(ctx context.Context, h Hook, event string, data map[string]any, test bool) Delivery {
	d := Build(h, event, data, s.now())
	d.Test = test
	start := time.Now()

	if h.Kind == "cmd" {
		s.runCommand(ctx, &d)
	} else {
		if h.Secret != "" {
			d.Headers = append(d.Headers, [2]string{"X-Swarm-Signature", Sign(h.Secret, d.Body)})
			d.Text = reqText(d)
			d.Copy = curlOf(d)
		}
		s.send(ctx, h, &d)
	}
	d.Millis = time.Since(start).Milliseconds()

	d = maskDelivery(d)
	stored, err := s.deliveries.Create(ctx, d)
	if err == nil {
		d = stored
	}
	if all, err := s.deliveries.List(ctx); err == nil && len(all) > 50 {
		for _, old := range all[:len(all)-50] {
			_ = s.deliveries.Delete(ctx, old.ID)
		}
	}
	if cur, err := s.hooks.Get(ctx, h.ID); err == nil {
		cur.Count++
		cur.Last, cur.LastEvent = d.T, event
		_ = s.hooks.Put(ctx, cur)
	}
	level := "debug"
	if d.Status == "failed" {
		level = "warn"
	}
	s.log.Add(level, "Hooks", fmt.Sprintf("%s: %s for %s%s. %s", h.Name, map[string]string{"cmd": "command", "api": "request"}[h.Kind], event, map[bool]string{true: " (test)"}[test], StatusText(d)))
	return d
}

func StatusText(d Delivery) string {
	switch d.Status {
	case "sent":
		return fmt.Sprintf("Sent, the server answered %d.", d.Response)
	case "ran":
		return "Ran and exited with 0."
	case "not_run":
		return "Not run: " + d.Error
	}
	return "Failed: " + d.Error
}

func (s *Service) send(ctx context.Context, h Hook, d *Delivery) {
	attempts := h.Retries + 1
	for i := 1; i <= attempts; i++ {
		d.Attempts = i
		code, err := s.once(ctx, *d, h.Secret)
		d.Response = code
		var missing missingSecretError
		if errors.As(err, &missing) {
			d.Status, d.Error = "failed", err.Error()
			return
		}
		switch {
		case err == nil && code < 400:
			d.Status, d.Error = "sent", ""
			return
		case err == nil:
			d.Status, d.Error = "failed", fmt.Sprintf("the server answered %d", code)
		default:
			d.Status, d.Error = "failed", err.Error()
		}
		if err == nil && code < 500 && code != 429 {
			return
		}
		if i < attempts {
			s.sleep(time.Duration(1<<min(i-1, 5)) * time.Second)
		}
	}
}

func (s *Service) once(ctx context.Context, sent Delivery, signingSecret string) (int, error) {
	d, missing := resolveSecrets(sent)
	if len(missing) > 0 {
		return 0, missingSecretError{missing[0]}
	}
	if signingSecret != "" && d.Body != sent.Body {
		for i := range d.Headers {
			if strings.EqualFold(d.Headers[i][0], "X-Swarm-Signature") {
				d.Headers[i][1] = Sign(signingSecret, d.Body)
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, d.Method, d.URL, strings.NewReader(d.Body))
	if err != nil {
		return 0, errors.New(scrubSecrets(redact.Err(err), sent))
	}
	for _, kv := range d.Headers {
		req.Header.Set(kv[0], kv[1])
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, errors.New(scrubSecrets(strings.TrimPrefix(redact.Err(err), d.Method+" "), sent))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, nil
}

func (s *Service) runCommand(ctx context.Context, d *Delivery) {
	if !s.commands {
		d.Status, d.Error = "not_run", "commands are turned off on this server. Start it with SWARM_ALLOW_COMMANDS=1 to run them."
		return
	}
	d.Attempts = 1
	cctx, cancel := context.WithTimeout(ctx, time.Duration(d.Timeout)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, "/bin/sh", "-c", d.Command)
	killTree(cmd)
	if d.Cwd != "" {
		cmd.Dir = expandHome(d.Cwd)
	}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	for _, kv := range d.Env {
		cmd.Env = append(cmd.Env, kv[0]+"="+kv[1])
	}
	if d.Stdin != "" {
		cmd.Stdin = strings.NewReader(d.Stdin)
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	if len(out.Bytes()) > 4096 {
		d.Output = out.String()[:4096]
	} else {
		d.Output = out.String()
	}
	switch {
	case errors.Is(cctx.Err(), context.DeadlineExceeded):
		d.Status, d.Error = "failed", fmt.Sprintf("stopped after %d seconds", d.Timeout)
	case err != nil:
		d.Status, d.Error = "failed", err.Error()
	default:
		d.Status = "ran"
	}
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return home + strings.TrimPrefix(p, "~")
		}
	}
	return p
}

func (s *Service) Seed(ctx context.Context) error {
	items, err := s.hooks.List(ctx)
	if err != nil || len(items) > 0 {
		return err
	}
	for _, h := range []Hook{
		{Kind: "api", Name: "Chat alert: download finished", Method: "POST", URL: "https://chat.example.com/api/webhooks/7731/swarm", Headers: "Content-Type: application/json",
			Body: "{\n  \"text\": \"Finished downloading {{torrent.name}} ({{torrent.size}}) into {{torrent.path}}\"\n}", Events: []string{"torrent.completed", "torrent.error"}, Retries: 3, Timeout: 60},
		{Kind: "api", Name: "Home server event log", Method: "POST", URL: "https://home.example.net/api/swarm/events", Headers: "Content-Type: application/json\nAuthorization: Bearer YOUR_TOKEN",
			Body: "{{payload}}", Events: slices.Clone(Events), Secret: "change-me", Retries: 5, Timeout: 60},
		{Kind: "cmd", Name: "Post-process finished downloads", Cmd: "/home/media/scripts/on-finished.sh {{torrent.path}} {{torrent.name}}", Cwd: "/home/media/scripts", Timeout: 300, Stdin: true,
			Events: []string{"torrent.completed"}, Retries: 1},
		{Kind: "api", Name: "Media library refresh", Method: "GET", URL: "http://media.local:8096/library/refresh?path={{torrent.path}}&name={{torrent.name}}",
			Events: []string{"torrent.completed", "torrent.renamed"}, Tag: "4K", Retries: 3, Timeout: 60},
	} {
		h.Enabled = h.Name != "Media library refresh"
		if _, err := s.hooks.Create(ctx, h); err != nil {
			return err
		}
	}
	return nil
}

func Marshal(v any) string {
	raw, _ := jsonutil.Marshal(v)
	return string(raw)
}

func (s *Service) Export(ctx context.Context) ([]Hook, error) { return s.hooks.List(ctx) }

func (s *Service) Import(ctx context.Context, h Hook) error {
	h.Enabled = false
	_, err := s.Create(ctx, h)
	return err
}
