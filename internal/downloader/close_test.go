package downloader

import (
	"runtime"
	"strings"
	"testing"
)

// liveWorkers counts the goroutines running the download worker, by
// reading their stacks.
//
// This is for the check after Close, where runtime.NumGoroutine is the
// wrong instrument: it counts the whole process, so anything that
// happened to start a goroutine in the same window — the testing
// package, an HTTP transport, the runtime itself — read as a leaked
// worker, and CI failed with "3 still running, started from 2".
//
// It is no good for counting workers as they start: a goroutine that
// has been created but not yet scheduled is not running its function
// yet, so it has no frame to match, and 20 downloaders counted 16.
// NumGoroutine sees those, which is why the check above still uses it.
func liveWorkers() int {
	buf := make([]byte, 1<<20)
	var n int
	for {
		n = runtime.Stack(buf, true)
		if n < len(buf) {
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	live := 0
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if !strings.Contains(g, "(*Downloader).worker") {
			continue
		}
		live++
	}
	return live
}

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
	// The runtime knows about a goroutine the moment it is created, so
	// this counts them all whether or not they have started running.
	if got := runtime.NumGoroutine(); got < before+n {
		t.Fatalf("%d goroutines after starting %d downloaders, want at least %d",
			got, n, before+n)
	}
	for _, d := range ds {
		d.Close()
	}

	// No sleep, no retry loop: that is the point. If Close waits, every
	// worker has already returned by the time the last one does.
	// One straggler is allowed, and only one. wg.Done is deferred, so
	// Wait can return while the last worker is still unwinding its own
	// return — it has finished its work and writes nothing more, but it
	// is still on a stack for a moment. Measured under -race, that shows
	// up as exactly one; a Close that does not wait leaves four or more.
	//
	// This is a probabilistic guard, not a proof: under -race the
	// scheduler lets workers exit quickly enough that a non-waiting
	// Close is caught about a third of the time. Catching it sometimes
	// over many runs is worth more than a check that fails for reasons
	// of its own.
	if got := liveWorkers(); got > 1 {
		t.Errorf("%d download workers still running after Close — "+
			"Close returned before its worker stopped", got)
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
