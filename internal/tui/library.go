package tui

import (
	"fmt"
	"strings"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
)

// searchResultToTrack converts a search result to a queue track.
func searchResultToTrack(r search.Result) queue.Track {
	d := r.Duration
	m := d / 60
	s := d % 60
	return queue.Track{
		ID:          r.ID,
		Title:       r.Title,
		Artist:      r.Uploader,
		Duration:    fmt.Sprintf("%d:%02d", m, s),
		DurationSec: d,
		URL:         r.URL,
	}
}

// ─── Library matching ───────────────────────────────────────────────
//
// Search results and library entries describe the same song with
// different field shapes:
//
//	search:  ID=YouTube video ID  | Title=raw (e.g. "Song (Official Video)")
//	                                Artist=uploader (e.g. "Channel - Topic")
//	library: ID=file path         | Title=cleaned (e.g. "Song")
//	                                Artist=filename prefix (e.g. "Channel")
//
// YouTube IDs and file paths can never match, so library lookup is
// done via a normalized "artist|title" signature that strips the
// common YouTube decorations (" - Topic" channel suffix, "(Official
// Video)" title suffix, etc.) before comparing.

// normalizeForMatch lower-cases, trims, and strips the YouTube-specific
// decorations that differ between a search result and a library entry
// even when they refer to the same track.
func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// YouTube auto-generated channels append " - Topic" to the channel
	// name. The library filename parser doesn't capture this suffix.
	s = strings.TrimSuffix(s, " - topic")
	// Common title decorations that the library filename parser strips
	// (see library.cleanTitle) but search results preserve.
	for _, suf := range []string{
		"(official music video)", "(official video)", "(official lyric video)",
		"(lyric video)", "(lyrics)", "(audio)", "(official audio)",
		"[official music video]", "[official video]", "[lyrics]",
	} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimSpace(s)
}

// trackMatchKey returns the signature used for library lookups.
// Format: "artist|title" with both fields normalized.
func trackMatchKey(t queue.Track) string {
	return normalizeForMatch(t.Artist) + "|" + normalizeForMatch(t.Title)
}

// findLibraryMatch searches the local library for a track whose
// signature matches t. Returns the matching library track and true,
// or a zero Track and false. Library entries with empty FilePath are
// skipped (they aren't actually playable).
func findLibraryMatch(library []queue.Track, t queue.Track) (queue.Track, bool) {
	sig := trackMatchKey(t)
	for i := range library {
		lt := library[i]
		if lt.FilePath == "" {
			continue
		}
		if trackMatchKey(lt) == sig {
			return lt, true
		}
	}
	return queue.Track{}, false
}

// resolveTrack converts a search result to a queue Track, consulting
// the local library so that an already-downloaded file is preferred
// over re-streaming from YouTube. The returned track has Downloaded
// and FilePath set when a library match exists, and the play sites
// (playSelectedQueueItem, SongEnded auto-advance, startTrackPlayback)
// all check those fields first.
func (m *Model) resolveTrack(r search.Result) queue.Track {
	t := searchResultToTrack(r)
	if lib, ok := findLibraryMatch(m.library, t); ok {
		t.Downloaded = true
		t.FilePath = lib.FilePath
	}
	return t
}

// backfillQueueFromLibrary walks the current queue and back-fills
// FilePath/Downloaded for any track that was added from search before
// the library scan completed (i.e. when resolveTrack saw an empty
// library). Call this from the LibraryScanMsg handler so the
// play-locally-first behavior is consistent regardless of timing.
func (m *Model) backfillQueueFromLibrary() {
	if m.queue == nil || len(m.library) == 0 {
		return
	}
	// Tracks() returns a copy so we can iterate safely while the
	// queue continues to be used. For each unresolved track whose
	// signature matches a library entry, patch the in-queue track
	// in place via the queue's matching-by-key update helper.
	for _, t := range m.queue.Tracks() {
		if t.Downloaded && t.FilePath != "" {
			continue
		}
		lib, ok := findLibraryMatch(m.library, t)
		if !ok {
			continue
		}
		fp := lib.FilePath
		sig := trackMatchKey(t)
		m.queue.UpdateTrackByMatch(sig, trackMatchKey, func(qt *queue.Track) {
			qt.Downloaded = true
			qt.FilePath = fp
		})
	}
}

// filteredLibrary returns library tracks that match the search input (case-insensitive).
// When the input is empty or not on the library page, returns all tracks.
func (m Model) filteredLibrary() []queue.Track {
	if m.activePage != PageLibrary {
		return m.library
	}
	q := m.searchInput.Value()
	if q == "" {
		return m.library
	}
	q = strings.ToLower(q)
	var out []queue.Track
	for _, t := range m.library {
		if strings.Contains(strings.ToLower(t.Title), q) || strings.Contains(strings.ToLower(t.Artist), q) {
			out = append(out, t)
		}
	}
	return out
}
