package pathcheck

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsure(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a-file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	readOnly := filepath.Join(root, "ro")
	if err := os.Mkdir(readOnly, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o755) })

	tests := []struct {
		name    string
		path    string
		wantErr string
		skip    bool
	}{
		{"existing folder", root, "", false},
		{"missing folder is created", filepath.Join(root, "new", "deeper"), "", false},
		{"surrounding spaces", " " + root + " ", "", false},
		{"empty", "  ", "Enter a folder path.", false},
		{"relative", "downloads/x", "full folder path", false},
		{"a file", file, "it’s a file, not a folder", false},
		{"cannot create inside a file", filepath.Join(file, "sub"), "can’t create", false},
		{"read-only parent", filepath.Join(readOnly, "sub"), "permission denied", os.Geteuid() == 0 || runtime.GOOS == "windows"},
		{"read-only folder", readOnly, "can’t write to", os.Geteuid() == 0 || runtime.GOOS == "windows"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.skip {
				t.Skip("permissions aren’t enforced here")
			}
			err := Ensure(tc.path)
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("Ensure(%q) = %v, want %q", tc.path, err, tc.wantErr)
			}
		})
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".swarm-write-test") {
			t.Errorf("probe file left behind: %s", e.Name())
		}
	}
}

func TestEnsure_Home(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := Ensure("~/swarm-downloads"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, "swarm-downloads")); err != nil {
		t.Errorf("~ was not expanded: %v", err)
	}
}

func TestError(t *testing.T) {
	var target *Error
	err := Ensure(filepath.Join(t.TempDir(), "x", "\x00bad"))
	if !errors.As(err, &target) || !strings.Contains(err.Error(), "Pick a folder Bitswarm has write access to.") {
		t.Errorf("err = %v", err)
	}
}
