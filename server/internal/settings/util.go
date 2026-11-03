package settings

import (
	"encoding/json"
	"sort"
	"strconv"
)

func itoa(n int) string { return strconv.Itoa(n) }

func sortStrings(s []string) { sort.Strings(s) }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func jsonutilLoose(raw []byte, dst *map[string]any) error {
	return json.Unmarshal(raw, dst)
}
