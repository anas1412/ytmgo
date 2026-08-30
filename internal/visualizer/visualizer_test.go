package visualizer

import (
	"os/exec"
	"testing"
	"time"
)

func TestAvailable(t *testing.T) {
	_, err := exec.LookPath("cava")
	if got := Available(); got != (err == nil) {
		t.Errorf("Available() = %v but LookPath err = %v", got, err)
	}
}

// TestLiveFrames runs the real cava and checks the frames parse. Values
// are all zero when nothing is playing, which is still a valid frame.
func TestLiveFrames(t *testing.T) {
	if !Available() {
		t.Skip("cava not installed")
	}
	v, err := Start(24)
	if err != nil {
		t.Skipf("cava unavailable (no audio source?): %v", err)
	}
	defer v.Close()

	select {
	case f := <-v.Frames():
		if len(f) != 24 {
			t.Fatalf("frame has %d bars, want 24", len(f))
		}
		for i, n := range f {
			if n < 0 || n > 100 {
				t.Fatalf("bar %d = %d, outside 0..100", i, n)
			}
		}
		t.Logf("got a %d-bar frame", len(f))
	case <-time.After(6 * time.Second):
		t.Fatal("no frame within 6s")
	}
}

// TestOddBarsProduceFrames guards the bug where an odd bar count made
// cava exit immediately and silently, leaving the UI waiting forever.
func TestOddBarsProduceFrames(t *testing.T) {
	if !Available() {
		t.Skip("cava not installed")
	}
	for _, bars := range []int{23, 17, 5} {
		v, err := Start(bars)
		if err != nil {
			t.Skipf("cava unavailable: %v", err)
		}
		select {
		case f, ok := <-v.Frames():
			if !ok {
				t.Errorf("bars=%d: cava died (err=%v)", bars, v.Err())
			} else if len(f) == 0 {
				t.Errorf("bars=%d: empty frame", bars)
			} else if len(f)%2 != 0 {
				t.Errorf("bars=%d: got %d bars, should be rounded even", bars, len(f))
			}
		case <-time.After(5 * time.Second):
			t.Errorf("bars=%d: no frame (err=%v)", bars, v.Err())
		}
		v.Close()
	}
}

// TestBarClamp keeps the requested bar count inside cava's limits.
func TestBarClamp(t *testing.T) {
	if !Available() {
		t.Skip("cava not installed")
	}
	for _, n := range []int{1, 1000} {
		v, err := Start(n)
		if err != nil {
			t.Skipf("cava unavailable: %v", err)
		}
		v.Close()
	}
}
