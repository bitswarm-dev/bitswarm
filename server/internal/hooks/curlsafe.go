package hooks

import "regexp"

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
	hasText := false
	for i := 0; i < len(tpl); i++ {
		if l, ok := starts[i]; ok {
			name := tpl[l[2]:l[3]]
			if !hasText && name != "payload" {
				return "in a curl hook, put {{" + name + "}} after a name and an equals sign, such as name={{" + name + "}}. On its own, curl can read it as a file or an option."
			}
			i, hasText = l[1]-1, true
			continue
		}
		c := tpl[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			} else {
				hasText = true
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '\\' && i+1 < len(tpl) && tpl[i+1] == '\n':
			i++
			hasText = false
		case c == '\\':
			i++
			hasText = true
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			hasText = false
		default:
			hasText = true
		}
	}
	return ""
}
