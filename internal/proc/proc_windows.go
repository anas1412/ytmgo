//go:build windows

package proc

import (
	"context"
	"os/exec"
	"strconv"
	"syscall"
)

// createNoWindow is CREATE_NO_WINDOW. A GUI app on Windows has no
// console, so without it every console tool it starts opens a window
// of its own for as long as it runs.
const createNoWindow = 0x08000000

// CommandContext builds a command that runs without a console window,
// and whose cancellation kills its whole process tree. Windows has no
// process groups to signal; taskkill /T walks the tree instead. That
// matters more here than on Unix: the yt-dlp .exe is itself a launcher
// that starts the real yt-dlp as a child.
//
// ponytail: taskkill, not a Job Object. /T finds children through their
// parent, so a child whose parent has already exited is missed; move to
// a Job Object if orphans ever show up.
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
	cmd.Cancel = func() error {
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
		return kill.Run()
	}
	return cmd
}
