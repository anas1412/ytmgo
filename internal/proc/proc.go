// Package proc builds the commands ytmgo runs for external tools
// (yt-dlp, ffmpeg, ffprobe), so every one of them is started and
// cancelled the same way on each platform.
//
// Cancelling has to kill the whole tree, not just the process started:
// yt-dlp spawns ffmpeg children that a plain Process.Kill would orphan.
// Each platform does that its own native way, in proc_unix.go and
// proc_windows.go; nothing here imitates one platform on another.
package proc

import (
	"bytes"
	"errors"
	"os/exec"
)

// Cmd is an exec.Cmd with the platform's process-tree handling around
// Start and Wait. Fields (Stdout, Stdin, ...) and the other methods are
// exec.Cmd's own.
type Cmd struct {
	*exec.Cmd
	platform
}

// Start starts the command, then puts it under the platform's tree
// handling.
func (c *Cmd) Start() error {
	if err := c.Cmd.Start(); err != nil {
		return err
	}
	c.started()
	return nil
}

// Wait waits for the command, then releases what Start set up.
func (c *Cmd) Wait() error {
	err := c.Cmd.Wait()
	c.finished()
	return err
}

// Run starts the command and waits for it.
func (c *Cmd) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}

// Output runs the command and returns its stdout. Like exec.Cmd's, on
// a failed exit the returned *exec.ExitError carries stderr. It is
// written out here because exec.Cmd.Output would call exec's own Run,
// skipping Start and Wait above.
func (c *Cmd) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	var stdout bytes.Buffer
	c.Stdout = &stdout
	var stderr *bytes.Buffer
	if c.Stderr == nil {
		stderr = &bytes.Buffer{}
		c.Stderr = stderr
	}
	err := c.Run()
	if ee, ok := err.(*exec.ExitError); ok && stderr != nil {
		ee.Stderr = stderr.Bytes()
	}
	return stdout.Bytes(), err
}
