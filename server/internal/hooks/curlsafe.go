package hooks

import (
	"regexp"
	"strings"
)

var curlFormRe = regexp.MustCompile(`(^|\s)(-[A-Za-z]*F|--form)(\s|=|$)`)

func curlProblem(tpl string) string {
	locs := placeholderRe.FindAllStringSubmatchIndex(tpl, -1)
	if len(locs) == 0 {
		return ""
	}
	if curlFormRe.MatchString(tpl) {
		return "curl hooks can’t use -F or --form with placeholders, because curl reads files from their values. Use --form-string or --data-urlencode name={{...}} instead."
	}
	starts := map[int][]int{}
	for _, l := range locs {
		starts[l[0]] = l
	}
	var quote byte
	var word strings.Builder
	prev := ""
	endWord := func() {
		if word.Len() > 0 {
			prev = word.String()
			word.Reset()
		}
	}
	for i := 0; i < len(tpl); i++ {
		if l, ok := starts[i]; ok {
			name := tpl[l[2]:l[3]]
			if name != "payload" && !curlSafeSpot(word.String(), prev) {
				return "in a curl hook, put {{" + name + "}} right after an equals sign, such as name={{" + name + "}}, or inside a URL or a -H header value. Anywhere else, curl can read part of the value as a file or an option."
			}
			word.WriteString("X")
			i = l[1] - 1
			continue
		}
		c := tpl[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				word.WriteByte(c)
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '\\' && i+1 < len(tpl) && tpl[i+1] == '\n':
			i++
			endWord()
		case c == '\\':
			i++
			if i < len(tpl) {
				word.WriteByte(tpl[i])
			}
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			endWord()
		default:
			word.WriteByte(c)
		}
	}
	return ""
}

func curlSafeSpot(word, prev string) bool {
	switch {
	case strings.HasSuffix(word, "="):
		return true
	case strings.HasPrefix(word, "http://") || strings.HasPrefix(word, "https://"):
		return true
	case word != "" && (prev == "-H" || prev == "--header"):
		return true
	}
	return false
}
