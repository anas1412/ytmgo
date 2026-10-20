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

// Open opens a folder in the file manager, or a link in the browser,
// with xdg-open, without waiting for what it opens to close.
func Open(path string) error {
	cmd := exec.Command("xdg-open", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it, so it does not linger as a zombie
	return nil
}

// Hand starts a program that is to outlive ytmgo, in no group of
// ytmgo's, with nothing waiting for it to finish.
func Hand(path string, args ...string) error {
	cmd := exec.Command(path, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // reap it, so it does not linger as a zombie
	return nil
}
