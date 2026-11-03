package db

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpen(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{"memory", ":memory:"},
		{"file in new dir", filepath.Join(t.TempDir(), "nested", "swarm.db")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conn, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Open() error = %v", err)
			}
			defer conn.Close()

			for _, table := range []string{"health_checks", "docs", "kv"} {
				var n int
				err := conn.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n)
				if err != nil {
					t.Errorf("table %s missing: %v", table, err)
				}
			}
		})
	}
}

func TestOpen_BadPath(t *testing.T) {
	if _, err := Open("/dev/null/x/swarm.db"); err == nil {
		t.Error("Open() expected error for unusable path, got nil")
	}
}

func TestMigrate_Idempotent(t *testing.T) {
	conn, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := Migrate(conn); err != nil {
		t.Errorf("second Migrate() error = %v", err)
	}
}

func TestOpen_FilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits don't apply on Windows")
	}
	t.Run("new file and folder", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "data")
		path := filepath.Join(dir, "swarm.db")
		conn, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS perm_check (x INTEGER)`); err != nil {
			t.Fatal(err)
		}
		assertMode(t, dir, 0o700)
		for _, p := range []string{path, path + "-wal", path + "-shm"} {
			if _, err := os.Stat(p); err == nil {
				assertMode(t, p, 0o600)
			}
		}
		assertMode(t, path, 0o600)
	})
	t.Run("an old world-readable file is tightened", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "swarm.db")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		conn, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		assertMode(t, path, 0o600)
	})
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s has mode %o, want %o", filepath.Base(path), got, want)
	}
}
