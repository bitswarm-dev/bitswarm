package redact

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestURL(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"no query", "https://example.com/a/b", "https://example.com/a/b"},
		{"query removed", "https://example.com/search?apikey=SECRET&q=x", "https://example.com/search?…"},
		{"password removed", "https://user:pass@example.com/x", "https://example.com/x"},
		{"fragment removed", "https://example.com/x#token=abc", "https://example.com/x"},
		{"unparseable keeps no query", "http://exa mple.com/x?key=SECRET", "http://exa mple.com/x?…"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := URL(tt.in)
			if got != tt.want {
				t.Errorf("URL(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if strings.Contains(got, "SECRET") || strings.Contains(got, "pass@") {
				t.Errorf("URL(%q) leaked a secret: %q", tt.in, got)
			}
		})
	}
}

func TestErr(t *testing.T) {
	raw := "https://api.example.com/v1/send?key=SECRET"
	wrapped := &url.Error{Op: "Post", URL: raw, Err: errors.New("dial tcp: lookup api.example.com: no such host")}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"plain error", errors.New("boom"), "boom"},
		{"url error", wrapped, `Post "https://api.example.com/v1/send?…": dial tcp: lookup api.example.com: no such host`},
		{"wrapped url error", errors.Join(errors.New("fetch failed"), wrapped), "fetch failed\n" + `Post "https://api.example.com/v1/send?…": dial tcp: lookup api.example.com: no such host`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Err(tt.err)
			if got != tt.want {
				t.Errorf("Err() = %q, want %q", got, tt.want)
			}
			if strings.Contains(got, "SECRET") {
				t.Errorf("Err() leaked a secret: %q", got)
			}
		})
	}
}
