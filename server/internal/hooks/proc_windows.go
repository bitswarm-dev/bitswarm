//go:build windows

package hooks

import (
	"os/exec"
	"time"
)

func killTree(cmd *exec.Cmd) {
	cmd.WaitDelay = 2 * time.Second
}
