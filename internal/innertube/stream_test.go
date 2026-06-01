// Package innertube — streaming benchmark test.
//
// This test is NOT a unit test (it makes live API calls and reads saved
// responses). Run with:
//
//	go test -v -run TestStreamTiming ./internal/innertube/
//	go test -v -run TestStreamTiming ./internal/innertube/ -saved-only
//
// Flags:
//
//	-saved-only   Use only the saved JSON; skip the live API call.
package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

var savedOnly = flag.Bool("saved-only", false, "skip live API, only use saved JSON")

// timestamps collects Result arrival times for analysis.
type timestamps struct {
	times []time.Duration // elapsed since start for each result
	start time.Time
}

func (ts *timestamps) add() {
	ts.times = append(ts.times, time.Since(ts.start))
}

// report prints a detailed timing analysis to the test log.
func (ts *timestamps) report(t *testing.T, label string) {
	if len(ts.times) == 0 {
		t.Logf("[%s] 0 results", label)
		return
	}

	t.Logf("")
	t.Logf("═══ %s ═══", label)
	t.Logf("  Results:     %d", len(ts.times))
	t.Logf("  Total time:  %v", ts.times[len(ts.times)-1])
	t.Logf("  First @      %v", ts.times[0])
	t.Logf("  Last  @      %v", ts.times[len(ts.times)-1])

	// Inter-arrival gaps.
	var gaps []time.Duration
	for i := 1; i < len(ts.times); i++ {
		gaps = append(gaps, ts.times[i]-ts.times[i-1])
	}

	minGap, maxGap := gaps[0], gaps[0]
	sum := time.Duration(0)
	for _, g := range gaps {
		if g < minGap {
			minGap = g
		}
		if g > maxGap {
			maxGap = g
		}
		sum += g
	}
	avgGap := sum / time.Duration(len(gaps))

	t.Logf("  Inter-arrival:")
	t.Logf("    min  %v", minGap)
	t.Logf("    avg  %v", avgGap)
	t.Logf("    max  %v", maxGap)

	// Batch detection: count how many results arrived within 1ms of the
	// previous one (same "batch").
	batches := 1
	batchSizes := []int{1}
	for i := 1; i < len(ts.times); i++ {
		gap := ts.times[i] - ts.times[i-1]
		if gap < 1*time.Millisecond {
			batchSizes[len(batchSizes)-1]++
		} else {
			batches++
			batchSizes = append(batchSizes, 1)
		}
	}
	t.Logf("  Batches:     %d", batches)
	t.Logf("  Batch sizes: %v", batchSizes)

	// Detail: print every result's arrival time.
	t.Logf("")
	t.Logf("  Per-result arrival:")
	for i, elapsed := range ts.times {
		gap := time.Duration(0)
		if i > 0 {
			gap = elapsed - ts.times[i-1]
		}
		t.Logf("  [%2d] %10v  (+%v)", i+1, elapsed, gap)
	}
	t.Logf("")
}

// ─── Saved JSON test ────────────────────────────────────────────────
//
// Feeds the saved response into the streaming parser at full speed (no
// network delay) to measure parsing + channel overhead.

