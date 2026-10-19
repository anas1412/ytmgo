package tui

import (
	"fmt"
	"strings"

	"ytmgo/internal/library"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
)

// searchResultToTrack converts a search result to a queue track.
func searchResultToTrack(r search.Result) queue.Track {
	d := r.Duration
	m := d / 60
	s := d % 60
	return queue.Track{
		ID:             r.ID,
		Title:          r.Title,
		Artist:         r.Uploader,
		Album:          r.Album,
		Duration:       fmt.Sprintf("%d:%02d", m, s),
		DurationSec:    d,
		URL:            r.URL,
		CoverURL:       r.CoverURL,
		AlbumBrowseID:  r.AlbumBrowseID,
		ArtistBrowseID: r.ArtistBrowseID,
	}
}

// resolveTrack converts a search result to a queue Track, consulting
// the local library so that an already-downloaded file is preferred
// over re-streaming from YouTube. The returned track has Downloaded
// and FilePath set when a library match exists, and the play sites
// (playSelectedQueueItem, SongEnded auto-advance, startTrackPlayback)
// all check those fields first.
func (m *Model) resolveTrack(r search.Result) queue.Track {
	t := searchResultToTrack(r)
	if lib, ok := library.Match(m.library, t); ok {
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
		lib, ok := library.Match(m.library, t)
		if !ok {
			continue
		}
		fp := lib.FilePath
		sig := library.MatchKey(t)
		m.queue.UpdateTrackByMatch(sig, library.MatchKey, func(qt *queue.Track) {
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
