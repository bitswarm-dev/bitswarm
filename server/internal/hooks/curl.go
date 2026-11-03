package hooks

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxCurlInput = 64 << 10

type CurlRequest struct {
	Command string `json:"command"`
}

type CurlImport struct {
	Method   string   `json:"method"`
	URL      string   `json:"url"`
	Headers  string   `json:"headers"`
	Body     string   `json:"body"`
	Warnings []string `json:"warnings"`
}

const shortWithArg = "AbcCdDeEFHKmoPQrtTuUwxXyYz"

var (
	curlShortNames = map[byte]string{
		'X': "request", 'H': "header", 'd': "data", 'u': "user", 'A': "user-agent", 'b': "cookie", 'e': "referer", 'F': "form",
		'o': "output", 'm': "max-time", 'x': "proxy", 'G': "get", 'L': "location", 'k': "insecure", 'I': "head", 's': "silent",
		'S': "show-error", 'i': "include", 'v': "verbose", 'f': "fail", 'T': "upload-file", 'K': "config", 'E': "cert",
	}
	curlLongWithArg = map[string]bool{
		"request": true, "header": true, "data": true, "data-raw": true, "data-ascii": true, "data-binary": true, "data-urlencode": true,
		"json": true, "url": true, "user": true, "user-agent": true, "cookie": true, "referer": true, "form": true, "form-string": true,
		"output": true, "max-time": true, "connect-timeout": true, "proxy": true, "cacert": true, "cert": true, "key": true,
		"resolve": true, "retry": true, "limit-rate": true, "upload-file": true, "write-out": true, "config": true, "cookie-jar": true,
		"dump-header": true, "oauth2-bearer": true, "proxy-user": true, "retry-delay": true, "retry-max-time": true, "interface": true,
		"range": true, "time-cond": true, "continue-at": true, "trace": true, "trace-ascii": true, "unix-socket": true,
	}
	curlQuiet = map[string]bool{
		"silent": true, "show-error": true, "location": true, "verbose": true, "compressed": true, "fail": true, "include": true,
		"globoff": true, "no-buffer": true, "http1.0": true, "http1.1": true, "http2": true, "http3": true, "max-time": true,
		"connect-timeout": true, "retry": true, "retry-delay": true, "retry-max-time": true, "fail-with-body": true, "location-trusted": true,
		"progress-bar": true, "no-progress-meter": true, "ipv4": true, "ipv6": true, "tcp-nodelay": true, "tcp-fastopen": true,
		"keepalive": true, "no-keepalive": true, "remote-name": true, "styled-output": true, "tr-encoding": true, "path-as-is": true,
	}
	curlIgnoredHeaders = map[string]string{
		"user-agent": "Bitswarm sets the User-Agent itself", "host": "the Host comes from the address", "content-length": "Bitswarm works out the length",
		"transfer-encoding": "Bitswarm handles the transfer encoding", "connection": "Bitswarm manages the connection",
	}
	secretParamRe = regexp.MustCompile(`(?i)^(api[_-]?key|apikey|key|token|access[_-]?token|auth|secret|password|passwd|pwd|signature|sig)$`)
)

type shellWord struct {
	text string
}

