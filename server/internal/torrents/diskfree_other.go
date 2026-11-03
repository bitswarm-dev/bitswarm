//go:build !unix

package torrents

func DiskFree(dir string) float64 { return 0 }
