package proc

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestMain lets the test binary stand in for a tool and its child, so
// TestWaitKillsOrphans needs no shell and no quoting: as "parent" it
// starts a copy of itself as "child" and exits without waiting; the
// child writes a marker after a few seconds, unless killed first.
func TestMain(m *testing.M) {
	switch os.Getenv("PROC_TEST_ROLE") {
	case "parent":
		child := exec.Command(os.Args[0])
		child.Env = append(os.Environ(), "PROC_TEST_ROLE=child")
		if err := child.Start(); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	case "child":
		time.Sleep(3 * time.Second)
		os.WriteFile(os.Getenv("PROC_TEST_MARKER"), []byte("alive"), 0644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A child that outlives its parent is still in the job, and goes when
// Wait closes it. Walking the process tree (taskkill /T) would not find
// it any more: its parent is gone.
func TestWaitKillsOrphans(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "alive")
	cmd := CommandContext(context.Background(), os.Args[0])
	cmd.Env = append(os.Environ(), "PROC_TEST_ROLE=parent", "PROC_TEST_MARKER="+marker)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the orphaned child was still running after Wait")
	}
}

// Hand passes its arguments as written: a folder with a space in it
// reaches the program unquoted, the way NSIS reads /D=.
func TestHandUnquoted(t *testing.T) {
	out := filepath.Join(t.TempDir(), "args.txt")
	if err := Hand(`C:\windows\system32\cmd.exe`, `/c echo /D=C:\Jane Doe\ytmgo>`+out); err != nil {
		t.Fatal(err)
	}
	var got []byte
	for i := 0; i < 50 && len(got) == 0; i++ {
		time.Sleep(100 * time.Millisecond)
		got, _ = os.ReadFile(out)
	}
	if s := strings.TrimSpace(string(got)); s != `/D=C:\Jane Doe\ytmgo` {
		t.Errorf("got %q", s)
	}
}
