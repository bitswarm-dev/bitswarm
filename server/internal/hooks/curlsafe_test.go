package hooks

import (
	"strings"
	"testing"
)

func TestCurlProblem(t *testing.T) {
	tests := []struct{ name, tpl, want string }{
		{"no placeholders", "curl -F file=@x https://example.org", ""},
		{"after name and equals", "curl --data-urlencode name={{torrent.name}} https://example.org", ""},
		{"inside a url", "curl 'https://example.org/h?q={{torrent.name}}'", ""},
		{"header value", "curl -H 'X-Name: {{torrent.name}}' https://example.org", ""},
		{"payload on its own", "curl -sf -X POST https://example.org --data-raw {{payload}}", ""},
		{"glued without an equals sign", "curl --data-urlencode x{{torrent.name}} https://example.org", "right after an equals sign"},
		{"colon in data", "curl --data-urlencode 'a: {{torrent.name}}' https://example.org", "right after an equals sign"},
		{"header after --header", "curl --header 'X-Name: {{torrent.name}}' https://example.org", ""},
		{"url path", "curl https://example.org/torrents/{{torrent.hash}}", ""},
		{"two in one word", "curl -d 'a={{torrent.name}}&b={{torrent.hash}}' https://example.org", ""},
		{"whole argument", "curl --data-urlencode {{torrent.name}} https://example.org", "right after an equals sign"},
		{"whole quoted argument", "curl --data-urlencode '{{torrent.name}}' https://example.org", "right after an equals sign"},
		{"whole double-quoted argument", `curl -d "{{torrent.name}}" https://example.org`, "right after an equals sign"},
		{"after a line continuation", "curl -X POST https://example.org \\\n  -d {{torrent.path}}", "right after an equals sign"},
		{"as the url", "curl {{torrent.name}}", "right after an equals sign"},
		{"form", "curl -F a={{torrent.name}} https://example.org", "can’t use -F"},
		{"long form", "curl --form a={{torrent.name}} https://example.org", "can’t use -F"},
		{"form-string is fine", "curl --form-string a={{torrent.name}} https://example.org", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := curlProblem(tc.tpl)
			switch {
			case tc.want == "" && got != "":
				t.Errorf("unexpected problem: %s", got)
			case tc.want != "" && !strings.Contains(got, tc.want):
				t.Errorf("problem = %q, want it to mention %q", got, tc.want)
			}
		})
	}
}
