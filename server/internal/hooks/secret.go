package hooks

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
)

const (
	secretEnvPrefix = "SWARM_SECRET_"
	secretMask      = "********"
)

var secretNonce = newNonce()

var (
	tokenOpen      = "⟦S" + secretNonce + ":"
	tokenClose     = "⟧"
	tokenRe        = regexp.MustCompile(regexp.QuoteMeta(tokenOpen) + `([A-Za-z0-9_]+)` + regexp.QuoteMeta(tokenClose))
	secretRe       = regexp.MustCompile(`\{\{\s*secret\.([A-Za-z0-9_]+)\s*\}\}`)
	secretNameRe   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)
	secretHeaderRe = regexp.MustCompile(`(?i)^(authorization|proxy-authorization|x-api-key|api-key|x-auth-token|cookie|set-cookie)$`)
	schemeRe       = regexp.MustCompile(`^(Bearer|Basic|Token|Digest|Bot)\s+`)
)

func newNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("hooks: no random source for the secret marker: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func secretToken(name string) string { return tokenOpen + name + tokenClose }

func displaySecrets(s string) string { return tokenRe.ReplaceAllString(s, "{{secret.$1}}") }

type missingSecretError struct{ name string }

func (e missingSecretError) Error() string {
	return "the secret " + e.name + " isn’t set on the server. Set " + secretEnvPrefix + e.name + " (or " + secretEnvPrefix + e.name + "_FILE) and restart Bitswarm."
}

func IsSecretHeader(name string) bool { return secretHeaderRe.MatchString(strings.TrimSpace(name)) }

func HasSecretRef(s string) bool { return secretRe.MatchString(s) }

func secretEnv(name string) (string, bool) {
	if v := os.Getenv(secretEnvPrefix + name); v != "" {
		return v, true
	}
	if p := os.Getenv(secretEnvPrefix + name + "_FILE"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			if v := strings.TrimRight(string(b), "\r\n"); v != "" {
				return v, true
			}
		}
	}
	return "", false
}

func secretNames(h Hook) []string {
	var names []string
	for _, m := range secretRe.FindAllStringSubmatch(h.URL+"\n"+h.Headers+"\n"+h.Body, -1) {
		if !slices.Contains(names, m[1]) {
			names = append(names, m[1])
		}
	}
	return names
}

func MissingSecrets(h Hook) []string {
	missing := []string{}
	for _, n := range secretNames(h) {
		if _, ok := secretEnv(n); !ok {
			missing = append(missing, n)
		}
	}
	return missing
}

func resolveSecrets(d Delivery) (Delivery, []string) {
	var missing []string
	isJSON := false
	for _, kv := range d.Headers {
		if strings.EqualFold(kv[0], "content-type") && jsonTypeRe.MatchString(kv[1]) {
			isJSON = true
		}
	}
	replace := func(mode string) func(string) string {
		return func(m string) string {
			name := tokenRe.FindStringSubmatch(m)[1]
			v, ok := secretEnv(name)
			if !ok {
				if !slices.Contains(missing, name) {
					missing = append(missing, name)
				}
				return m
			}
			switch mode {
			case "url":
				return strings.ReplaceAll(url.QueryEscape(v), "+", "%20")
			case "json":
				return jsonString(v)
			case "header":
				return strings.NewReplacer("\r", "", "\n", "").Replace(v)
			}
			return v
		}
	}
	out := d
	out.URL = tokenRe.ReplaceAllStringFunc(d.URL, replace("url"))
	bodyMode := "text"
	if isJSON {
		bodyMode = "json"
	}
	out.Body = tokenRe.ReplaceAllStringFunc(d.Body, replace(bodyMode))
	out.Headers = make([][2]string, len(d.Headers))
	for i, kv := range d.Headers {
		out.Headers[i] = [2]string{kv[0], tokenRe.ReplaceAllStringFunc(kv[1], replace("header"))}
	}
	return out, missing
}

func scrubSecrets(msg string, sent Delivery) string {
	text := sent.URL + "\n" + sent.Body
	for _, kv := range sent.Headers {
		text += "\n" + kv[1]
	}
	seen := map[string]bool{}
	for _, m := range tokenRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		v, ok := secretEnv(name)
		if !ok || len(v) < 3 {
			continue
		}
		ref := "{{secret." + name + "}}"
		msg = strings.ReplaceAll(msg, v, ref)
		msg = strings.ReplaceAll(msg, url.QueryEscape(v), ref)
		msg = strings.ReplaceAll(msg, url.PathEscape(v), ref)
	}
	return msg
}

func maskValue(v string) string {
	switch {
	case v == "", secretRe.MatchString(v), strings.HasSuffix(v, secretMask):
		return v
	}
	if m := schemeRe.FindString(v); m != "" {
		return m + secretMask
	}
	return secretMask
}

func maskHeaderText(txt string) string {
	lines := strings.Split(txt, "\n")
	for i, l := range lines {
		if j := strings.Index(l, ":"); j > 0 && IsSecretHeader(l[:j]) {
			lines[i] = l[:j+1] + " " + maskValue(strings.TrimSpace(l[j+1:]))
		}
	}
	return strings.Join(lines, "\n")
}

func restoreHeaders(newTxt, oldTxt string) string {
	old := map[string]string{}
	for _, l := range strings.Split(oldTxt, "\n") {
		if j := strings.Index(l, ":"); j > 0 {
			old[strings.ToLower(strings.TrimSpace(l[:j]))] = strings.TrimSpace(l[j+1:])
		}
	}
	lines := strings.Split(newTxt, "\n")
	for i, l := range lines {
		j := strings.Index(l, ":")
		if j <= 0 || !IsSecretHeader(l[:j]) {
			continue
		}
		if v := strings.TrimSpace(l[j+1:]); strings.HasSuffix(v, secretMask) {
			if prev, ok := old[strings.ToLower(strings.TrimSpace(l[:j]))]; ok {
				lines[i] = l[:j+1] + " " + prev
			}
		}
	}
	return strings.Join(lines, "\n")
}

func maskDelivery(d Delivery) Delivery {
	if d.Kind == "cmd" {
		return d
	}
	d.URL, d.Body = displaySecrets(d.URL), displaySecrets(d.Body)
	hs := make([][2]string, len(d.Headers))
	for i, kv := range d.Headers {
		kv[1] = displaySecrets(kv[1])
		if IsSecretHeader(kv[0]) {
			kv[1] = maskValue(kv[1])
		}
		hs[i] = kv
	}
	d.Headers = hs
	d.Text, d.Copy = reqText(d), curlOf(d)
	return d
}