func TestStreamTiming_SavedJSON(t *testing.T) {
	data, err := os.ReadFile("/tmp/innertube_full.json")
	if err != nil {
		t.Skipf("saved response not found: %v", err)
	}

	// Verify it's valid JSON.
	if !json.Valid(data) {
		t.Fatal("saved response is not valid JSON")
	}

	// Count sections to verify we're parsing the right structure.
	// getIn can't navigate through arrays, so we walk manually.
	var root map[string]interface{}
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	tabs := getIn(root, "contents", "tabbedSearchResultsRenderer", "tabs")
	if tabs == nil {
		t.Fatal("no tabs array in response")
	}
	tabsArr, ok := tabs.([]interface{})
	if !ok || len(tabsArr) == 0 {
		t.Fatal("tabs is not a non-empty array")
	}
	tab0, ok := tabsArr[0].(map[string]interface{})
	if !ok {
		t.Fatal("tabs[0] is not an object")
	}
	tabRenderer := getIn(tab0, "tabRenderer", "content", "sectionListRenderer", "contents")
	if tabRenderer == nil {
		t.Fatal("cannot find sectionListRenderer.contents in saved response")
	}
	contentsVal, ok := tabRenderer.([]interface{})
	if !ok {
		t.Fatal("contents is not an array")
	}
	t.Logf("Saved JSON: %d sections, %d bytes", len(contentsVal), len(data))

	// ── Benchmark 1: Instant read (full buffer already in memory) ──
	t.Run("instant", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result)
		done := make(chan struct{})
		defer close(done)

		// Reader: produce all results into channel.
		go func() {
			r := bytes.NewReader(data)
			streamParseSearchResults(r, 0, ch)
			close(ch)
		}()

		// Consumer: record arrival times.
		for r := range ch {
			_ = r
			ts.add()
			// Drain as fast as possible — measure the decoder's actual
			// output pacing, not channel backpressure.
		}
		ts.report(t, "INSTANT READ (no network)")
	})

	// ── Benchmark 2: Chunked read (simulate ~16 KB chunks over ~44 chunks) ──
	t.Run("chunked", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result)
		done := make(chan struct{})
		defer close(done)

		go func() {
			slow := &chunkedReader{data: data, chunkSize: 16 * 1024, delay: 5 * time.Millisecond}
			streamParseSearchResults(slow, 0, ch)
			close(ch)
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "CHUNKED (~16 KB/5ms per chunk)")
	})

	// ── Benchmark 3: Simulate network ~500ms delivery ──
	t.Run("network-sim", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result)
		done := make(chan struct{})
		defer close(done)

		// Deliver the full response over ~500ms in bursts.
		totalSize := len(data)
		bursts := 8
		burstSize := totalSize / bursts
		burstDelay := 500 * time.Millisecond / time.Duration(bursts)

		go func() {
			slow := &burstReader{
				data:      data,
				burstSize: burstSize,
				delay:     burstDelay,
			}
			streamParseSearchResults(slow, 0, ch)
			close(ch)
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "NETWORK SIM (~500ms total)")
	})
}

// ─── Live API test ──────────────────────────────────────────────────
//
// Makes a real InnerTube API call and runs the streaming parser on the
// raw response body. This is the most realistic timing measurement.

func TestStreamTiming_LiveAPI(t *testing.T) {
	if *savedOnly {
		t.Skip("-saved-only flag: skipping live API call")
	}

	client := NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	body := map[string]interface{}{"query": "music"}
	reader, err := client.postRaw(ctx, "search", body)
	if err != nil {
		t.Skipf("live API unavailable: %v", err)
	}
	defer reader.Close()

	// Read the response body so we can analyze it.
	raw, _ := io.ReadAll(reader)

	var root map[string]interface{}
	json.Unmarshal(raw, &root)
	tabs := getIn(root, "contents", "tabbedSearchResultsRenderer", "tabs")
	sections := 0
	if arr, ok := tabs.([]interface{}); ok && len(arr) > 0 {
		if tab0, ok := arr[0].(map[string]interface{}); ok {
			if c := getIn(tab0, "tabRenderer", "content", "sectionListRenderer", "contents"); c != nil {
				if ca, ok := c.([]interface{}); ok {
					sections = len(ca)
				}
			}
		}
	}
	t.Logf("Live API: %d sections, %d bytes", sections, len(raw))

	// Feed through streaming parser with two channel configurations.
	// We do this from buffer because the body is already consumed; but
	// this exercises the exact same parsing code path.

	// ── Buffered channel (old behavior) ──
	t.Run("buffered-ch", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result, 4)
		done := make(chan struct{})
		defer close(done)

		go func() {
			r := bytes.NewReader(raw)
			streamParseSearchResults(r, 0, ch)
			close(ch)
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "BUFFERED CH (cap=4)")
	})

	// ── Unbuffered channel (new behavior) ──
	t.Run("unbuffered-ch", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result) // cap=0
		done := make(chan struct{})
		defer close(done)

		go func() {
			r := bytes.NewReader(raw)
			streamParseSearchResults(r, 0, ch)
			close(ch)
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "UNBUFFERED CH (cap=0)")
	})

	// ── Unbuffered + consumer delay (simulates TUI frame rate) ──
	t.Run("unbuffered+tui-sim", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result)
		done := make(chan struct{})
		defer close(done)

		go func() {
			r := bytes.NewReader(raw)
			streamParseSearchResults(r, 0, ch)
			close(ch)
		}()

		for r := range ch {
			_ = r
			ts.add()
			// Simulate TUI frame time (~16ms at 60fps).
			time.Sleep(16 * time.Millisecond)
		}
		ts.report(t, "UNBUFFERED + TUI SIM (16ms per result)")
	})
}

