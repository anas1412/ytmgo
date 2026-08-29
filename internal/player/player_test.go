package player

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// writeWAV writes a silent 16-bit mono PCM WAV of the given duration.
func writeWAV(t *testing.T, path string, seconds float64) {
	t.Helper()
	const rate = 8000
	n := int(seconds * rate)
	data := make([]byte, 44+n*2)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+n*2))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(data[22:24], 1) // mono
	binary.LittleEndian.PutUint32(data[24:28], rate)
	binary.LittleEndian.PutUint32(data[28:32], rate*2) // byte rate
	binary.LittleEndian.PutUint16(data[32:34], 2)      // block align
	binary.LittleEndian.PutUint16(data[34:36], 16)     // bits
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(n*2))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func requireMpv(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed; skipping integration test")
	}
	t.Setenv("YTMGO_MPV_AO", "null")
}

// TestPersistentPlayback verifies the full lifecycle against a real mpv:
// one process across multiple tracks, position events, a natural end
// (reason "eof") emitting Ended, and manual Stop NOT emitting Ended.
func TestPersistentPlayback(t *testing.T) {
	requireMpv(t)

	dir := t.TempDir()
	short := filepath.Join(dir, "short.wav")
	long := filepath.Join(dir, "long.wav")
	writeWAV(t, short, 1.0)
	writeWAV(t, long, 30.0)

	p := New()
	defer p.Shutdown()

	if err := p.Play(short); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if got := p.State(); got != StatePlaying {
		t.Fatalf("state after Play = %v, want StatePlaying", got)
	}
	firstPid := p.cmd.Process.Pid

	// Position updates must arrive from the observe_property stream.
	select {
	case <-p.Positions():
	case <-time.After(5 * time.Second):
		t.Fatal("no position update within 5s")
	}

	// The 1s file must end naturally and emit exactly one Ended.
	select {
	case <-p.Ended():
	case <-time.After(10 * time.Second):
		t.Fatal("no Ended after track finished")
	}

	// Second Play must reuse the same mpv process (no respawn).
	if err := p.Play(long); err != nil {
		t.Fatalf("second Play: %v", err)
	}
	if pid := p.cmd.Process.Pid; pid != firstPid {
		t.Fatalf("mpv respawned: pid %d != %d", pid, firstPid)
	}

	// Pause round-trip.
	if !p.Pause() {
		t.Fatal("Pause returned false")
	}
	if got := p.State(); got != StatePaused {
		t.Fatalf("state after Pause = %v, want StatePaused", got)
	}
	if !p.Pause() {
		t.Fatal("un-Pause returned false")
	}
	if got := p.State(); got != StatePlaying {
		t.Fatalf("state after resume = %v, want StatePlaying", got)
	}

	// Track switch while playing must NOT emit Ended (reason "stop").
	if err := p.Play(long); err != nil {
		t.Fatalf("switch Play: %v", err)
	}
	// Manual Stop must NOT emit Ended either.
	p.Stop()
	select {
	case <-p.Ended():
		t.Fatal("Ended emitted for track switch or manual stop")
	case <-time.After(1500 * time.Millisecond):
	}
	if got := p.State(); got != StateStopped {
		t.Fatalf("state after Stop = %v, want StateStopped", got)
	}
}

// TestVolumeClamp exercises the volume bounds without needing mpv.
func TestVolumeClamp(t *testing.T) {
	p := New()
	p.SetVolume(150)
	if got := p.Volume(); got != 100 {
		t.Fatalf("volume = %d, want 100", got)
	}
	p.SetVolume(-5)
	if got := p.Volume(); got != 0 {
		t.Fatalf("volume = %d, want 0", got)
	}
}
