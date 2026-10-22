package discordrpc

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// A song without a cover link is looked up once, in the background;
// later asks get the answer kept. A failure keeps nothing.
func TestCoverForAsksOnce(t *testing.T) {
	var asks atomic.Int32
	fail := atomic.Bool{}
	release := make(chan struct{})
	findCover = func(artist, title string) (string, error) {
		asks.Add(1)
		<-release
		if fail.Load() {
			return "", errors.New("offline")
		}
		return "https://yt/" + title + ".jpg", nil
	}
	got := make(chan string, 4)
	for range 3 {
		if c, ok := coverFor("Djo", "End of Beginning", func(c string) { got <- c }); ok || c != "" {
			t.Fatalf("first asks: %q, %v", c, ok)
		}
	}
	close(release)
	if c := <-got; c != "https://yt/End of Beginning.jpg" {
		t.Errorf("found %q", c)
	}
	if c, ok := coverFor("djo", "end of beginning", nil); !ok || c != "https://yt/End of Beginning.jpg" {
		t.Errorf("kept: %q, %v", c, ok)
	}
	if n := asks.Load(); n != 1 {
		t.Errorf("asked %d times, want once", n)
	}

	fail.Store(true)
	coverFor("X", "Offline", func(string) { t.Error("a failure was reported as a cover") })
	deadline := time.Now().Add(time.Second)
	for asks.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if _, ok := coverFor("X", "Offline", func(string) {}); ok {
		t.Error("a failure was kept as an answer")
	}
}