// ─── StreamRecommendations end-to-end ───────────────────────────────
//
// Measures the full pipeline: SearchStream → StreamRecommendations → channel.

func TestStreamTiming_EndToEnd(t *testing.T) {
	if *savedOnly {
		t.Skip("-saved-only flag: skipping live API call")
	}

	// Simulate what the TUI does.
	const limit = 20

	// Buffered channel path (old).
	t.Run("e2e-buffered", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result, 10)
		done := make(chan struct{})

		go func() {
			defer close(ch)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			itubeCh := make(chan Result, 4)
			go func() {
				defer close(itubeCh)
				NewClient().SearchStream(ctx, "music", limit, itubeCh)
			}()

			for r := range itubeCh {
				select {
				case ch <- r:
				case <-done:
					return
				}
			}
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "E2E BUFFERED (itube=4→ch=10)")
	})

	// Unbuffered channel path (new).
	t.Run("e2e-unbuffered", func(t *testing.T) {
		ts := &timestamps{start: time.Now()}
		ch := make(chan Result) // cap=0
		done := make(chan struct{})

		go func() {
			defer close(ch)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			itubeCh := make(chan Result) // cap=0
			go func() {
				defer close(itubeCh)
				NewClient().SearchStream(ctx, "music", limit, itubeCh)
			}()

			for r := range itubeCh {
				select {
				case ch <- r:
				case <-done:
					return
				}
			}
		}()

		for r := range ch {
			_ = r
			ts.add()
		}
		ts.report(t, "E2E UNBUFFERED (itube=0→ch=0)")
	})
}

// ─── Helper: chunked reader ─────────────────────────────────────────

// chunkedReader wraps data and delivers it in small chunks with a delay
// between chunks to simulate a slow network stream.
type chunkedReader struct {
	data      []byte
	chunkSize int
	delay     time.Duration
	offset    int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	end := r.offset + r.chunkSize
	if end > len(r.data) {
		end = len(r.data)
	}
	n := copy(p, r.data[r.offset:end])
	r.offset += n

	if r.delay > 0 {
		time.Sleep(r.delay)
	}
	return n, nil
}

// ─── Helper: burst reader ───────────────────────────────────────────

// burstReader delivers data in N bursts over a total time period,
// simulating how a real HTTP response arrives in packets.
type burstReader struct {
	data      []byte
	burstSize int
	delay     time.Duration
	offset    int
	buf       []byte // remaining data from partial reads
}

func (r *burstReader) Read(p []byte) (int, error) {
	// Drain any buffered data first.
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	if r.offset >= len(r.data) {
		return 0, io.EOF
	}

	end := r.offset + r.burstSize
	if end > len(r.data) {
		end = len(r.data)
	}

	chunk := r.data[r.offset:end]
	r.offset = end

	n := copy(p, chunk)
	if n < len(chunk) {
		// Buffer overflow — save remaining for next Read.
		r.buf = make([]byte, len(chunk)-n)
		copy(r.buf, chunk[n:])
	}

	if r.delay > 0 && r.offset < len(r.data) {
		time.Sleep(r.delay)
	}
	return n, nil
}

// ─── Print helpers ──────────────────────────────────────────────────

// TestMain handles flag setup and pretty-print for the benchmark suite.
func TestMain(m *testing.M) {
	flag.Parse()
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  innertube streaming benchmark")
	fmt.Println(strings.Repeat("─", 60))
	os.Exit(m.Run())
}
