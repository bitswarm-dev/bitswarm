package torrents

import (
	"path/filepath"
	"testing"
)

func TestDiskFree(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name string
		dir  string
	}{
		{"existing folder", dir},
		{"folder that doesn't exist yet", filepath.Join(dir, "a", "b")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DiskFree(tc.dir); got <= 0 {
				t.Errorf("DiskFree(%q) = %v, want more than 0", tc.dir, got)
			}
		})
	}
}
