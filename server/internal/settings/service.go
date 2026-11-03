package settings

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"bitswarm/server/internal/docstore"
	"bitswarm/server/internal/events"
	"bitswarm/server/internal/jsonutil"
)

const kvKey = "settings"

var timeRe = regexp.MustCompile(`^\d{2}:\d{2}$`)

type PasswordSetter interface {
	SetPassword(ctx context.Context, pw string) error
}

type Service struct {
	kv    *docstore.KV
	bus   events.Emitter
	pw    PasswordSetter
	check func(string) error
	mu    sync.RWMutex
	curr  Settings
}

func NewService(kv *docstore.KV, bus events.Emitter, pw PasswordSetter) (*Service, error) {
	s := &Service{kv: kv, bus: bus, pw: pw, curr: Defaults()}
	saved := Defaults()
	ok, err := kv.Load(context.Background(), kvKey, &saved)
	if err != nil {
		return nil, err
	}
	if ok {
		saved.WebPass = ""
		s.curr = saved
	}
	return s, nil
}

func (s *Service) SetPathCheck(fn func(string) error) { s.check = fn }

func (s *Service) pathErrors(d Settings) map[string]string {
	errs := map[string]string{}
	if s.check == nil {
		return errs
	}
	for _, f := range []struct {
		key  string
		on   bool
		path string
	}{
		{"savePath", true, d.SavePath}, {"incompletePath", d.UseIncomplete, d.IncompletePath},
		{"completedPath", d.MoveCompleted, d.CompletedPath}, {"watchPath", d.WatchEnabled, d.WatchPath},
	} {
		if f.on {
			if err := s.check(f.path); err != nil {
				errs[f.key] = err.Error()
			}
		}
	}
	return errs
}

func (s *Service) SetPasswords(pw PasswordSetter) {
	s.mu.Lock()
	s.pw = pw
	s.mu.Unlock()
}

func (s *Service) Get() Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.curr
}

func (s *Service) Save(ctx context.Context, next Settings) (Settings, map[string]string) {
	if errs := Validate(next); len(errs) > 0 {
		return Settings{}, errs
	}
	if errs := s.pathErrors(next); len(errs) > 0 {
		return Settings{}, errs
	}

	newPass := next.WebPass
	next.WebPass = ""
	next.ProxyPass = strings.TrimSpace(next.ProxyPass)

	s.mu.Lock()
	changed := diff(s.curr, next)
	s.curr = next
	s.mu.Unlock()

	if err := s.kv.Save(ctx, kvKey, next); err != nil {
		return Settings{}, map[string]string{"_": "Settings could not be saved: " + err.Error()}
	}
	if newPass != "" && s.pw != nil {
		if err := s.pw.SetPassword(ctx, newPass); err != nil {
			return Settings{}, map[string]string{"webPass": err.Error()}
		}
		changed = append(changed, "webPass")
	}
	s.bus.Emit("settings.saved", map[string]any{"changed": changed})
	return next, nil
}

func (s *Service) Username() string {
	return s.Get().WebUser
}

func (s *Service) LocalBypass() bool {
	return s.Get().WebLocalBypass
}

func (s *Service) Replace(ctx context.Context, next Settings) error {
	next.WebPass = ""
	s.mu.Lock()
	s.curr = next
	s.mu.Unlock()
	return s.kv.Save(ctx, kvKey, next)
}

func diff(a, b Settings) []string {
	am, bm := toMap(a), toMap(b)
	out := []string{}
	for k, v := range bm {
		if k == "webPass" {
			continue
		}
		if am[k] != v {
			out = append(out, k)
		}
	}
	sortStrings(out)
	return out
}

func toMap(s Settings) map[string]any {
	raw, _ := jsonutil.Marshal(s)
	m := map[string]any{}
	_ = jsonutilLoose(raw, &m)
	return m
}

