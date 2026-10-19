package proc

import (
	"bytes"
	"context"
	"testing"
	"time"
)

// Cancelling has to end the child too, not only the process started:
// yt-dlp.exe is a launcher with the real yt-dlp as its child. Here cmd
// is the launcher and ping the child. ping holds the stdout pipe, and
// Wait does not return until the pipe closes, so a cancel that killed
// only cmd would leave Wait blocked for as long as ping runs.
func TestCancelKillsTree(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := CommandContext(ctx, "cmd", "/c", "ping -n 30 127.0.0.1")
	cmd.Stdout = &bytes.Buffer{}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)

	start := time.Now()
	cancel()
	cmd.Wait()
	if d := time.Since(start); d > 10*time.Second {
		t.Fatalf("child still running %s after cancel", d)
	}
}