func splitCurl(input string) ([]string, []string, error) {
	r := []rune(strings.ReplaceAll(input, "\r\n", "\n"))
	var words, warns []string
	var cur strings.Builder
	inWord, subst := false, false
	flush := func() {
		if inWord {
			words = append(words, cur.String())
			cur.Reset()
			inWord = false
		}
	}
	warnSubst := func() {
		if !subst {
			subst = true
			warns = append(warns, "Shell substitutions and variables such as $(…) and $NAME aren’t expanded. They are kept as plain text.")
		}
	}
	for i := 0; i < len(r); {
		c := r[i]
		switch {
		case c == '\\':
			switch {
			case i+1 >= len(r):
				cur.WriteRune('\\')
				inWord = true
				i++
			case r[i+1] == '\n':
				i += 2
			default:
				cur.WriteRune(r[i+1])
				inWord = true
				i += 2
			}
		case c == '\'':
			j := i + 1
			for j < len(r) && r[j] != '\'' {
				j++
			}
			if j >= len(r) {
				return nil, nil, Invalid("The command has a quote that is never closed. Check the single quotes.")
			}
			cur.WriteString(string(r[i+1 : j]))
			inWord = true
			i = j + 1
		case c == '"':
			k := i + 1
			closed := false
			for k < len(r) {
				ch := r[k]
				if ch == '"' {
					closed = true
					break
				}
				if ch == '\\' && k+1 < len(r) {
					switch n := r[k+1]; n {
					case '"', '\\', '$', '`':
						cur.WriteRune(n)
						k += 2
					case '\n':
						k += 2
					default:
						cur.WriteRune('\\')
						k++
					}
					continue
				}
				if ch == '$' || ch == '`' {
					warnSubst()
				}
				cur.WriteRune(ch)
				k++
			}
			if !closed {
				return nil, nil, Invalid("The command has a quote that is never closed. Check the double quotes.")
			}
			inWord = true
			i = k + 1
		case c == '$' && i+1 < len(r) && r[i+1] == '\'':
			text, next, err := ansiC(r, i+2)
			if err != nil {
				return nil, nil, err
			}
			cur.WriteString(text)
			inWord = true
			i = next
		case c == '\n':
			flush()
			if strings.TrimSpace(string(r[i+1:])) != "" {
				warns = append(warns, "Only the first command was read. The lines after it were ignored.")
			}
			return words, warns, nil
		case unicode.IsSpace(c):
			flush()
			i++
		case c == '#' && !inWord:
			for i < len(r) && r[i] != '\n' {
				i++
			}
		case c == '|' || c == ';' || c == '&' || c == '>' || c == '<':
			flush()
			if strings.TrimSpace(string(r[i:])) != "" {
				warns = append(warns, fmt.Sprintf("Everything from “%c” on was ignored. Hooks can’t pipe, chain or redirect. If that was part of the address, put the address in quotes.", c))
			}
			return words, warns, nil
		default:
			if c == '$' || c == '`' {
				warnSubst()
			}
			cur.WriteRune(c)
			inWord = true
			i++
		}
	}
	flush()
	return words, warns, nil
}

