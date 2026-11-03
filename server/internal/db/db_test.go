package db

import (
	"path/filepath"
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
