package config

import (
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		wantPort string
		wantPath string
		wantDemo bool
	}{
		{name: "defaults", wantPort: "8080", wantPath: "data/swarm.db"},
		{name: "overrides", env: map[string]string{"PORT": "9000", "DB_PATH": "/tmp/x.db", "SWARM_DEMO": "1"}, wantPort: "9000", wantPath: "/tmp/x.db", wantDemo: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"PORT", "DB_PATH", "SWARM_DEMO", "SWARM_DOWNLOADS", "SWARM_ALLOW_COMMANDS", "SWARM_UPDATE_FEED"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if cfg.Port != tc.wantPort || cfg.DBPath != tc.wantPath || cfg.Demo != tc.wantDemo {
				t.Errorf("Load() = %q, %q, %v, want %q, %q, %v", cfg.Port, cfg.DBPath, cfg.Demo, tc.wantPort, tc.wantPath, tc.wantDemo)
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		value    string
		fallback string
		want     string
	}{
		{name: "value set", key: "TEST_KEY_SET", value: "custom", fallback: "default", want: "custom"},
		{name: "value unset", key: "TEST_KEY_UNSET", value: "", fallback: "default", want: "default"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.key, tc.value)
			if got := getEnv(tc.key, tc.fallback); got != tc.want {
				t.Errorf("getEnv() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLoad_DownloadDir(t *testing.T) {
	t.Setenv("SWARM_DOWNLOADS", "/srv/dl")
	if cfg, _ := Load(); cfg.DownloadDir != "/srv/dl" {
		t.Errorf("DownloadDir = %q", cfg.DownloadDir)
	}
	t.Setenv("SWARM_DOWNLOADS", "")
	if cfg, _ := Load(); !filepath.IsAbs(cfg.DownloadDir) || filepath.Base(cfg.DownloadDir) != "downloads" {
		t.Errorf("default DownloadDir = %q", cfg.DownloadDir)
	}
}

func TestLoad_AllowCommands(t *testing.T) {
	t.Setenv("SWARM_ALLOW_COMMANDS", "")
	if cfg, _ := Load(); cfg.AllowCommands {
		t.Error("commands must be off by default")
	}
	t.Setenv("SWARM_ALLOW_COMMANDS", "1")
	if cfg, _ := Load(); !cfg.AllowCommands {
		t.Error("SWARM_ALLOW_COMMANDS=1 should turn commands on")
	}
}

func TestLoad_UpdateFeed(t *testing.T) {
	t.Setenv("SWARM_UPDATE_FEED", "https://example.org/releases.json")
	if cfg, _ := Load(); cfg.UpdateFeed != "https://example.org/releases.json" {
		t.Errorf("UpdateFeed = %q", cfg.UpdateFeed)
	}
}
