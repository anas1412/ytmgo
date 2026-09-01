package search

import (
	"fmt"
	"time"

	"ytmgo/internal/queue"
	"ytmgo/internal/ytmusic"
)

// ─── Result ───────────────────────────────────────────────────────────

// Result is a single search/recommendation result.
type Result struct {
	ID            string
	Title         string
	Uploader      string
	Album         string // album title (empty when the source didn't say)
	Duration      int    // seconds
	URL           string // playable YouTube Music watch URL
	CoverURL      string // album art URL (empty if unavailable)
	AlbumBrowseID string // album page id (MPREb_…); empty when unknown
}

// ToTrack converts a search Result to a queue.Track.
func (r Result) ToTrack() queue.Track {
	return queue.Track{
		ID:            r.ID,
		Title:         r.Title,
		Artist:        r.Uploader,
		Album:         r.Album,
		Duration:      formatDuration(r.Duration),
		DurationSec:   r.Duration,
		URL:           r.URL,
		CoverURL:      r.CoverURL,
		AlbumBrowseID: r.AlbumBrowseID,
	}
}

// ─── Public API ───────────────────────────────────────────────────────

// Search runs a YouTube Music songs search and returns up to limit
// results. Every result carries the exact videoId (as ID) and a
// playable watch URL, so no later artist-title resolution is needed.
func Search(query string, limit int) ([]Result, error) {
	tracks, err := ytmusic.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("youtube music search failed: %w", err)
	}
	return ytTracksToResults(tracks), nil
}

// FetchRecommendations returns recommended tracks seeded from listening
// history. seedVideoIDs are the most recent unique videoIds from play
// history (newest first); each seed contributes its YouTube Music radio
// queue.
//
// Everything it returns is related to something actually played. With no
// seeds, or with a radio that returns nothing usable, it returns
// nothing: there is no filler. Padding the result with whatever charts
// globally is what dropped an unrelated hit into a queue of game
// soundtrack — a recommendation nobody asked for is worse than no
// recommendation, and autoplay appending one is worse still.
func FetchRecommendations(limit int, seedVideoIDs []string) ([]Result, error) {
	seen := make(map[string]bool)
	for _, id := range seedVideoIDs {
		seen[id] = true // never recommend something just played
	}

	var results []Result
	var lastErr error
	maxSeeds := 2
	for i, seed := range seedVideoIDs {
		if i >= maxSeeds || len(results) >= limit {
			break
		}
		// Ask for headroom, not just what is wanted: the radio for a
		// track tends to open with its neighbours, which are exactly
		// the tracks just played and about to be filtered out. Asking
		// for `limit` alone left autoplay (limit 1) with a single
		// candidate that was almost always already seen.
		tracks, err := ytmusic.Radio(seed, limit+len(seen))
		if err != nil {
			lastErr = err
			continue
		}
		for _, t := range tracks {
			if seen[t.VideoID] {
				continue
			}
			seen[t.VideoID] = true
			results = append(results, ytTrackToResult(t))
			if len(results) >= limit {
				break
			}
		}
	}

	// A radio that returned nothing usable is worth reporting: the
	// caller can say so rather than showing an empty list as though
	// there were genuinely nothing to play next.
	if len(results) == 0 && len(seedVideoIDs) > 0 && lastErr != nil {
		return nil, fmt.Errorf("youtube music recommendations failed: %w", lastErr)
	}

	return results, nil
}

// ─── conversion ───────────────────────────────────────────────────────

func ytTrackToResult(t ytmusic.Track) Result {
	return Result{
		ID:            t.VideoID,
		Title:         t.Title,
		Uploader:      t.Artist,
		Album:         t.Album,
		Duration:      t.Duration,
		URL:           ytmusic.WatchURL(t.VideoID),
		CoverURL:      t.CoverURL,
		AlbumBrowseID: t.AlbumBrowseID,
	}
}

func ytTracksToResults(tracks []ytmusic.Track) []Result {
	results := make([]Result, 0, len(tracks))
	for _, t := range tracks {
		results = append(results, ytTrackToResult(t))
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
