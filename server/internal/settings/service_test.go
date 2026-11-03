package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"bitswarm/server/internal/db"
	"bitswarm/server/internal/docstore"
)

type recorder struct{ events []string }

func (r *recorder) Emit(ev string, _ map[string]any) { r.events = append(r.events, ev) }

type pwRecorder struct{ got string }

func (p *pwRecorder) SetPassword(_ context.Context, pw string) error { p.got = pw; return nil }

func newService(t *testing.T) (*Service, *recorder, *pwRecorder) {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	rec, pw := &recorder{}, &pwRecorder{}
	svc, err := NewService(docstore.NewKV(conn), rec, pw)
	if err != nil {
		t.Fatal(err)
	}
	return svc, rec, pw
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Settings)
		wantKey string
	}{
		{"defaults are valid", func(*Settings) {}, ""},
		{"port too high", func(s *Settings) { s.Port = 70000 }, "port"},
		{"web port equals port", func(s *Settings) { s.WebPort = s.Port }, "webPort"},
		{"negative search delay", func(s *Settings) { s.SearchDelay = -1 }, "searchDelay"},
		{"search delay too long", func(s *Settings) { s.SearchDelay = 4000 }, "searchDelay"},
		{"search delay off is valid", func(s *Settings) { s.SearchDelay = 0 }, ""},
		{"negative limit", func(s *Settings) { s.DlLimit = -1 }, "dlLimit"},
		{"bad ratio", func(s *Settings) { s.RatioOn = true; s.Ratio = 0 }, "ratio"},
		{"proxy needs host", func(s *Settings) { s.ProxyType = "socks5" }, "proxyHost"},
		{"same alt times", func(s *Settings) { s.AltSchedule = true; s.AltFrom = "08:00"; s.AltTo = "08:00" }, "altTo"},
		{"short password", func(s *Settings) { s.WebPass = "abc" }, "webPass"},
		{"empty user", func(s *Settings) { s.WebUser = " " }, "webUser"},
		{"bad option", func(s *Settings) { s.Encryption = "nope" }, "encryption"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Defaults()
			tc.mutate(&s)
			errs := Validate(s)
			if tc.wantKey == "" && len(errs) != 0 {
				t.Fatalf("unexpected errors: %v", errs)
			}
			if tc.wantKey != "" && errs[tc.wantKey] == "" {
				t.Errorf("errors = %v, want key %q", errs, tc.wantKey)
			}
		})
	}
}

func TestService_Save(t *testing.T) {
	svc, rec, pw := newService(t)
	ctx := context.Background()

	next := svc.Get()
	next.Port = 6881
	next.WebPass = "secret1"
	saved, errs := svc.Save(ctx, next)
	if len(errs) != 0 {
		t.Fatalf("Save() errors = %v", errs)
	}
	if saved.Port != 6881 || saved.WebPass != "" || pw.got != "secret1" {
		t.Errorf("saved = %+v, pw = %q", saved, pw.got)
	}
	if len(rec.events) != 1 || rec.events[0] != "settings.saved" {
		t.Errorf("events = %v", rec.events)
	}

	bad := svc.Get()
	bad.Port = 0
	if _, errs := svc.Save(ctx, bad); errs["port"] == "" {
		t.Error("expected port error")
	}
	if svc.Get().Port != 6881 {
		t.Error("invalid save changed settings")
	}
}

func TestService_Save_FolderChecks(t *testing.T) {
	svc, _, _ := newService(t)
	ctx := context.Background()
	var checked []string
	svc.SetPathCheck(func(p string) error {
		checked = append(checked, p)
		if strings.HasPrefix(p, "/locked") {
			return errors.New("Bitswarm can’t write to " + p + ": permission denied.")
		}
		return nil
	})

	next := svc.Get()
	next.UseIncomplete, next.IncompletePath = false, "/locked/incomplete"
	if _, errs := svc.Save(ctx, next); len(errs) != 0 {
		t.Fatalf("unused folders must not be checked: %v", errs)
	}
	if len(checked) != 1 || checked[0] != next.SavePath {
		t.Errorf("checked = %v", checked)
	}

	next.SavePath, next.UseIncomplete, next.MoveCompleted, next.CompletedPath, next.WatchEnabled, next.WatchPath = "/locked/dl", true, true, "/locked/done", true, "/ok/watch"
	_, errs := svc.Save(ctx, next)
	if errs["savePath"] == "" || errs["incompletePath"] == "" || errs["completedPath"] == "" || errs["watchPath"] != "" {
		t.Errorf("errs = %v", errs)
	}
	if svc.Get().SavePath == "/locked/dl" {
		t.Error("a failed folder check changed the saved settings")
	}
}

func TestDefaults_UseDownloadDir(t *testing.T) {
	defer SetDownloadDir("/downloads")
	SetDownloadDir("/srv/swarm/")
	d := Defaults()
	if d.SavePath != "/srv/swarm" || d.IncompletePath != "/srv/swarm/incomplete" || d.CompletedPath != "/srv/swarm/complete" || d.WatchPath != "/srv/swarm/watch" {
		t.Errorf("defaults = %q %q %q %q", d.SavePath, d.IncompletePath, d.CompletedPath, d.WatchPath)
	}
}
