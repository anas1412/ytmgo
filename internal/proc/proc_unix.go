//go:build !windows

package proc

import (
	"context"
	"os/exec"
	"syscall"
)

// platform needs no state on Unix: the process group is the kernel's.
type platform struct{}

func (*platform) started()  {}
func (*platform) finished() {}

// CommandContext builds a command in its own process group, so
// cancelling the context kills the group and everything in it.
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	return &Cmd{Cmd: cmd}
}
