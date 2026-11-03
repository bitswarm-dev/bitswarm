package hooks

import (
	"strings"
	"testing"
)

func TestParseCurl(t *testing.T) {
	cloudflare := "curl -X POST \\\n  \"https://api.cloudflare.com/client/v4/accounts/xxxxx/email/sending/send\" \\\n  -H \"Authorization: Bearer xxxx\" \\\n  -H \"Content-Type: application/json\" \\\n  -d '{\n    \"to\": \"recipient@example.com\",\n    \"from\": \"welcome@alerts.example.com\",\n    \"subject\": \"Welcome to our service!\",\n    \"html\": \"<h1>Welcome!</h1><p>Thanks for signing up.</p>\",\n    \"text\": \"Welcome! Thanks for signing up.\"\n  }'"
	chrome := "curl 'https://example.com/api' \\\n  -H 'accept: application/json, text/plain, */*' \\\n  -H 'content-type: application/json' \\\n  -H 'accept-encoding: gzip, deflate, br' \\\n  --data-raw $'{\"name\":\"it\\'s\",\"n\":1,\"e\":\"\\u00e9\\n\"}' \\\n  --compressed"
	tests := []struct {
		name     string
		in       string
		method   string
		url      string
		headers  string
		body     string
		warnings []string
	}{
		{
			name: "a multi-line command with continuations and a multi-line body", in: cloudflare,
			method: "POST", url: "https://api.cloudflare.com/client/v4/accounts/xxxxx/email/sending/send",
			headers:  "Authorization: Bearer xxxx\nContent-Type: application/json",
			body:     "{\n    \"to\": \"recipient@example.com\",\n    \"from\": \"welcome@alerts.example.com\",\n    \"subject\": \"Welcome to our service!\",\n    \"html\": \"<h1>Welcome!</h1><p>Thanks for signing up.</p>\",\n    \"text\": \"Welcome! Thanks for signing up.\"\n  }",
			warnings: []string{"key in plain text (Authorization header)"},
		},
		{
			name: "copy as cURL from a browser", in: chrome,
			method: "POST", url: "https://example.com/api",
			headers: "accept: application/json, text/plain, */*\ncontent-type: application/json",
			body:    "{\"name\":\"it's\",\"n\":1,\"e\":\"é\n\"}",
		},
		{
			name: "a simple one-liner", in: `curl -X POST https://api.example.com/send -H 'Content-Type: application/json' -d '{"a":"{{torrent.name}}"}'`,
			method: "POST", url: "https://api.example.com/send", headers: "Content-Type: application/json", body: `{"a":"{{torrent.name}}"}`,
		},
		{
			name: "double quotes with escapes", in: `curl https://x.test/h -H "Content-Type: application/json" -d "{\"a\":1,\"b\":\"say \\\"hi\\\"\"}"`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/json", body: `{"a":1,"b":"say \"hi\""}`,
		},
		{
			name: "the secret reference is not flagged", in: `curl https://x.test/h -H 'Authorization: Bearer {{secret.MAIL_KEY}}' -d 'a=1'`,
			method: "POST", url: "https://x.test/h", headers: "Authorization: Bearer {{secret.MAIL_KEY}}\nContent-Type: application/x-www-form-urlencoded", body: "a=1",
		},
		{
			name: "json option", in: `curl --json '{"a":1}' https://x.test/h`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/json\nAccept: application/json", body: `{"a":1}`,
		},
		{
			name: "basic auth", in: `curl -u user:pass -d x=1 https://x.test/h`,
			method: "POST", url: "https://x.test/h", headers: "Authorization: Basic dXNlcjpwYXNz\nContent-Type: application/x-www-form-urlencoded", body: "x=1",
			warnings: []string{"Authorization header"},
		},
		{
			name: "get with data becomes a query", in: `curl -G -d 'q=a b' -d 'n=2' https://x.test/s`,
			method: "GET", url: "https://x.test/s?q=a%20b&n=2",
		},
		{
			name: "put with the method after the address", in: `curl https://x.test/h -XPUT -d 'a=1'`,
			method: "PUT", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "a=1",
		},
		{
			name: "combined short flags", in: `curl -sSL -X PATCH https://x.test/h -d 'a=1'`,
			method: "PATCH", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "a=1",
		},
		{
			name: "options written with an equals sign", in: `curl --request=PATCH --header='X-A: b' --data='x' https://x.test/h`,
			method: "PATCH", url: "https://x.test/h", headers: "X-A: b\nContent-Type: application/x-www-form-urlencoded", body: "x",
		},
		{
			name: "a prompt marker in front", in: `$ curl https://x.test/ping`,
			method: "GET", url: "https://x.test/ping",
		},
		{
			name: "the address comes from --url", in: `curl --url https://x.test/ping -H 'X-A: 1'`,
			method: "GET", url: "https://x.test/ping", headers: "X-A: 1",
		},
		{
			name: "no scheme", in: `curl x.test/ping`,
			method: "GET", url: "https://x.test/ping", warnings: []string{"https:// was added"},
		},
		{
			name: "a key in the address", in: `curl 'https://x.test/s?apikey=abc123&q=1'`,
			method: "GET", url: "https://x.test/s?apikey=abc123&q=1", warnings: []string{"“apikey” in the address"},
		},
		{
			name: "dropped headers", in: `curl https://x.test/h -H 'User-Agent: x' -H 'Host: y' -H 'Content-Length: 3' -H 'X-Keep: 1'`,
			method: "GET", url: "https://x.test/h", headers: "X-Keep: 1", warnings: []string{"Skipped the User-Agent", "Skipped the Host", "Skipped the Content-Length"},
		},
		{
			name: "multiple data options are joined", in: `curl https://x.test/h -d a=1 -d b=2`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "a=1&b=2",
		},
		{
			name: "a url-encoded value", in: `curl https://x.test/h --data-urlencode 'q=a b&c' --data-urlencode 'plain text'`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "q=a%20b%26c&plain%20text",
		},
		{
			name: "a file reference is skipped, not read", in: `curl https://x.test/h -d @/etc/passwd`,
			method: "GET", url: "https://x.test/h", warnings: []string{"data that comes from a file"},
		},
		{
			name: "raw data that starts with an at sign is kept", in: `curl https://x.test/h --data-raw '@handle'`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "@handle",
		},
		{
			name: "substitutions are kept as text", in: `curl https://x.test/h -d "$(cat /etc/passwd)" -H "X-T: $TOKEN"`,
			method: "POST", url: "https://x.test/h", headers: "X-T: $TOKEN\nContent-Type: application/x-www-form-urlencoded", body: "$(cat /etc/passwd)",
			warnings: []string{"aren’t expanded"},
		},
		{
			name: "a pipe ends the command", in: `curl https://x.test/h -d a=1 | jq .`,
			method: "POST", url: "https://x.test/h", headers: "Content-Type: application/x-www-form-urlencoded", body: "a=1", warnings: []string{"Everything from “|” on was ignored"},
		},
		{
			name: "a second command is ignored", in: "curl https://x.test/a\ncurl https://x.test/b",
			method: "GET", url: "https://x.test/a", warnings: []string{"Only the first command was read"},
		},
		{
			name: "a get with a body", in: `curl -X GET https://x.test/h -d a=1`,
			method: "GET", url: "https://x.test/h", warnings: []string{"GET requests in Bitswarm don’t send a body"},
		},
		{
			name: "a header with a line break is skipped", in: "curl https://x.test/h -H $'X-A: b\\nX-B: c' -H 'X-Ok: 1'",
			method: "GET", url: "https://x.test/h", headers: "X-Ok: 1", warnings: []string{"line break"},
		},
		{
			name: "options that don't apply", in: `curl -k -o out.txt -x http://proxy:3128 https://x.test/h`,
			method: "GET", url: "https://x.test/h", warnings: []string{"skip certificate checks", "--output", "--proxy"},
		},
		{
			name: "a comment after the command", in: "curl https://x.test/h # note",
			method: "GET", url: "https://x.test/h",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseCurl(tc.in)
			if err != nil {
				t.Fatalf("ParseCurl() error = %v", err)
			}
			if got.Method != tc.method || got.URL != tc.url || got.Headers != tc.headers || got.Body != tc.body {
				t.Errorf("ParseCurl() =\n method  %q\n url     %q\n headers %q\n body    %q\nwant\n method  %q\n url     %q\n headers %q\n body    %q", got.Method, got.URL, got.Headers, got.Body, tc.method, tc.url, tc.headers, tc.body)
			}
			joined := strings.Join(got.Warnings, "\n")
			for _, w := range tc.warnings {
				if !strings.Contains(joined, w) {
					t.Errorf("warnings %q lack %q", got.Warnings, w)
				}
			}
			if len(tc.warnings) == 0 && len(got.Warnings) > 0 {
				t.Errorf("unexpected warnings: %q", got.Warnings)
			}
		})
	}
}

