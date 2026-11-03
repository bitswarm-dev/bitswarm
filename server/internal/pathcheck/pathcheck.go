package pathcheck

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Error struct {
	Path   string
	Action string
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("Bitswarm can’t %s %s: %s. Pick a folder Bitswarm has write access to.", e.Action, e.Path, e.Reason)
}

func reason(err error) string {
	var pe *fs.PathError
	switch {
	case errors.Is(err, fs.ErrPermission):
		return "permission denied"
	case errors.As(err, &pe):
		return strings.ToLower(pe.Err.Error())
	}
	return strings.ToLower(err.Error())
}

func expand(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

func Ensure(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("Enter a folder path.")
	}
	full := expand(path)
	if !filepath.IsAbs(full) {
		return errors.New("Enter a full folder path, such as /downloads/movies.")
	}
	if info, err := os.Stat(full); err == nil && !info.IsDir() {
		return &Error{Path: path, Action: "use", Reason: "it’s a file, not a folder"}
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		return &Error{Path: path, Action: "create", Reason: reason(err)}
	}
	probe, err := os.CreateTemp(full, ".swarm-write-test-*")
	if err != nil {
		return &Error{Path: path, Action: "write to", Reason: reason(err)}
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(name)
		return &Error{Path: path, Action: "write to", Reason: reason(err)}
	}
	if err := os.Remove(name); err != nil {
		return &Error{Path: path, Action: "write to", Reason: reason(err)}
	}
	return nil
}
