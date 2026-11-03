package eventlog

import (
	"fmt"
	"strings"
)

func SourceOf(event string) string {
	switch {
	case strings.HasPrefix(event, "torrent."):
		return "Torrents"
	case strings.HasPrefix(event, "schedule."):
		return "Schedules"
	case event == "maintenance.run":
		return "Maintenance"
	case event == "settings.saved":
		return "Settings"
	case event == "update.available":
		return "Updates"
	}
	return "App"
}

func str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func sub(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

func num(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func flag(m map[string]any, key string) bool {
	v, _ := m[key].(bool)
	return v
}

func Plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func why(reason string) string {
	if reason == "" || reason == "manual" {
		return ""
	}
	return " (" + strings.ReplaceAll(reason, "_", " ") + ")"
}

func tagList(t map[string]any) string {
	var out []string
	switch v := t["tags"].(type) {
	case []string:
		out = v
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		return "none"
	}
	return strings.Join(out, ", ")
}

func Describe(event string, d map[string]any) (level, msg string) {
	t := sub(d, "torrent")
	n := str(t, "name")
	sched := ""
	if s := sub(d, "schedule"); len(s) > 0 {
		sched = "“" + str(s, "name") + "”"
	}
	level = "info"

	switch event {
	case "torrent.added":
		msg = "Added " + n
		if str(d, "source") == "schedule" {
			msg += " from schedule " + sched
		}
		if total := num(d, "files_total"); total > 0 && num(d, "files_selected") < total {
			msg += fmt.Sprintf(", %d of %d files", num(d, "files_selected"), total)
		}
		if flag(d, "skip_hash_check") {
			msg += ", without a hash check"
		}
	case "torrent.metadata":
		msg = "Received the metadata for " + n
	case "torrent.resumed":
		msg = "Resumed " + n + why(str(d, "reason"))
	case "torrent.paused":
		msg = "Paused " + n + why(str(d, "reason"))
	case "torrent.completed":
		msg = "Finished downloading " + n
	case "torrent.error":
		level = "error"
		e := str(d, "error")
		if e == "" {
			e = "error"
		}
		msg = n + ": " + e
	case "torrent.removed":
		msg = "Removed " + n
		if flag(d, "deleted_files") {
			level = "warn"
			msg += " and deleted its files"
		}
		msg += why(str(d, "reason"))
	case "torrent.renamed":
		msg = fmt.Sprintf("Renamed “%s” to “%s” with preset “%s”", str(d, "from"), str(d, "to"), str(d, "preset"))
	case "torrent.tagged":
		msg = "Tags of " + n + " are now: " + tagList(t)
	case "torrent.moved":
		msg = fmt.Sprintf("Moved %s from %s to %s", n, str(d, "from"), str(d, "to"))
		if v, ok := d["moved_files"].(bool); ok && !v {
			msg += " (files left in place)"
		}
	case "schedule.search":
		urls, _ := d["urls"].([]string)
		msg = fmt.Sprintf("Schedule %s searched %s", sched, Plural(len(urls), "URL"))
	case "schedule.links":
		msg = fmt.Sprintf("Schedule %s found %s, %d new", sched, Plural(num(d, "count"), "link"), num(d, "new"))
	case "schedule.finished":
		if str(d, "status") == "error" {
			level = "error"
			msg = fmt.Sprintf("Schedule %s failed: %s", sched, str(d, "error"))
		} else {
			msg = fmt.Sprintf("Schedule %s finished and added %s", sched, Plural(num(d, "count"), "torrent"))
		}
	case "maintenance.run":
		msg = str(sub(d, "task"), "name") + ": " + str(d, "result")
	case "settings.saved":
		changed, _ := d["changed"].([]string)
		if len(changed) > 0 {
			msg = "Settings saved. Changed: " + strings.Join(changed, ", ")
		} else {
			msg = "Settings saved"
		}
	case "update.available":
		level = "warn"
		msg = fmt.Sprintf("BitSwarm %s is available. You’re running %s.", str(d, "latest"), str(d, "current"))
	default:
		msg = event
	}
	return level, msg
}
