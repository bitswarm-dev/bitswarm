package hooks

import (
	"bytes"
	"encoding/json"
)

func looseUnmarshal(raw []byte, dst any) error { return json.Unmarshal(raw, dst) }

func jsonIndent(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return string(bytes.TrimRight(b.Bytes(), "\n")), nil
}

func jsonString(v string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	out := bytes.TrimRight(b.Bytes(), "\n")
	return string(out[1 : len(out)-1])
}
