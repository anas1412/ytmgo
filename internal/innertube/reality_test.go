// Package innertube — live terminal simulation test.
//
// Matches the real Bubbletea TUI pipeline exactly:
//   - Live API call via SearchStream (same function the app uses)
//   - itubeCh → ch forwarding (same channel sizes as real app)
//   - Per-frame reads synchronized to 60fps (same as Bubbletea event loop)
//
// Run:
//
//	go test -v -run TestReality ./internal/innertube/
package innertube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// Real TUI pipeline:
//
//	SearchStream(ctx, "music", limit, itubeCh)   ← live HTTP + streaming JSON decode
//	  → itubeCh (cap=0)                            ← innertube.Result
//	  → ch (cap=0)                                 ← search.Result (via convertResult)
//	  → readNextRecCmd ← tea.Msg                  ← one per Bubbletea frame
//
// All channels are cap=0 (unbuffered). Each result blocks on send until the
// TUI finishes processing the previous frame. This test simulates that exact
// flow with a configurable channel capacity knob.

// frame captures what the TUI would show at one point in time.
type frame struct {
	at     time.Duration // wall time since test start
	count  int           // how many results visible so far
	result *Result       // the new result that just arrived (nil = initial frame)
}

// TestReality simulates the full TUI experience:
//  1. Live API call to "music" via SearchStream
//  2. Full streaming pipeline (SearchStream → itubeCh → ch)
//  3. Bubbletea-like event loop that processes one result per frame
//  4. Terminal output with actual titles, artists, timestamps
func TestReality(t *testing.T) {
	// Verify the API is reachable so both paths use live data.
	if !canReachAPI() {
		t.Skip("API unreachable — no live test possible")
	}

	// ── Run both pipeline variants side by side ───────────────────
	t.Run("buffered-vs-unbuffered", func(t *testing.T) {
		// Old pipeline: buffered channels (cap 4 → cap 10) + live API.
		t.Logf("\n╔══════════════════════════════════════════════╗")
		t.Logf("║  OLD: BUFFERED CHANNELS (itube=4 → ch=10)  ║")
		t.Logf("╚══════════════════════════════════════════════╝")
		frames := runPipeline(true, t)
		renderTimeline(t, frames)

		// New pipeline: unbuffered channels (cap=0) + live API.
		t.Logf("\n╔══════════════════════════════════════════════╗")
		t.Logf("║  NEW: UNBUFFERED CHANNELS (itube=0 → ch=0)  ║")
		t.Logf("╚══════════════════════════════════════════════╝")
		frames = runPipeline(false, t)
		renderTimeline(t, frames)
	})
}

// canReachAPI does a lightweight probe to the innertube search endpoint.
func canReachAPI() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewClient()
	reader, err := client.postRaw(ctx, "search", map[string]interface{}{"query": "music"})
	if err != nil {
		return false
	}
	// Drain and close to avoid resource leaks.
	io.Copy(io.Discard, reader)
	reader.Close()
	return true
}

