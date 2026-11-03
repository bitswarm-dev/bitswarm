package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	Port string

	LogLevel  string
	LogFormat string

	DBPath        string
	Demo          bool
	DownloadDir   string
	AllowCommands bool
	UpdateFeed    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("PORT", "8080"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFormat:     getEnv("LOG_FORMAT", "json"),
		DBPath:        getEnv("DB_PATH", "data/swarm.db"),
		Demo:          os.Getenv("SWARM_DEMO") == "1",
		DownloadDir:   downloadDir(),
		AllowCommands: os.Getenv("SWARM_ALLOW_COMMANDS") == "1",
		UpdateFeed:    os.Getenv("SWARM_UPDATE_FEED"),
	}

	return cfg, nil
}

func downloadDir() string {
	if v := os.Getenv("SWARM_DOWNLOADS"); v != "" {
		return v
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "downloads")
	}
	return "/downloads"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
