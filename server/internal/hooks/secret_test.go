package hooks

import (
	"context"
	"errors"
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

func TestFill_SecretPlaceholderBecomesMarker(t *testing.T) {
	vars := map[string]string{"torrent.name": "x"}
	tests := []struct{ tpl, mode, want string }{
		{"Bearer {{secret.API_KEY}}", "text", "Bearer {{secret.API_KEY}}"},
		{"?key={{ secret.API_KEY }}", "url", "?key={{secret.API_KEY}}"},
		{`"{{secret.API_KEY}}"`, "json", `"{{secret.API_KEY}}"`},
		{"{{secret.API_KEY}} {{torrent.name}}", "text", "{{secret.API_KEY}} x"},
	}
	for _, tc := range tests {
		got := Fill(tc.tpl, vars, tc.mode)
		if strings.Contains(got, "{{secret.") || !strings.Contains(got, tokenOpen) {
			t.Errorf("Fill(%q, %s) = %q; the author’s reference should become the private marker", tc.tpl, tc.mode, got)
		}
		if shown := displaySecrets(got); shown != tc.want {
			t.Errorf("displaySecrets(Fill(%q, %s)) = %q, want %q", tc.tpl, tc.mode, shown, tc.want)
		}
	}
	if got := Fill("echo {{secret.API_KEY}}", vars, "shell"); got != "echo {{secret.API_KEY}}" {
		t.Errorf("a shell template should keep the text as it is, got %q", got)
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
	tok := secretToken
	tests := []struct {
		name    string
		d       Delivery
		url     string
		body    string
		header  string
		missing []string
	}{
		{"url is escaped", Delivery{URL: "https://x/s?k=" + tok("KEY")}, "https://x/s?k=a%20b%26%22c%22", "", "", nil},
		{"json body is escaped", Delivery{Body: `{"k":"` + tok("KEY") + `"}`, Headers: [][2]string{{"Content-Type", "application/json"}}}, "", `{"k":"a b&\"c\""}`, "", nil},
		{"text body is raw", Delivery{Body: "k=" + tok("KEY"), Headers: [][2]string{{"Content-Type", "text/plain"}}}, "", `k=a b&"c"`, "", nil},
		{"header drops newlines", Delivery{Headers: [][2]string{{"Authorization", "Bearer " + tok("NEWLINE")}}}, "", "", "Bearer line1X-Evil: 1", nil},
		{"spaces in the template reference", Delivery{Headers: [][2]string{{"X-Api-Key", Fill("{{ secret.KEY }}", nil, "text")}}}, "", "", `a b&"c"`, nil},
		{"missing is reported once", Delivery{URL: "https://x/" + tok("NOPE") + "/" + tok("NOPE"), Body: tok("OTHER")}, "https://x/" + tok("NOPE") + "/" + tok("NOPE"), tok("OTHER"), "", []string{"NOPE", "OTHER"}},
		{"literal text is never resolved", Delivery{URL: "https://x/{{secret.KEY}}", Body: "{{secret.KEY}}", Headers: [][2]string{{"X-Name", "{{secret.KEY}}"}}}, "https://x/{{secret.KEY}}", "{{secret.KEY}}", "{{secret.KEY}}", nil},
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

func TestSecretInjection_EventDataCannotPullSecrets(t *testing.T) {
	t.Setenv("SWARM_SECRET_API_KEY", "real-key-5150")
	t.Setenv("SWARM_SECRET_OTHER", "other-secret-777")
	names := []string{"{{secret.API_KEY}}", "{{ secret.OTHER }}", "x{{secret.API_KEY}}{{secret.OTHER}}y"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			var gotURL, gotName, gotAuth, gotBody string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				gotURL, gotName, gotAuth, gotBody = r.URL.RawQuery, r.Header.Get("X-Name"), r.Header.Get("Authorization"), string(b)
			}))
			defer srv.Close()
			svc, _ := newSvc(t, false)
			svc.SetSamples(Samples{Torrent: func(context.Context) map[string]any { return torrentInfo(name, "") }})
			ctx := context.Background()
			h := apiHook(srv.URL + "/send?n={{torrent.name}}")
			h.Headers = "Content-Type: application/json\nAuthorization: Bearer {{secret.API_KEY}}\nX-Name: {{torrent.name}}"
			h.Body = `{"n":"{{torrent.name}}","p":{{payload}}}`
			v, err := svc.Create(ctx, h)
			if err != nil {
				t.Fatal(err)
			}
			d, _ := svc.Test(ctx, v.ID)
			if d.Status != "sent" {
				t.Fatalf("status = %q (%s)", d.Status, d.Error)
			}
			if gotAuth != "Bearer real-key-5150" {
				t.Errorf("the author’s own reference should resolve, got %q", gotAuth)
			}
			for where, text := range map[string]string{"query": gotURL, "X-Name header": gotName, "body": gotBody} {
				if strings.Contains(text, "real-key-5150") || strings.Contains(text, "other-secret-777") {
					t.Errorf("event data pulled a secret into the %s: %q", where, text)
				}
			}
			if !strings.Contains(gotBody, "secret.") {
				t.Errorf("the torrent name should arrive as plain text, got body %q", gotBody)
			}
		})
	}
}