func Validate(d Settings) map[string]string {
	e := map[string]string{}
	in := func(key string, v, min, max int, name string) {
		if v < min || v > max {
			e[key] = name + " must be a whole number from " + itoa(min) + " to " + itoa(max) + "."
		}
	}
	in("port", d.Port, 1, 65535, "Listening port")
	in("webPort", d.WebPort, 1, 65535, "Web UI port")
	in("searchDelay", d.SearchDelay, 0, 3600, "Wait between searches")
	in("speedWindow", d.SpeedWindow, 30, 3600, "Speed chart window")
	in("maxConn", d.MaxConn, 0, 99999, "Global maximum connections")
	in("maxConnPer", d.MaxConnPer, 0, 99999, "Connections per torrent")
	in("maxUpSlots", d.MaxUpSlots, 0, 9999, "Global upload slots")
	in("maxUpSlotsPer", d.MaxUpSlotsPer, 0, 9999, "Upload slots per torrent")
	in("dlLimit", d.DlLimit, 0, 10000000, "Download limit")
	in("ulLimit", d.UlLimit, 0, 10000000, "Upload limit")
	in("altDl", d.AltDl, 0, 10000000, "Alternative download limit")
	in("altUl", d.AltUl, 0, 10000000, "Alternative upload limit")
	if d.Queueing {
		in("maxActiveDl", d.MaxActiveDl, 1, 999, "Maximum active downloads")
		in("maxActiveUp", d.MaxActiveUp, 1, 999, "Maximum active uploads")
		in("maxActive", d.MaxActive, 1, 999, "Maximum active torrents")
	}
	if d.RatioOn && !(d.Ratio > 0 && d.Ratio <= 9999) {
		e["ratio"] = "Ratio must be a number greater than 0, such as 2 or 1.5."
	}
	if d.SeedTimeOn {
		in("seedTime", d.SeedTime, 1, 525600, "Seeding time")
	}
	path := func(key string, on bool, v, what string) {
		if on && strings.TrimSpace(v) == "" {
			e[key] = "Enter a folder for " + what + "."
		}
	}
	path("savePath", true, d.SavePath, "downloads")
	path("incompletePath", d.UseIncomplete, d.IncompletePath, "incomplete torrents")
	path("completedPath", d.MoveCompleted, d.CompletedPath, "completed downloads")
	path("watchPath", d.WatchEnabled, d.WatchPath, "the watched folder")
	if d.ProxyType != "none" {
		if strings.TrimSpace(d.ProxyHost) == "" {
			e["proxyHost"] = "Enter the proxy server’s address."
		}
		in("proxyPort", d.ProxyPort, 1, 65535, "Proxy port")
		if d.ProxyAuth && strings.TrimSpace(d.ProxyUser) == "" {
			e["proxyUser"] = "Enter the proxy username."
		}
	}
	if d.AltSchedule {
		if !timeRe.MatchString(d.AltFrom) {
			e["altFrom"] = "Enter a time, such as 08:00."
		}
		if !timeRe.MatchString(d.AltTo) {
			e["altTo"] = "Enter a time, such as 08:00."
		}
		if e["altFrom"] == "" && d.AltFrom == d.AltTo {
			e["altTo"] = "The end time must be different from the start time."
		}
	}
	if e["port"] == "" && e["webPort"] == "" && d.Port == d.WebPort {
		e["webPort"] = "The Web UI port must be different from the listening port."
	}
	if strings.TrimSpace(d.WebUser) == "" {
		e["webUser"] = "Enter a username for the Web UI."
	}
	if d.WebPass != "" && len(d.WebPass) < 6 {
		e["webPass"] = "Use at least 6 characters for the Web UI password."
	}
	for key, allowed := range map[string][]string{
		"proxyType": {"none", "socks5", "socks4", "http"}, "altDays": {"every", "weekdays", "weekends"},
		"encryption": {"prefer", "require", "allow", "off"}, "seedAction": {"pause", "remove", "removeData", "superseed"},
	} {
		v := map[string]string{"proxyType": d.ProxyType, "altDays": d.AltDays, "encryption": d.Encryption, "seedAction": d.SeedAction}[key]
		if !contains(allowed, v) {
			e[key] = "Choose one of the listed options."
		}
	}
	return e
}

func (s *Service) Import(ctx context.Context, partial map[string]any) ([]string, error) {
	cur := toMap(s.Get())
	for k, v := range partial {
		old, known := cur[k]
		if !known || k == "webPass" {
			continue
		}
		if fmt.Sprintf("%T", old) == fmt.Sprintf("%T", v) {
			cur[k] = v
		}
	}
	raw, err := jsonutil.Marshal(cur)
	if err != nil {
		return nil, err
	}
	cand := Defaults()
	if err := jsonutil.Unmarshal(raw, &cand); err != nil {
		return nil, err
	}
	kept := []string{}
	prev := s.Get()
	for key := range Validate(cand) {
		kept = append(kept, key)
		cur[key] = toMap(prev)[key]
	}
	if len(kept) > 0 {
		raw, _ = jsonutil.Marshal(cur)
		cand = Defaults()
		if err := jsonutil.Unmarshal(raw, &cand); err != nil {
			return nil, err
		}
	}
	sortStrings(kept)
	if err := s.Replace(ctx, cand); err != nil {
		return nil, err
	}
	changed := diff(prev, cand)
	s.bus.Emit("settings.saved", map[string]any{"changed": changed})
	return kept, nil
}
