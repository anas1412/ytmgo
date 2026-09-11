package downloader

import (
	"runtime"
	"testing"
)

// Close has to wait for the worker, not just signal it. Returning early
// leaves a goroutine that is still writing into the download directory:
// on CI that showed up as a test's temporary directory being "not
// empty" while Go was removing it, and in the app it means quitting
// mid-download can leave a partial file behind.
//
// The symptom is a race and will not reproduce on demand, so this
// checks the property instead: once Close returns, the worker is gone.
func TestCloseWaitsForTheWorker(t *testing.T) {
	before := runtime.NumGoroutine()

	const n = 20
	ds := make([]*Downloader, 0, n)
	for i := 0; i < n; i++ {
		ds = append(ds, New(t.TempDir(), "m4a"))
	}
	if got := runtime.NumGoroutine(); got < before+n {
		t.Fatalf("%d goroutines after starting %d downloaders, want at least %d",
			got, n, before+n)
	}
	for _, d := range ds {
		d.Close()
	}

	// No sleep, no retry loop: that is the point. If Close waits, every
	// worker has already returned by the time the last one does.
	if got := runtime.NumGoroutine(); got > before {
		t.Errorf("%d goroutines still running after Close, started from %d — "+
			"Close returned before its worker stopped", got, before)
	}
}

// A shutting-down downloader must not pin its worker on a progress
// send. The channel is buffered, but nothing guarantees a reader, and
// Close waits for the worker — so a blocking send would turn into a
// hang on quit.
func TestSendDoesNotBlockAfterClose(t *testing.T) {
	d := New(t.TempDir(), "m4a")
	// Exactly to capacity: one more before Close would block, with no
	// reader and the context still live, which is correct.
	for i := 0; i < cap(d.progress); i++ {
		d.send(ProgressEvent{TrackID: "x"})
	}
	d.Close()
	// Past the buffer, with no reader: this returns only because send
	// gives up when the context is done.
	for i := 0; i < 100; i++ {
		d.send(ProgressEvent{TrackID: "y"})
	}
}
