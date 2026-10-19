//go:build !windows

package proc

import (
	"bufio"
	"context"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Cancelling has to take the children down too: yt-dlp's ffmpeg is a
// grandchild of ytmgo, and a plain Process.Kill would leave it running.
func TestCancelKillsGrandchild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// The shell starts a sleep in the background, prints its pid, and
	// waits on it: the sleep is the grandchild.
	cmd := CommandContext(ctx, "sh", "-c", "sleep 30 & echo $!; wait")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("pid %q: %v", line, err)
	}

	cancel()
	cmd.Wait()

	// The sleep is reparented and reaped once it dies, so poll for it
	// to stop existing rather than expecting it gone at once.
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("grandchild %d still running after cancel", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