func ansiC(r []rune, i int) (string, int, error) {
	var b strings.Builder
	for i < len(r) {
		c := r[i]
		if c == '\'' {
			return b.String(), i + 1, nil
		}
		if c != '\\' || i+1 >= len(r) {
			b.WriteRune(c)
			i++
			continue
		}
		n := r[i+1]
		i += 2
		switch n {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'e', 'E':
			b.WriteByte(0x1b)
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\', '\'', '"', '?':
			b.WriteRune(n)
		case 'x', 'u', 'U':
			max := map[rune]int{'x': 2, 'u': 4, 'U': 8}[n]
			j := i
			for j < len(r) && j-i < max && isHex(r[j]) {
				j++
			}
			if j == i {
				b.WriteRune('\\')
				b.WriteRune(n)
				break
			}
			v, _ := strconv.ParseUint(string(r[i:j]), 16, 32)
			if n == 'x' {
				b.WriteByte(byte(v))
			} else if utf8.ValidRune(rune(v)) {
				b.WriteRune(rune(v))
			}
			i = j
		case '0', '1', '2', '3', '4', '5', '6', '7':
			j := i
			for j < len(r) && j-(i-1) < 3 && r[j] >= '0' && r[j] <= '7' {
				j++
			}
			v, _ := strconv.ParseUint(string(r[i-1:j]), 8, 16)
			b.WriteByte(byte(v))
			i = j
		default:
			b.WriteRune('\\')
			b.WriteRune(n)
		}
	}
	return "", 0, Invalid("The command has a $'…' quote that is never closed.")
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func ParseCurl(input string) (CurlImport, error) {
	out := CurlImport{Warnings: []string{}}
	if len(input) > maxCurlInput {
		return out, Invalid("That command is too long to import.")
	}
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "$ ") || strings.HasPrefix(input, "> ") {
		input = input[2:]
	}
	words, warns, err := splitCurl(input)
	if err != nil {
		return out, err
	}
	out.Warnings = append(out.Warnings, warns...)
	for len(words) > 0 && words[0] == "sudo" {
		words = words[1:]
	}
	if len(words) == 0 || words[0] != "curl" {
		return out, Invalid("Paste a command that starts with curl.")
	}

	var (
		method, target, user              string
		headers                           [][2]string
		data                              []string
		getMode, jsonMode, sawContentType bool
		warned                            = map[string]bool{}
		positional                        []string
	)
	warnOnce := func(msg string) {
		if !warned[msg] {
			warned[msg] = true
			out.Warnings = append(out.Warnings, msg)
		}
	}
	addHeader := func(name, value string) {
		name = strings.TrimSpace(name)
		switch {
		case !headerNameRe.MatchString(name):
			warnOnce("Skipped a header whose name isn’t valid: “" + name + "”.")
		case strings.ContainsAny(value, "\r\n"):
			warnOnce("Skipped the " + name + " header because its value has a line break.")
		case curlIgnoredHeaders[strings.ToLower(name)] != "":
			warnOnce("Skipped the " + name + " header: " + curlIgnoredHeaders[strings.ToLower(name)] + ".")
		case strings.EqualFold(name, "accept-encoding"):
		default:
			if strings.EqualFold(name, "content-type") {
				sawContentType = true
			}
			headers = append(headers, [2]string{name, value})
		}
	}
	apply := func(name, val string) error {
		switch name {
		case "request":
			method = strings.ToUpper(strings.TrimSpace(val))
		case "url":
			target = val
		case "header":
			n, v, ok := strings.Cut(val, ":")
			switch {
			case !ok && strings.HasSuffix(val, ";"):
				warnOnce("Skipped the empty header “" + strings.TrimSuffix(val, ";") + "”.")
			case !ok:
				warnOnce("Skipped a header that isn’t written as Name: value.")
			case strings.TrimSpace(v) == "":
				warnOnce("Skipped the header “" + strings.TrimSpace(n) + "” because it has no value.")
			default:
				addHeader(n, strings.TrimLeft(v, " \t"))
			}
		case "data", "data-ascii", "data-binary", "data-raw":
			if name != "data-raw" && strings.HasPrefix(val, "@") {
				warnOnce("Skipped data that comes from a file. Hooks can’t read files. Paste the content into the body instead.")
				return nil
			}
			data = append(data, val)
		case "data-urlencode":
			enc := func(s string) string { return strings.ReplaceAll(url.QueryEscape(s), "+", "%20") }
			switch k, v, ok := strings.Cut(val, "="); {
			case strings.Contains(val, "@") && !ok:
				warnOnce("Skipped data that comes from a file. Hooks can’t read files. Paste the content into the body instead.")
			case !ok:
				data = append(data, enc(val))
			case k == "":
				data = append(data, enc(v))
			default:
				data = append(data, k+"="+enc(v))
			}
		case "json":
			jsonMode = true
			data = append(data, val)
		case "user":
			user = val
		case "oauth2-bearer":
			addHeader("Authorization", "Bearer "+val)
		case "user-agent":
			warnOnce("Skipped the User-Agent: Bitswarm sets the User-Agent itself.")
		case "cookie":
			if strings.Contains(val, "=") {
				addHeader("Cookie", val)
			} else {
				warnOnce("Skipped a cookie file. Hooks can’t read files.")
			}
		case "referer":
			addHeader("Referer", val)
		case "get":
			getMode = true
		case "head":
			return Invalid("Hooks can send POST, PUT, PATCH and GET requests. This command sends HEAD.")
		case "form", "form-string":
			warnOnce("Skipped a form field. Hooks can’t send multipart forms. Use a JSON body or a form-encoded body instead.")
		case "insecure":
			warnOnce("Ignored the option to skip certificate checks. Bitswarm always checks certificates.")
		case "output", "proxy", "cacert", "cert", "key", "upload-file", "config", "cookie-jar", "dump-header", "write-out", "proxy-user", "resolve", "interface", "unix-socket":
			warnOnce("Ignored the curl option --" + name + ". Hooks don’t support it.")
		default:
			if !curlQuiet[name] {
				warnOnce("Ignored the curl option --" + name + ".")
			}
		}
		return nil
	}

	for i := 1; i < len(words); i++ {
		w := words[i]
		switch {
		case w == "--":
			positional = append(positional, words[i+1:]...)
			i = len(words)
		case strings.HasPrefix(w, "--"):
			name, val, hasVal := strings.Cut(w[2:], "=")
			if curlLongWithArg[name] && !hasVal {
				if i+1 >= len(words) {
					return out, Invalid("The curl option --" + name + " needs a value.")
				}
				i++
				val = words[i]
			}
			if err := apply(name, val); err != nil {
				return out, err
			}
		case len(w) > 1 && w[0] == '-':
			for j := 1; j < len(w); j++ {
				ch := w[j]
				name, known := curlShortNames[ch]
				if !known {
					name = "-" + string(ch)
				}
				if strings.IndexByte(shortWithArg, ch) >= 0 {
					val := w[j+1:]
					if val == "" {
						if i+1 >= len(words) {
							return out, Invalid("The curl option -" + string(ch) + " needs a value.")
						}
						i++
						val = words[i]
					}
					if err := apply(name, val); err != nil {
						return out, err
					}
					break
				}
				if err := apply(name, ""); err != nil {
					return out, err
				}
			}
		default:
			positional = append(positional, w)
		}
	}

	if target == "" && len(positional) > 0 {
		target, positional = positional[0], positional[1:]
	}
	if len(positional) > 0 {
		warnOnce("Only the first address was used.")
	}
	if strings.TrimSpace(target) == "" {
		return out, Invalid("I couldn’t find an address in that command.")
	}
	target = strings.TrimSpace(target)
	switch lower := strings.ToLower(target); {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
	case strings.Contains(target, "://"):
		return out, Invalid("Hooks can only call http:// and https:// addresses.")
	default:
		target = "https://" + target
		warnOnce("The address had no http:// or https://, so https:// was added.")
	}

	if user != "" {
		addHeader("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user)))
	}
	body := strings.Join(data, "&")
	if jsonMode {
		body = strings.Join(data, "")
	}
	if getMode && len(data) > 0 {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + strings.ReplaceAll(body, " ", "%20")
		body = ""
		if method == "" {
			method = "GET"
		}
	}
	if method == "" {
		method = "GET"
		if body != "" {
			method = "POST"
		}
	}
	if !map[string]bool{"POST": true, "PUT": true, "PATCH": true, "GET": true}[method] {
		return out, Invalid("Hooks can send POST, PUT, PATCH and GET requests. This command sends " + method + ".")
	}
	if method == "GET" && body != "" {
		warnOnce("GET requests in Bitswarm don’t send a body, so the data was left out.")
		body = ""
	}
	if body != "" && !sawContentType {
		ct := "application/x-www-form-urlencoded"
		if jsonMode {
			ct = "application/json"
		}
		headers = append(headers, [2]string{"Content-Type", ct})
	}
	if jsonMode && !slicesHasHeader(headers, "accept") {
		headers = append(headers, [2]string{"Accept", "application/json"})
	}

	lines := make([]string, 0, len(headers))
	var literal []string
	for _, kv := range headers {
		lines = append(lines, kv[0]+": "+kv[1])
		if IsSecretHeader(kv[0]) && !HasSecretRef(kv[1]) {
			literal = append(literal, kv[0]+" header")
		}
	}
	if u, err := url.Parse(target); err == nil {
		for k, vs := range u.Query() {
			if secretParamRe.MatchString(k) && len(vs) > 0 && vs[0] != "" && !HasSecretRef(vs[0]) {
				literal = append(literal, "“"+k+"” in the address")
			}
		}
	}
	if len(literal) > 0 {
		out.Warnings = append(out.Warnings, "This command has a key in plain text ("+strings.Join(literal, ", ")+"). It will be saved in Bitswarm’s database. To keep it out, replace it with {{secret.NAME}} and set SWARM_SECRET_NAME on the server.")
	}

	out.Method, out.URL, out.Headers, out.Body = method, target, strings.Join(lines, "\n"), body
	return out, nil
}

func slicesHasHeader(hs [][2]string, name string) bool {
	for _, kv := range hs {
		if strings.EqualFold(kv[0], name) {
			return true
		}
	}
	return false
}