func TestParseCurl_Errors(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", "starts with curl"},
		{"not curl", "wget https://x.test", "starts with curl"},
		{"no address", "curl -H 'X-A: 1'", "couldn’t find an address"},
		{"unclosed single quote", "curl 'https://x.test", "never closed"},
		{"unclosed double quote", `curl "https://x.test`, "never closed"},
		{"unclosed ansi quote", `curl $'https://x.test`, "never closed"},
		{"head request", "curl -I https://x.test", "sends HEAD"},
		{"delete request", "curl -X DELETE https://x.test", "sends DELETE"},
		{"other scheme", "curl ftp://x.test/file", "http:// and https://"},
		{"option without a value", "curl https://x.test -H", "needs a value"},
		{"long option without a value", "curl https://x.test --request", "needs a value"},
		{"too long", "curl https://x.test/" + strings.Repeat("a", maxCurlInput), "too long"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCurl(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestParseCurl_NeverRevealsKeysInMessages(t *testing.T) {
	in := `curl -H 'Authorization: Bearer SUPERSECRETKEY' -H 'X-Bad' --data-urlencode '@SUPERSECRETFILE' 'https://x.test/h?token=SUPERSECRETKEY' -X DELETE`
	_, err := ParseCurl(in)
	if err == nil || strings.Contains(err.Error(), "SUPERSECRET") {
		t.Fatalf("error = %v", err)
	}
	got, err := ParseCurl(strings.Replace(in, " -X DELETE", "", 1))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range got.Warnings {
		if strings.Contains(w, "SUPERSECRET") {
			t.Errorf("a warning shows the key: %q", w)
		}
	}
}
