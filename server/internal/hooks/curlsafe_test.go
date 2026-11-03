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
		{"whole argument", "curl --data-urlencode {{torrent.name}} https://example.org", "after a name and an equals sign"},
		{"whole quoted argument", "curl --data-urlencode '{{torrent.name}}' https://example.org", "after a name and an equals sign"},
		{"whole double-quoted argument", `curl -d "{{torrent.name}}" https://example.org`, "after a name and an equals sign"},
		{"after a line continuation", "curl -X POST https://example.org \\\n  -d {{torrent.path}}", "after a name and an equals sign"},
		{"as the url", "curl {{torrent.name}}", "after a name and an equals sign"},
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