// runPipeline executes one full streaming cycle matching the real TUI pipeline.
//
//   - buffered=true:  OLD behavior — full response via postRaw, batch decode via
//     parseSearchResults, buffered channels (itubeCh cap=4, ch cap=10).
//     All results arrive in memory before the first one is sent.
//
//   - buffered=false: NEW behavior — streaming decode via SearchStream,
//     unbuffered channels (itubeCh cap=0, ch cap=0).  Results arrive one
//     by one as each section is decoded off the wire.  This matches the
//     current real TUI in model.go and search.go.
func runPipeline(buffered bool, t *testing.T) []frame {
	const limit = 20
	start := time.Now()
	var frames []frame

	// Record initial frame (0 results).
	frames = append(frames, frame{at: 0, count: 0, result: nil})

	ch := make(chan Result)
	if buffered {
		ch = make(chan Result, 10)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		defer close(ch)

		if buffered {
			// ── OLD PATH ──────────────────────────────────────────
			// Full HTTP response via postRaw → batch decode with
			// parseSearchResults → send all results at once through
			// buffered channels.
			client := NewClient()
			reader, err := client.postRaw(ctx, "search",
				map[string]interface{}{"query": "music"})
			if err != nil {
				t.Logf("  postRaw error: %v", err)
				return
			}
			raw, _ := io.ReadAll(reader)
			reader.Close()

			var resp map[string]interface{}
			json.Unmarshal(raw, &resp)
			results, _ := parseSearchResults(resp, limit)

			itubeCh := make(chan Result, 4)
			go func() {
				defer close(itubeCh)
				for _, r := range results {
					select {
					case itubeCh <- r:
					case <-ctx.Done():
						return
					}
				}
			}()
			for r := range itubeCh {
				select {
				case ch <- r:
				case <-ctx.Done():
					return
				}
			}
		} else {
			// ── NEW PATH ──────────────────────────────────────────
			// Streaming JSON decode via SearchStream → send one-by-one
			// through unbuffered channels.  Matches real TUI exactly
			// (see search.go StreamRecommendations + model.go).
			itubeCh := make(chan Result)
			go func() {
				defer close(itubeCh)
				client := NewClient()
				err := client.SearchStream(ctx, "music", limit, itubeCh)
				if err != nil {
					t.Logf("  SearchStream error: %v", err)
				}
			}()
			for r := range itubeCh {
				select {
				case ch <- r:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Bubbletea event loop: one result per frame, 60fps.
	seq := 0
	for r := range ch {
		seq++
		frames = append(frames, frame{
			at:     time.Since(start),
			count:  seq,
			result: &r,
		})

		// Same timing as Bubbletea: process then sleep to 60fps boundary.
		elapsed := time.Since(start)
		nextFrame := time.Duration(seq) * (time.Second / 60)
		if remaining := nextFrame - elapsed; remaining > 0 {
			time.Sleep(remaining)
		}
	}

	return frames
}

// renderTimeline prints a frame-by-frame terminal simulation.
func renderTimeline(t *testing.T, frames []frame) {
	if len(frames) <= 1 {
		t.Logf("  No results\n")
		return
	}

	header := fmt.Sprintf("  %-6s  %-9s  %-45s  %-25s  %s",
		"FRAME", "TIME", "TITLE", "ARTIST", "ID")
	t.Logf("  " + strings.Repeat("─", len(header)+4))
	t.Logf("  " + header)
	t.Logf("  " + strings.Repeat("─", len(header)+4))

	for i, f := range frames {
		if f.result == nil {
			if i == 0 {
				t.Logf("  %-6s  %-9s  %-45s  %-25s  %s",
					"init", "0s", "(loading…)", "", "")
			}
			continue
		}

		r := f.result
		timeStr := fmt.Sprintf("%.3fs", f.at.Seconds())

		title := r.Title
		if len(title) > 43 {
			title = title[:40] + "..."
		}
		artist := r.Uploader
		if len(artist) > 23 {
			artist = artist[:20] + "..."
		}

		id := r.ID
		if len(id) > 12 {
			id = id[:12] + "..."
		}
		t.Logf("  %-6s  %-9s  %-45s  %-25s  %s",
			fmt.Sprintf("#%d", f.count),
			timeStr,
			title,
			artist,
			id,
		)
	}

	// Summary.
	last := frames[len(frames)-1]
	totalTime := last.at

	t.Logf("  " + strings.Repeat("─", len(header)+4))
	t.Logf("")
	t.Logf("  Results: %d  |  Total: %.3fs  |  Rate: %.1f results/s",
		last.count, totalTime.Seconds(), float64(last.count)/totalTime.Seconds())

	// Per-result arrival detail.
	t.Logf("")
	t.Logf("  Per-result arrival timeline:")
	for i, f := range frames {
		if f.result == nil {
			continue
		}
		gap := time.Duration(0)
		if i > 1 && frames[i-1].result != nil {
			gap = f.at - frames[i-1].at
		} else if i > 1 {
			for j := i - 1; j > 0; j-- {
				if frames[j].result != nil {
					gap = f.at - frames[j].at
					break
				}
			}
		}
		t.Logf("    [#%2d] %8.3fs  (+%7.3fs)  %s",
			f.count, f.at.Seconds(), gap.Seconds(), f.result.Title)
	}

	// Batch detection (same frame group = < 20ms gap).
	t.Logf("")
	var batchStarts []time.Duration
	batchSizes := []int{}
	batchIdx := -1
	for i, f := range frames {
		if f.result == nil {
			continue
		}
		var gap time.Duration
		for j := i - 1; j >= 0; j-- {
			if frames[j].result != nil {
				gap = f.at - frames[j].at
				break
			}
		}
		if gap > 20*time.Millisecond || gap == 0 {
			batchStarts = append(batchStarts, f.at)
			batchSizes = append(batchSizes, 1)
			batchIdx++
		} else {
			batchSizes[batchIdx]++
		}
	}
	t.Logf("  Batches:  %d  %v", len(batchSizes), batchSizes)
	for i, size := range batchSizes {
		at := batchStarts[i]
		rate := "..."
		if size > 1 && i > 0 {
			prevAt := batchStarts[i-1]
			rate = fmt.Sprintf("%.1f/s", float64(size)/(at-prevAt).Seconds())
		}
		t.Logf("    batch %d: %d results @ %.3fs  %s", i+1, size, at.Seconds(), rate)
	}
	t.Logf("")
}


