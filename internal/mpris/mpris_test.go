package mpris

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestLiveBus exercises the full MPRIS surface against the real session
// bus: property publishing, metadata, and inbound method calls. Skipped
// when no session bus is available (CI, headless).
func TestLiveBus(t *testing.T) {
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no session bus")
	}

	svc, err := Start()
	if err != nil {
		t.Skipf("mpris unavailable: %v", err)
	}
	defer svc.Close()

	svc.Publish(Snapshot{
		TrackID:  "sZxzPcT1Meg",
		Title:    "ラブ・ストーリーは突然に",
		Artist:   "Kazumasa Oda",
		Duration: 298,
		Position: 42,
		Playing:  true,
		Volume:   80,
	})

	busctl := func(args ...string) string {
		out, err := exec.Command("busctl", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("busctl %v: %v: %s", args, err, out)
		}
		return string(out)
	}
	if _, err := exec.LookPath("busctl"); err != nil {
		t.Skip("busctl not installed")
	}

	status := busctl("--user", "get-property",
		"org.mpris.MediaPlayer2.ytmgo", "/org/mpris/MediaPlayer2",
		"org.mpris.MediaPlayer2.Player", "PlaybackStatus")
	if !strings.Contains(status, "Playing") {
		t.Fatalf("PlaybackStatus = %q, want Playing", status)
	}

	md := busctl("--user", "get-property",
		"org.mpris.MediaPlayer2.ytmgo", "/org/mpris/MediaPlayer2",
		"org.mpris.MediaPlayer2.Player", "Metadata")
	if !strings.Contains(md, "Kazumasa Oda") {
		t.Fatalf("Metadata missing artist: %q", md)
	}

	// Fire PlayPause at ourselves like a media key would.
	busctl("--user", "call",
		"org.mpris.MediaPlayer2.ytmgo", "/org/mpris/MediaPlayer2",
		"org.mpris.MediaPlayer2.Player", "PlayPause")
	select {
	case c := <-svc.Commands():
		if c != CmdPlayPause {
			t.Fatalf("command = %v, want CmdPlayPause", c)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no command received after PlayPause call")
	}
}
