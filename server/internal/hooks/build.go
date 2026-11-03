package hooks

import (
	"bitswarm/server/internal/buildinfo"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"bitswarm/server/internal/jsonutil"
)

var Version = buildinfo.Version

var (
	placeholderRe = regexp.MustCompile(`\{\{\s*([\w.]+)\s*\}\}`)
	headerNameRe  = regexp.MustCompile(`^[A-Za-z0-9-]+$`)
	jsonTypeRe    = regexp.MustCompile(`(?i)json`)
)

func Sq(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

func normalize(data map[string]any) map[string]any {
	raw, err := jsonutil.Marshal(data)
	out := map[string]any{}
	if err == nil {
		_ = looseUnmarshal(raw, &out)
	}
	return out
}

func str(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	raw, _ := jsonutil.Marshal(v)
	return string(raw)
}

func sub(m map[string]any, k string) map[string]any {
	v, _ := m[k].(map[string]any)
	return v
}

func Vars(event string, data map[string]any, now time.Time) map[string]string {
	d := normalize(data)
	v := map[string]string{"event": event, "time": now.UTC().Format("2006-01-02T15:04:05.000Z")}
	if t := sub(d, "torrent"); t != nil {
		var tags []string
		if arr, ok := t["tags"].([]any); ok {
			for _, x := range arr {
				tags = append(tags, str(x))
			}
		}
		size, _ := t["size"].(float64)
		v["torrent.name"], v["torrent.hash"] = str(t["name"]), str(t["hash"])
		v["torrent.tags"], v["torrent.size"] = strings.Join(tags, ", "), str(t["size_text"])
		v["torrent.bytes"], v["torrent.path"] = strconv.FormatInt(int64(size+0.5), 10), str(t["save_path"])
		v["torrent.ratio"], v["torrent.state"] = str(t["ratio"]), str(t["state"])
	}
	if s := sub(d, "schedule"); s != nil {
		v["schedule.name"] = str(s["name"])
	}
	if t := sub(d, "task"); t != nil {
		v["task.name"] = str(t["name"])
	}
	if c, ok := d["count"]; ok {
		v["count"] = str(c)
	}
	payload := map[string]any{"event": event, "time": v["time"]}
	for k, val := range d {
		payload[k] = val
	}
	raw, _ := jsonutil.Marshal(payload)
	v["payload"] = string(raw)
	return v
}

func Fill(tpl string, vars map[string]string, mode string) string {
	return placeholderRe.ReplaceAllStringFunc(tpl, func(m string) string {
		k := placeholderRe.FindStringSubmatch(m)[1]
		if strings.HasPrefix(k, "secret.") {
			return m
		}
		val, ok := vars[k]
		if !ok {
			return ""
		}
		switch {
		case mode == "shell":
			return Sq(val)
		case mode == "url":
			return strings.ReplaceAll(url.QueryEscape(val), "+", "%20")
		case mode == "json" && k != "payload":
			return jsonString(val)
		}
		return val
	})
}

func ParseHeaders(txt string) [][2]string {
	var out [][2]string
	for _, l := range strings.Split(txt, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if i := strings.Index(l, ":"); i > 0 {
			out = append(out, [2]string{strings.TrimSpace(l[:i]), strings.TrimSpace(l[i+1:])})
		} else {
			out = append(out, [2]string{l, "\x00"})
		}
	}
	return out
}

func prettyJSON(s string) string {
	var v any
	if err := looseUnmarshal([]byte(s), &v); err != nil {
		return s
	}
	raw, err := jsonIndent(v)
	if err != nil {
		return s
	}
	return raw
}

func Build(h Hook, event string, data map[string]any, now time.Time) Delivery {
	vars := Vars(event, data, now)
	d := Delivery{T: now.UnixMilli(), HookID: h.ID, Hook: h.Name, Event: event, Kind: h.Kind, Timeout: h.Timeout, Headers: [][2]string{}, Env: [][2]string{}}
	if h.Kind == "cmd" {
		d.Command = Fill(h.Cmd, vars, "shell")
		d.Cwd = h.Cwd
		d.Env = [][2]string{{"SWARM_EVENT", event}, {"SWARM_TIME", vars["time"]}}
		if _, ok := vars["torrent.name"]; ok {
			d.Env = append(d.Env, [2]string{"SWARM_TORRENT_NAME", vars["torrent.name"]}, [2]string{"SWARM_TORRENT_HASH", vars["torrent.hash"]},
				[2]string{"SWARM_TORRENT_PATH", vars["torrent.path"]}, [2]string{"SWARM_TORRENT_SIZE", vars["torrent.bytes"]})
		}
		if s, ok := vars["schedule.name"]; ok {
			d.Env = append(d.Env, [2]string{"SWARM_SCHEDULE", s})
		}
		if t, ok := vars["task.name"]; ok {
			d.Env = append(d.Env, [2]string{"SWARM_TASK", t})
		}
		if h.Stdin {
			d.Stdin = vars["payload"]
		}
		d.Text = cmdText(d)
		d.Copy = shellOf(d)
		return d
	}
	d.Method = h.Method
	hs := [][2]string{}
	for _, kv := range ParseHeaders(h.Headers) {
		if kv[1] != "\x00" {
			hs = append(hs, kv)
		}
	}
	isJSON := false
	for _, kv := range hs {
		if strings.EqualFold(kv[0], "content-type") && jsonTypeRe.MatchString(kv[1]) {
			isJSON = true
		}
	}
	d.URL = Fill(h.URL, vars, "url")
	if h.Method != "GET" {
		mode := "text"
		if isJSON {
			mode = "json"
		}
		d.Body = Fill(h.Body, vars, mode)
	}
	for _, kv := range hs {
		d.Headers = append(d.Headers, [2]string{kv[0], Fill(kv[1], vars, "text")})
	}
	d.Headers = append(d.Headers, [2]string{"User-Agent", "Bitswarm/" + Version}, [2]string{"X-Swarm-Event", event})
	d.Text = reqText(d)
	d.Copy = curlOf(d)
	return d
}

func Sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func cmdText(d Delivery) string {
	var b strings.Builder
	if d.Cwd != "" {
		fmt.Fprintf(&b, "cd %s\n", d.Cwd)
	}
	fmt.Fprintf(&b, "$ %s\n\n# Environment\n", d.Command)
	for i, kv := range d.Env {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s=%s", kv[0], kv[1])
	}
	if d.Stdin != "" {
		fmt.Fprintf(&b, "\n\n# Standard input\n%s", prettyJSON(d.Stdin))
	}
	fmt.Fprintf(&b, "\n\n# Stopped if it runs longer than %d s", d.Timeout)
	return b.String()
}

func shellOf(d Delivery) string {
	var b strings.Builder
	if d.Cwd != "" {
		fmt.Fprintf(&b, "cd %s && ", Sq(d.Cwd))
	}
	if d.Stdin != "" {
		fmt.Fprintf(&b, "printf '%%s' %s | ", Sq(d.Stdin))
	}
	b.WriteString("env")
	for _, kv := range d.Env {
		fmt.Fprintf(&b, " %s=%s", kv[0], Sq(kv[1]))
	}
	b.WriteString(" " + d.Command)
	return b.String()
}

func reqText(d Delivery) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", d.Method, d.URL)
	for i, kv := range d.Headers {
		if i > 0 {
			b.WriteByte('\n')
		}
		fmt.Fprintf(&b, "%s: %s", kv[0], kv[1])
	}
	if d.Body != "" {
		b.WriteString("\n\n" + prettyJSON(d.Body))
	}
	return b.String()
}

func curlOf(d Delivery) string {
	parts := []string{fmt.Sprintf("curl -X %s %s", d.Method, Sq(d.URL))}
	for _, kv := range d.Headers {
		parts = append(parts, "  -H "+Sq(kv[0]+": "+kv[1]))
	}
	if d.Body != "" {
		parts = append(parts, "  --data "+Sq(d.Body))
	}
	return strings.Join(parts, " \\\n")
}

func templateOf(h Hook) string {
	if h.Kind == "cmd" {
		return h.Cmd
	}
	return h.URL + h.Body + h.Headers
}

func Placeholders_(h Hook) (names []string) {
	for _, m := range placeholderRe.FindAllStringSubmatch(templateOf(h), -1) {
		if strings.HasPrefix(m[1], "secret.") {
			continue
		}
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	return names
}
