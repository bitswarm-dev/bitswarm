package redact

import (
	"errors"
	"net/url"
	"strings"
)

func URL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexAny(raw, "?#"); i >= 0 {
			return raw[:i] + "?…"
		}
		return raw
	}
	hadQuery := u.RawQuery != "" || u.ForceQuery
	u.User, u.RawQuery, u.ForceQuery, u.Fragment, u.RawFragment = nil, "", false, "", ""
	out := u.String()
	if hadQuery {
		out += "?…"
	}
	return out
}

func Err(err error) string {
	if err == nil {
		return ""
	}
	var ue *url.Error
	if errors.As(err, &ue) && ue.URL != "" {
		return strings.ReplaceAll(err.Error(), ue.URL, URL(ue.URL))
	}
	return err.Error()
}
