package hooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFill_SecretPlaceholderStaysLiteral(t *testing.T) {
	vars := map[string]string{"torrent.name": "x"}
	tests := []struct{ tpl, mode, want string }{
		{"Bearer {{secret.API_KEY}}", "text", "Bearer {{secret.API_KEY}}"},
		{"?key={{ secret.API_KEY }}", "url", "?key={{ secret.API_KEY }}"},
		{`"{{secret.API_KEY}}"`, "json", `"{{secret.API_KEY}}"`},
		{"{{secret.API_KEY}} {{torrent.name}}", "text", "{{secret.API_KEY}} x"},
	}
	for _, tc := range tests {
		if got := Fill(tc.tpl, vars, tc.mode); got != tc.want {
			t.Errorf("Fill(%q, %s) = %q, want %q", tc.tpl, tc.mode, got, tc.want)
		}
	}
}

func TestSecretEnv(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "key")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty")
	if err := os.WriteFile(empty, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		env  map[string]string
		want string
		ok   bool
	}{
		{"plain variable", map[string]string{"SWARM_SECRET_K": "plain"}, "plain", true},
		{"file variable", map[string]string{"SWARM_SECRET_K_FILE": file}, "from-file", true},
		{"plain wins over file", map[string]string{"SWARM_SECRET_K": "plain", "SWARM_SECRET_K_FILE": file}, "plain", true},
		{"missing file", map[string]string{"SWARM_SECRET_K_FILE": filepath.Join(dir, "nope")}, "", false},
		{"empty file", map[string]string{"SWARM_SECRET_K_FILE": empty}, "", false},
		{"nothing set", nil, "", false},
		{"other prefix is ignored", map[string]string{"K": "no", "SWARM_K": "no"}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"SWARM_SECRET_K", "SWARM_SECRET_K_FILE", "K", "SWARM_K"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, ok := secretEnv("K")
			if got != tc.want || ok != tc.ok {
				t.Errorf("secretEnv(K) = %q, %v; want %q, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestResolveSecrets(t *testing.T) {
	t.Setenv("SWARM_SECRET_KEY", `a b&"c"`)
	t.Setenv("SWARM_SECRET_NEWLINE", "line1\r\nX-Evil: 1")
	tests := []struct {
		name    string
		d       Delivery
		url     string
		body    string
		header  string
		missing []string
	}{
		{"url is escaped", Delivery{URL: "https://x/s?k={{secret.KEY}}"}, "https://x/s?k=a%20b%26%22c%22", "", "", nil},
		{"json body is escaped", Delivery{Body: `{"k":"{{secret.KEY}}"}`, Headers: [][2]string{{"Content-Type", "application/json"}}}, "", `{"k":"a b&\"c\""}`, "", nil},
		{"text body is raw", Delivery{Body: "k={{secret.KEY}}", Headers: [][2]string{{"Content-Type", "text/plain"}}}, "", `k=a b&"c"`, "", nil},
		{"header drops newlines", Delivery{Headers: [][2]string{{"Authorization", "Bearer {{secret.NEWLINE}}"}}}, "", "", "Bearer line1X-Evil: 1", nil},
		{"spaces in the placeholder", Delivery{Headers: [][2]string{{"X-Api-Key", "{{ secret.KEY }}"}}}, "", "", `a b&"c"`, nil},
		{"missing is reported once", Delivery{URL: "https://x/{{secret.NOPE}}/{{secret.NOPE}}", Body: "{{secret.OTHER}}"}, "https://x/{{secret.NOPE}}/{{secret.NOPE}}", "{{secret.OTHER}}", "", []string{"NOPE", "OTHER"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			orig := tc.d
			got, missing := resolveSecrets(tc.d)
			if tc.url != "" && got.URL != tc.url {
				t.Errorf("URL = %q, want %q", got.URL, tc.url)
			}
			if tc.body != "" && got.Body != tc.body {
				t.Errorf("Body = %q, want %q", got.Body, tc.body)
			}
			if tc.header != "" && got.Headers[0][1] != tc.header {
				t.Errorf("header = %q, want %q", got.Headers[0][1], tc.header)
			}
			if strings.Join(missing, ",") != strings.Join(tc.missing, ",") {
				t.Errorf("missing = %v, want %v", missing, tc.missing)
			}
			if len(orig.Headers) > 0 && strings.Contains(orig.Headers[0][1], "line1") {
				t.Errorf("the original delivery was modified: %v", orig.Headers)
			}
		})
	}
}

func TestMaskValue(t *testing.T) {
	tests := map[string]string{
		"":                           "",
		"Bearer abc123":              "Bearer ********",
		"Basic dXNlcjpwYXNz":         "Basic ********",
		"abc123":                     "********",
		"Bearer ********":            "Bearer ********",
		"********":                   "********",
		"Bearer {{secret.API_KEY}}":  "Bearer {{secret.API_KEY}}",
		"{{ secret.API_KEY }}":       "{{ secret.API_KEY }}",
		"sk_live_abc Bearer in text": "********",
	}
	for in, want := range tests {
		if got := maskValue(in); got != want {
			t.Errorf("maskValue(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsSecretHeader(t *testing.T) {
	tests := map[string]bool{
		"Authorization": true, "authorization": true, " X-Api-Key ": true, "Api-Key": true, "X-Auth-Token": true,
		"Proxy-Authorization": true, "Cookie": true, "Set-Cookie": true,
		"Content-Type": false, "User-Agent": false, "X-Swarm-Event": false, "X-Swarm-Signature": false, "Authorization-Info": false,
	}
	for in, want := range tests {
		if got := IsSecretHeader(in); got != want {
			t.Errorf("IsSecretHeader(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMaskAndRestoreHeaders(t *testing.T) {
	real := "Content-Type: application/json\nAuthorization: Bearer REALKEY\nX-Api-Key: other-secret\nX-Note: hello"
	masked := maskHeaderText(real)
	for _, leak := range []string{"REALKEY", "other-secret"} {
		if strings.Contains(masked, leak) {
			t.Errorf("masked text still has %q: %q", leak, masked)
		}
	}
	for _, keep := range []string{"Content-Type: application/json", "X-Note: hello", "Authorization: Bearer ********", "X-Api-Key: ********"} {
		if !strings.Contains(masked, keep) {
			t.Errorf("masked text lost %q: %q", keep, masked)
		}
	}
	tests := []struct {
		name, edited, want string
	}{
		{"untouched", masked, real},
		{"new value replaces", strings.Replace(masked, "Bearer ********", "Bearer NEWKEY", 1), strings.Replace(real, "REALKEY", "NEWKEY", 1)},
		{"line removed stays removed", strings.Replace(masked, "X-Api-Key: ********\n", "", 1), "Content-Type: application/json\nAuthorization: Bearer REALKEY\nX-Note: hello"},
		{"secret reference is kept", "Authorization: Bearer {{secret.API_KEY}}", "Authorization: Bearer {{secret.API_KEY}}"},
		{"mask with no old value stays", "Cookie: ********", "Cookie: ********"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := restoreHeaders(tc.edited, real); got != tc.want {
				t.Errorf("restoreHeaders() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMaskDelivery(t *testing.T) {
	d := Build(Hook{Kind: "api", Method: "POST", URL: "https://x/y", Headers: "Content-Type: application/json\nAuthorization: Bearer REALKEY", Body: `{"a":1}`}, "torrent.completed", map[string]any{}, time.Unix(1_700_000_000, 0))
	if !strings.Contains(d.Text, "REALKEY") || !strings.Contains(d.Copy, "REALKEY") {
		t.Fatal("test setup: the unmasked delivery should hold the key")
	}
	got := maskDelivery(d)
	for name, v := range map[string]string{"Text": got.Text, "Copy": got.Copy, "Headers": got.Headers[1][1]} {
		if strings.Contains(v, "REALKEY") {
			t.Errorf("%s still has the key: %q", name, v)
		}
	}
	if !strings.Contains(got.Text, "Authorization: Bearer ********") || !strings.Contains(got.Text, "Content-Type: application/json") {
		t.Errorf("Text = %q", got.Text)
	}
	if d.Headers[1][1] != "Bearer REALKEY" {
		t.Errorf("the original was modified: %v", d.Headers)
	}
	cmd := Delivery{Kind: "cmd", Command: "echo 1", Text: "$ echo 1"}
	if out := maskDelivery(cmd); out.Text != cmd.Text || out.Command != cmd.Command {
		t.Errorf("a command delivery changed: %+v", out)
	}
}

func TestDelivery_SecretsReachTheServerButAreNotStored(t *testing.T) {
	t.Setenv("SWARM_SECRET_API_KEY", "env-key-123")
	var gotAuth, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotAuth, gotQuery, gotBody = r.Header.Get("Authorization"), r.URL.RawQuery, string(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	h := Hook{Kind: "api", Name: "Mail", Method: "POST", URL: srv.URL + "/send?key={{secret.API_KEY}}", Headers: "Content-Type: application/json\nAuthorization: Bearer {{secret.API_KEY}}\nX-Api-Key: typed-directly-456", Body: `{"token":"{{secret.API_KEY}}","n":"{{torrent.name}}"}`, Events: []string{"torrent.completed"}, Retries: 0, Timeout: 30, Enabled: true}
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	d, err := svc.Test(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.Status != "sent" {
		t.Fatalf("status = %q (%s)", d.Status, d.Error)
	}
	if gotAuth != "Bearer env-key-123" || gotQuery != "key=env-key-123" || !strings.Contains(gotBody, `"token":"env-key-123"`) {
		t.Errorf("the server got auth=%q query=%q body=%q", gotAuth, gotQuery, gotBody)
	}
	all, _ := svc.Deliveries(ctx)
	if len(all) != 1 {
		t.Fatalf("deliveries = %d", len(all))
	}
	for name, v := range map[string]string{"URL": all[0].URL, "Body": all[0].Body, "Text": all[0].Text, "Copy": all[0].Copy, "returned": d.Text + d.Copy + d.URL + d.Body} {
		for _, leak := range []string{"env-key-123", "typed-directly-456"} {
			if strings.Contains(v, leak) {
				t.Errorf("%s has %q: %q", name, leak, v)
			}
		}
	}
	if !strings.Contains(all[0].Text, "Authorization: Bearer {{secret.API_KEY}}") || !strings.Contains(all[0].Text, "X-Api-Key: ********") {
		t.Errorf("stored text = %q", all[0].Text)
	}
	hooks, _ := svc.List(ctx)
	if strings.Contains(hooks[0].Headers, "typed-directly-456") || !strings.Contains(hooks[0].Headers, "X-Api-Key: ********") {
		t.Errorf("the listed headers = %q", hooks[0].Headers)
	}
}

func TestDelivery_MissingSecretFailsWithoutRetrying(t *testing.T) {
	t.Setenv("SWARM_SECRET_NOT_SET", "")
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	svc, _ := newSvc(t, false)
	var slept atomic.Int32
	svc.sleep = func(time.Duration) { slept.Add(1) }
	ctx := context.Background()
	h := apiHook(srv.URL)
	h.Headers = "Content-Type: application/json\nAuthorization: Bearer {{secret.NOT_SET}}"
	h.Retries = 3
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := svc.Test(ctx, v.ID)
	if d.Status != "failed" || !strings.Contains(d.Error, "SWARM_SECRET_NOT_SET") || d.Attempts != 1 {
		t.Errorf("delivery = status %q error %q attempts %d", d.Status, d.Error, d.Attempts)
	}
	if hits.Load() != 0 || slept.Load() != 0 {
		t.Errorf("hits = %d, sleeps = %d; a missing secret must not send or retry", hits.Load(), slept.Load())
	}
}

func TestDelivery_SignatureCoversTheSentBody(t *testing.T) {
	t.Setenv("SWARM_SECRET_TOKEN", "tok-789")
	var body, sig string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, sig = string(b), r.Header.Get("X-Swarm-Signature")
	}))
	defer srv.Close()
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	h := apiHook(srv.URL)
	h.Body, h.Secret = `{"t":"{{secret.TOKEN}}"}`, "signing-secret"
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := svc.Test(ctx, v.ID); d.Status != "sent" {
		t.Fatalf("status = %q (%s)", d.Status, d.Error)
	}
	if body != `{"t":"tok-789"}` || sig != Sign("signing-secret", body) {
		t.Errorf("body = %q, signature = %q, want %q", body, sig, Sign("signing-secret", body))
	}
}

func TestDelivery_ErrorsDoNotRevealTheQuery(t *testing.T) {
	t.Setenv("SWARM_SECRET_KEY", "query-secret-321")
	svc, l := newSvc(t, false)
	ctx := context.Background()
	h := apiHook("http://127.0.0.1:1/send?key={{secret.KEY}}&plain=visible")
	h.Retries = 0
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := svc.Test(ctx, v.ID)
	if d.Status != "failed" {
		t.Fatalf("status = %q", d.Status)
	}
	all, _ := svc.Deliveries(ctx)
	logged := strings.Join(l.lines, "\n")
	for name, text := range map[string]string{"error": d.Error, "log": logged} {
		if strings.Contains(text, "query-secret-321") || strings.Contains(text, "plain=visible") || !strings.Contains(text, "127.0.0.1:1/send") {
			t.Errorf("%s = %q; it should name the address but not the query", name, text)
		}
	}
	for name, text := range map[string]string{"stored URL": all[0].URL, "stored text": all[0].Text, "stored copy": all[0].Copy} {
		if strings.Contains(text, "query-secret-321") {
			t.Errorf("%s holds the secret value: %q", name, text)
		}
	}
}

func TestValidate_Secrets(t *testing.T) {
	svc, _ := newSvc(t, true)
	ctx := context.Background()
	tests := []struct {
		name string
		h    Hook
		want string
	}{
		{"api with a valid reference", func() Hook {
			h := apiHook("https://x/y")
			h.Headers = "Authorization: Bearer {{secret.RESEND_KEY}}"
			return h
		}(), ""},
		{"api with a lowercase name", func() Hook {
			h := apiHook("https://x/y")
			h.Headers = "Authorization: Bearer {{secret.resend_key}}"
			return h
		}(), "capital letters"},
		{"command with a reference", Hook{Kind: "cmd", Name: "C", Cmd: "curl -H 'k: {{secret.KEY}}' x", Events: []string{"torrent.completed"}, Timeout: 30, Enabled: true}, "Command hooks can’t use"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tc.h)
			switch {
			case tc.want == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestUpdate_KeepsMaskedHeaders(t *testing.T) {
	svc, _ := newSvc(t, false)
	ctx := context.Background()
	h := apiHook("https://x/y")
	h.Headers = "Content-Type: application/json\nAuthorization: Bearer REALKEY"
	v, err := svc.Create(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(v.Headers, "REALKEY") {
		t.Errorf("Create returned the key: %q", v.Headers)
	}
	edit := v.Hook
	edit.Name = "Renamed"
	if _, err := svc.Update(ctx, edit); err != nil {
		t.Fatal(err)
	}
	stored, _ := svc.hooks.Get(ctx, v.ID)
	if !strings.Contains(stored.Headers, "Authorization: Bearer REALKEY") || stored.Name != "Renamed" {
		t.Errorf("after saving the masked form the stored headers = %q", stored.Headers)
	}
	edit.Headers = "Content-Type: application/json\nAuthorization: Bearer ROTATED"
	if _, err := svc.Update(ctx, edit); err != nil {
		t.Fatal(err)
	}
	stored, _ = svc.hooks.Get(ctx, v.ID)
	if !strings.Contains(stored.Headers, "Bearer ROTATED") || strings.Contains(stored.Headers, "REALKEY") {
		t.Errorf("after typing a new key the stored headers = %q", stored.Headers)
	}
}

func TestPreview_MasksKeysAndFlagsMissingSecrets(t *testing.T) {
	t.Setenv("SWARM_SECRET_SET_ONE", "x")
	t.Setenv("SWARM_SECRET_MISSING_ONE", "")
	svc, _ := newSvc(t, false)
	h := apiHook("https://x/y")
	h.Headers = "Content-Type: application/json\nAuthorization: Bearer typed-key\nX-Api-Key: {{secret.SET_ONE}}\nX-Other: {{secret.MISSING_ONE}}"
	p := svc.Preview(context.Background(), h)
	if strings.Contains(p.Text, "typed-key") || !strings.Contains(p.Text, "Authorization: Bearer ********") {
		t.Errorf("preview text = %q", p.Text)
	}
	if strings.Join(p.MissingSecrets, ",") != "MISSING_ONE" || len(p.Unknown) != 0 {
		t.Errorf("missing = %v, unknown = %v", p.MissingSecrets, p.Unknown)
	}
}
