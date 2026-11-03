package db

import (
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func Open(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("db: create dir: %w", err)
		}
		if f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600); err == nil {
			f.Close()
		}
		for _, p := range []string{path, path + "-wal", path + "-shm"} {
			_ = os.Chmod(p, 0o600)
		}
	}

	conn, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	conn.SetMaxOpenConns(1)

	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func Migrate(conn *sql.DB) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("db: read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		stmt, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("db: read %s: %w", name, err)
		}
		if _, err := conn.Exec(string(stmt)); err != nil {
			return fmt.Errorf("db: apply %s: %w", name, err)
		}
	}
	return nil
}
