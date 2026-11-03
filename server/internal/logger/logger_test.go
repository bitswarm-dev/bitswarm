package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Level
	}{
		{name: "debug", in: "debug", want: LevelDebug},
		{name: "warn", in: "warn", want: LevelWarn},
		{name: "warning alias", in: "warning", want: LevelWarn},
		{name: "error", in: "error", want: LevelError},
		{name: "mixed case", in: "ERROR", want: LevelError},
		{name: "unknown defaults to info", in: "nope", want: LevelInfo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseLevel(tc.in); got != tc.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Level: LevelWarn, Format: "console", Output: &buf})

	log.Info("should be dropped")
	log.Warn("should appear")

	out := buf.String()
	if strings.Contains(out, "should be dropped") {
		t.Errorf("expected Info to be filtered out at Warn level, got: %q", out)
	}
	if !strings.Contains(out, "should appear") {
		t.Errorf("expected Warn message in output, got: %q", out)
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Level: LevelInfo, Format: "console", Output: &buf})

	child := log.With("request_id", "abc123")
	child.Info("handled request", "status", 200)

	out := buf.String()
	if !strings.Contains(out, "request_id=abc123") {
		t.Errorf("expected bound field in output, got: %q", out)
	}
	if !strings.Contains(out, "status=200") {
		t.Errorf("expected call-site field in output, got: %q", out)
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Level: LevelInfo, Format: "json", Output: &buf})

	log.Info("hello", "key", "value")

	out := buf.String()
	if !strings.Contains(out, `"msg":"hello"`) {
		t.Errorf("expected JSON msg field, got: %q", out)
	}
	if !strings.Contains(out, `"key":"value"`) {
		t.Errorf("expected JSON key field, got: %q", out)
	}
}

func TestLogger_ConsoleFormatIsDefault(t *testing.T) {
	var buf bytes.Buffer
	log := New(Options{Level: LevelInfo, Format: "not-a-real-format", Output: &buf})

	log.Info("hello")

	out := buf.String()
	if strings.Contains(out, "{") {
		t.Errorf("expected console (non-JSON) output for unrecognized format, got: %q", out)
	}
	if !strings.Contains(out, "INFO") {
		t.Errorf("expected level in console output, got: %q", out)
	}
}