func TestDelivery_ErrorsDoNotRevealASecretInThePath(t *testing.T) {
	tests := []struct {
		name  string
		value string
		leaks []string
	}{
		{"token with a colon and a slash", "123456:ABC-DEF_ghi/jkl", []string{"ABC-DEF", "jkl", "123456"}},
		{"space and plus", "ab c+d", []string{"ab c", "ab%20c", "ab+c", "c+d", "c%2Bd"}},
		{"percent and unicode", "p%41ss-é", []string{"p%41ss", "p%2541ss", "é", "%C3%A9"}},
		{"two characters", "qz", []string{"/botqz", "qz/"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SWARM_SECRET_TG_TOKEN", tc.value)
			svc, l := newSvc(t, false)
			ctx := context.Background()
			h := apiHook("http://127.0.0.1:1/bot{{secret.TG_TOKEN}}/sendMessage")
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
			for name, text := range map[string]string{"error": d.Error, "stored URL": all[0].URL, "stored text": all[0].Text, "log": logged} {
				for _, leak := range tc.leaks {
					if strings.Contains(text, leak) {
						t.Errorf("%s reveals part of the secret (%q): %q", name, leak, text)
					}
				}
			}
			if !strings.Contains(d.Error, "/bot{{secret.TG_TOKEN}}/sendMessage") {
				t.Errorf("the error should name the secret where it was written: %q", d.Error)
			}
		})
	}
}

func TestMaskDelivery_ShowsReferencesNotMarkers(t *testing.T) {
	h := Hook{Kind: "api", Method: "POST", URL: "https://x/y?k={{secret.K1}}", Headers: "Content-Type: application/json\nAuthorization: Bearer {{secret.K1}}", Body: `{"k":"{{secret.K1}}"}`}
	d := Build(h, "torrent.completed", map[string]any{}, time.Unix(1_700_000_000, 0))
	if !strings.Contains(d.URL, tokenOpen) {
		t.Fatal("test setup: Build should hold the private marker")
	}
	got := maskDelivery(d)
	for name, v := range map[string]string{"URL": got.URL, "Body": got.Body, "Text": got.Text, "Copy": got.Copy, "header": got.Headers[1][1]} {
		if strings.Contains(v, tokenOpen) || strings.Contains(v, "⟦") {
			t.Errorf("%s still has the private marker: %q", name, v)
		}
	}
	if got.URL != "https://x/y?k={{secret.K1}}" || got.Headers[1][1] != "Bearer {{secret.K1}}" || got.Body != `{"k":"{{secret.K1}}"}` {
		t.Errorf("references should show as written: url=%q header=%q body=%q", got.URL, got.Headers[1][1], got.Body)
	}
}

func TestFill_InvalidSecretNamesProduceNoMarker(t *testing.T) {
	tests := []string{
		"x {{secret.a.b}} y", "x {{secret.lower}} y", "x {{secret.Mixed_Case}} y", "x {{secret.}} y", "x {{secret.1ABC}} y", "x {{secret.A.B.C}} y",
	}
	for _, tpl := range tests {
		for _, mode := range []string{"text", "url", "json"} {
			got := Fill(tpl, nil, mode)
			if strings.Contains(got, tokenOpen) || strings.Contains(got, "⟦") {
				t.Errorf("Fill(%q, %s) = %q; an invalid secret name must not produce a marker", tpl, mode, got)
			}
		}
	}
	if got := Fill("{{secret.GOOD_NAME_2}}", nil, "text"); !strings.Contains(got, tokenOpen) {
		t.Errorf("a valid name should produce a marker, got %q", got)
	}
}

func TestValidate_SecretNameGrammar(t *testing.T) {
	svc, _ := newSvc(t, true)
	ctx := context.Background()
	tests := []struct {
		name, ref string
		ok        bool
	}{
		{"valid", "{{secret.RESEND_KEY}}", true},
		{"valid with digits", "{{secret.KEY_2}}", true},
		{"with a dot", "{{secret.A.B}}", false},
		{"lowercase", "{{secret.resend}}", false},
		{"mixed case", "{{secret.Resend}}", false},
		{"empty name", "{{secret.}}", false},
		{"starts with a digit", "{{secret.1KEY}}", false},
	}
	for _, tc := range tests {
		for place, mk := range map[string]func(string) Hook{
			"header": func(r string) Hook { h := apiHook("https://x/y"); h.Headers = "Authorization: Bearer " + r; return h },
			"url":    func(r string) Hook { return apiHook("https://x/y?k=" + r) },
			"body":   func(r string) Hook { h := apiHook("https://x/y"); h.Headers = ""; h.Body = `"` + r + `"`; return h },
		} {
			t.Run(tc.name+" in the "+place, func(t *testing.T) {
				_, err := svc.Create(ctx, mk(tc.ref))
				if tc.ok && err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if !tc.ok && (err == nil || !strings.Contains(err.Error(), "capital letters")) {
					t.Errorf("error = %v, want the secret-name message", err)
				}
			})
		}
	}
}

func TestOnce_RefusesToSendAnUnresolvedMarker(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer srv.Close()
	svc, _ := newSvc(t, false)
	stray := tokenOpen + "A.B" + tokenClose
	tests := []struct {
		name string
		d    Delivery
	}{
		{"in the URL", Delivery{Method: "POST", URL: srv.URL + "/" + stray}},
		{"in a header value", Delivery{Method: "POST", URL: srv.URL, Headers: [][2]string{{"X-Key", stray}}}},
		{"in a header name", Delivery{Method: "POST", URL: srv.URL, Headers: [][2]string{{"X-" + tokenOpen, "v"}}}},
		{"in the body", Delivery{Method: "POST", URL: srv.URL, Body: stray}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, err := svc.once(context.Background(), tc.d, "")
			var stop noRetry
			if err == nil || !errors.As(err, &stop) || code != 0 {
				t.Errorf("once() = %d, %v; want a refusal that isn’t retried", code, err)
			}
			if strings.Contains(err.Error(), tokenOpen) {
				t.Errorf("the refusal leaks the marker: %v", err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Errorf("the server was called %d times; nothing should have been sent", hits.Load())
	}
}
