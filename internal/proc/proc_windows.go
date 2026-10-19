//go:build windows

package proc

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// platform holds the command's Job Object: Windows' own way to treat a
// process and everything it starts as one unit. The job is created to
// kill all of it when its last handle closes, which gives three things
// from one mechanism:
//
//   - Cancel terminates the job: the tool and every child it started.
//     yt-dlp.exe is a launcher with the real yt-dlp as a child, which
//     in turn starts ffmpeg.
//   - Wait closes the handle, which kills anything still left in the
//     job — a child whose parent already exited, that a walk of the
//     process tree would no longer find.
//   - If ytmgo itself crashes, Windows closes the handle for it, so its
//     tools do not run on as orphans. Linux gets the same from the
//     process group and the player's Pdeathsig.
type platform struct {
	mu  sync.Mutex
	job windows.Handle
}

// CommandContext builds a command that runs without a console window,
// and whose process tree goes with it on cancel, on exit and on crash.
func CommandContext(ctx context.Context, name string, args ...string) *Cmd {
	c := &Cmd{Cmd: exec.CommandContext(ctx, name, args...)}
	// A GUI app has no console, so without this every console tool it
	// starts opens a window of its own for as long as it runs.
	c.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	c.Cancel = func() error {
		c.mu.Lock()
		job := c.job
		c.mu.Unlock()
		if job != 0 {
			return windows.TerminateJobObject(job, 1)
		}
		return c.Process.Kill() // no job: at least the tool itself
	}
	return c
}

// finished closes the job, killing anything still in it.
func (p *platform) finished() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job != 0 {
		windows.CloseHandle(p.job)
		p.job = 0
	}
}

// started puts the new process in a job of its own. If any step fails
// the command still runs, just without the job: cancel then kills only
// the tool itself.
//
// ponytail: the process is assigned just after it starts, so a child it
// spawns in those first microseconds escapes the job. yt-dlp's launcher
// unpacks itself before starting anything, and ffmpeg and ffprobe start
// nothing. Starting suspended would close the gap, but exec cannot
// resume a suspended process.
func (c *Cmd) started() {
	job, err := killOnCloseJob()
	if err != nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(c.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(job, h); err != nil {
		windows.CloseHandle(job)
		return
	}
	c.mu.Lock()
	c.job = job
	c.mu.Unlock()
}

func killOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}

// Open opens a folder in Explorer. Not through CommandContext: the
// window is the user's, and must not close when ytmgo does.
func Open(path string) error {
	cmd := exec.Command("explorer.exe", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() // release the process handle
	return nil
}
