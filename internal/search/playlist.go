package search

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"ytmgo/internal/spotify"
	"ytmgo/internal/ytmusic"
)

// PlaylistRef is a playlist link the search box accepted.
type PlaylistRef struct {
	Source string // "youtube" or "spotify"
	ID     string
}

// Only links are recognised, never a bare id: a plain query like "PLAY"
// must stay a search, so the URL (or spotify: URI) is what marks the
// input as a playlist.
var (
	ytListRe = regexp.MustCompile(`(?i)\b(?:music\.|www\.|m\.)?youtube\.com/\S*?[?&]list=([A-Za-z0-9_-]+)`)
	spotRe   = regexp.MustCompile(`(?i)(?:open\.spotify\.com/(?:intl-[a-z-]+/)?playlist/|spotify:playlist:)([A-Za-z0-9]+)`)
)

// ParsePlaylist reports whether s is a playlist link, and which one.
func ParsePlaylist(s string) (PlaylistRef, bool) {
	s = strings.TrimSpace(s)
	if m := spotRe.FindStringSubmatch(s); m != nil {
		return PlaylistRef{Source: "spotify", ID: m[1]}, true
	}
	if m := ytListRe.FindStringSubmatch(s); m != nil {
		return PlaylistRef{Source: "youtube", ID: m[1]}, true
	}
	return PlaylistRef{}, false
}

// ─── Fetching ───────────────────────────────────────────────────────

// matchTolerance is how far a candidate's length may sit from the one
// Spotify reported and still count as the same recording. Masters differ
// by a second or two; a live take, an extended mix or an hour-long loop
// does not.
const matchTolerance = 5

// resolveWorkers bounds the fan-out. A hundred-track playlist is a
// hundred searches, and firing them all at once looks like abuse.
const resolveWorkers = 6

// PlaylistTracks resolves a playlist reference into playable results.
// It returns the playlist's title, the tracks it could resolve, and how
// many it could not — a playlist is still worth having when one track
// is missing, so a miss is skipped rather than failing the whole fetch.
func PlaylistTracks(ref PlaylistRef) (title string, results []Result, missed int, err error) {
	switch ref.Source {
	case "youtube":
		p, err := ytmusic.PlaylistTracks(ref.ID)
		if err != nil {
			return "", nil, 0, fmt.Errorf("youtube playlist failed: %w", err)
		}
		t := p.Title
		if t == "" {
			t = "Playlist"
		}
		return t, ytTracksToResults(p.Tracks), 0, nil

	case "spotify":
		p, err := spotify.PlaylistTracks(ref.ID)
		if err != nil {
			return "", nil, 0, err
		}
		found := resolveSpotify(p.Tracks)
		results = make([]Result, 0, len(found))
		for _, r := range found {
			if r.ID != "" {
				results = append(results, r)
			}
		}
		t := p.Title
		if t == "" {
			t = "Playlist"
		}
		return t, results, len(p.Tracks) - len(results), nil
	}
	return "", nil, 0, fmt.Errorf("unknown playlist source %q", ref.Source)
}

// resolveSpotify searches YouTube Music for each Spotify track, keeping
// playlist order. Entries that resolve to nothing come back zeroed.
func resolveSpotify(tracks []spotify.Track) []Result {
	out := make([]Result, len(tracks))
	sem := make(chan struct{}, resolveWorkers)
	var wg sync.WaitGroup
	for i, t := range tracks {
		wg.Add(1)
		go func(i int, t spotify.Track) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// A few candidates, not one: the top hit is usually right but
			// is also where the live takes and hour-long loops turn up.
			cands, err := ytmusic.Search(t.Title+" "+t.Artist, 5)
			if err != nil || len(cands) == 0 {
				return
			}
			out[i] = ytTrackToResult(bestMatch(cands, t.DurationSec))
		}(i, t)
	}
	wg.Wait()
	return out
}

// bestMatch picks the candidate that is most likely the same recording:
// the first hit whose length is within tolerance, since the results are
// already ordered by relevance. With nothing in range it falls back to
// the closest length rather than the top hit, which is what keeps a
// three-minute song from importing as somebody's hour-long loop of it.
func bestMatch(cands []ytmusic.Track, wantSec int) ytmusic.Track {
	if wantSec <= 0 {
		return cands[0]
	}
	best, bestDiff := cands[0], 1<<30
	for _, c := range cands {
		if c.Duration <= 0 {
			continue
		}
		diff := c.Duration - wantSec
		if diff < 0 {
			diff = -diff
		}
		if diff <= matchTolerance {
			return c
		}
		if diff < bestDiff {
			best, bestDiff = c, diff
		}
	}
	return best
}
