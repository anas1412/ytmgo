//go:build !windows

package proc

import (
	"context"
	"os/exec"
	"syscall"
)

// CommandContext builds a command in its own process group, so
// cancelling the context kills the group and everything in it.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return cmd
}
