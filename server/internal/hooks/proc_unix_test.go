//go:build !windows

package hooks

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestKillTree(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
	}{
		{"single command", "sleep 5"},
		{"shell with a child that outlives it", "sleep 5 & wait"},
		{"pipeline", "sleep 5 | cat"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", tt.cmd)
			killTree(cmd)
			start := time.Now()
			_ = cmd.Run()
			if took := time.Since(start); took > 3*time.Second {
				t.Errorf("%q kept running for %v after its timeout", tt.cmd, took)
			}
		})
	}
}
