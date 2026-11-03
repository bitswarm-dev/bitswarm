//go:build unix

package torrents

import (
	"path/filepath"
	"syscall"
)

func DiskFree(dir string) float64 {
	var st syscall.Statfs_t
	for cur := dir; ; cur = filepath.Dir(cur) {
		if syscall.Statfs(cur, &st) == nil {
			return float64(st.Bavail) * float64(st.Bsize)
		}
		if cur == filepath.Dir(cur) {
			return 0
		}
	}
}
