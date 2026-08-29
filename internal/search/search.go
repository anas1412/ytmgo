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
	ID       string
	Title    string
	Uploader string
	Duration int    // seconds
	URL      string // playable YouTube Music watch URL
	CoverURL string // album art URL (empty if unavailable)
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
// queue. Falls back to a trending search when no seeds are usable.
func FetchRecommendations(limit int, seedVideoIDs []string) ([]Result, error) {
	seen := make(map[string]bool)
	for _, id := range seedVideoIDs {
		seen[id] = true // never recommend something just played
	}

	var results []Result
	maxSeeds := 2
	for i, seed := range seedVideoIDs {
		if i >= maxSeeds || len(results) >= limit {
			break
		}
		tracks, err := ytmusic.Radio(seed, limit)
		if err != nil {
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

	// Fallback: nothing seeded (fresh install, or radio unavailable).
	if len(results) < limit {
		tracks, err := ytmusic.Search("trending songs", limit-len(results))
		if err != nil && len(results) == 0 {
			return nil, fmt.Errorf("youtube music recommendations failed: %w", err)
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

	return results, nil
}

// ─── conversion ───────────────────────────────────────────────────────

func ytTrackToResult(t ytmusic.Track) Result {
	return Result{
		ID:       t.VideoID,
		Title:    t.Title,
		Uploader: t.Artist,
		Duration: t.Duration,
		URL:      ytmusic.WatchURL(t.VideoID),
		CoverURL: t.CoverURL,
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
