package search

import (
	"fmt"
	"strconv"
	"time"

	"ytmgo/internal/queue"
	"ytmgo/internal/tidal"
)

// ─── Result ───────────────────────────────────────────────────────────

// Result is a single search/recommendation result.
type Result struct {
	ID       string
	Title    string
	Uploader string
	Duration int    // seconds
	URL      string
	CoverURL string // TIDAL album cover art URL (empty if unavailable)
}

// ToTrack converts a search Result to a queue.Track.
func (r Result) ToTrack() queue.Track {
	return queue.Track{
		ID:          r.ID,
		Title:       r.Title,
		Artist:      r.Uploader,
		Duration:    formatDuration(r.Duration),
		DurationSec: r.Duration,
		URL:         r.URL,
		CoverURL:    r.CoverURL,
	}
}

// ─── Public API ───────────────────────────────────────────────────────

// Search runs a TIDAL track search and returns up to limit results.
// Uses the provided TIDAL client for API calls.
func Search(query string, limit int, tc *tidal.Client) ([]Result, error) {
	tracks, err := tc.SearchTracks(query, limit, 0)
	if err != nil {
		return nil, fmt.Errorf("tidal search failed: %w", err)
	}
	return tidalResultsToResults(tracks), nil
}

// FetchRecommendations returns recommended tracks seeded from listening history.
// historyTrackIDs contains the most recent unique track IDs from play history.
// Falls back to a trending search if nothing else works.
func FetchRecommendations(limit int, tc *tidal.Client, historyTrackIDs []int) ([]Result, error) {
	tracks, err := tc.FetchRecommendations(limit, historyTrackIDs)
	if err != nil {
		return nil, fmt.Errorf("tidal recommendations failed: %w", err)
	}
	return tidalResultsToResults(tracks), nil
}

// MockSearch returns fake results for dev mode (no TIDAL API required).
func MockSearch(query string, limit int) []Result {
	songs := []struct{ title, artist string }{
		{"Bohemian Rhapsody", "Queen"},
		{"Hotel California", "Eagles"},
		{"Stairway to Heaven", "Led Zeppelin"},
		{"Smells Like Teen Spirit", "Nirvana"},
		{"Imagine", "John Lennon"},
		{"Purple Rain", "Prince"},
		{"Like a Rolling Stone", "Bob Dylan"},
		{"Hey Jude", "The Beatles"},
		{"What's Going On", "Marvin Gaye"},
		{"Respect", "Aretha Franklin"},
	}
	var results []Result
	for i, s := range songs {
		if i >= limit {
			break
		}
		id := fmt.Sprintf("mock_%d_%d", i, time.Now().UnixNano())
		results = append(results, Result{
			ID:       id,
			Title:    fmt.Sprintf("%s (matching: %s)", s.title, query),
			Uploader: s.artist,
			Duration: 180 + i*30,
			URL:      "https://tidal.com/browse/track/" + id,
		})
	}
	return results
}

// ─── TIDAL conversion ─────────────────────────────────────────────────

// tidalResultsToResults converts TIDAL API track results into search Results.
func tidalResultsToResults(tracks []tidal.TrackResult) []Result {
	results := make([]Result, 0, len(tracks))
	for _, t := range tracks {
		id := strconv.Itoa(t.ID)
		r := Result{
			ID:       id,
			Title:    t.Title,
			Uploader: t.ArtistName(),
			Duration: t.Duration,
			URL:      "",
			CoverURL: t.CoverURL(320, 320), // 320x320 album art from TIDAL
		}
		results = append(results, r)
	}
	return results
}

// ─── Formatting ───────────────────────────────────────────────────────

func formatDuration(secs int) string {
	d := time.Duration(secs) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}
