// Package spotify reads a public Spotify playlist's track names.
//
// Spotify's Web API needs a registered app and an OAuth token, which is
// more than this is worth: the embed player that any page can iframe
// serves the same tracklist as plain JSON with no credentials at all.
// That is what this reads.
//
// It returns names, never anything playable — Spotify audio is not
// reachable. Turning a name into a recording is the resolver's job.
package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Track is one playlist entry as Spotify lists it.
type Track struct {
	Title       string
	Artist      string
	DurationSec int
}

// Playlist is a fetched playlist and its tracks, in playlist order.
type Playlist struct {
	ID     string
	Title  string
	Tracks []Track
}

// nextDataRe pulls the Next.js payload the embed page ships its state in.
var nextDataRe = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// embedPayload is the slice of the embed state that matters.
type embedPayload struct {
	Props struct {
		PageProps struct {
			State struct {
				Data struct {
					Entity struct {
						Name      string `json:"name"`
						TrackList []struct {
							Title    string `json:"title"`
							Subtitle string `json:"subtitle"` // artists, comma separated
							Duration int    `json:"duration"` // milliseconds
						} `json:"trackList"`
					} `json:"entity"`
				} `json:"data"`
			} `json:"state"`
		} `json:"pageProps"`
	} `json:"props"`
}

// PlaylistTracks fetches a public playlist by id.
func PlaylistTracks(id string) (Playlist, error) {
	p := Playlist{ID: id}

	req, err := http.NewRequest("GET", "https://open.spotify.com/embed/playlist/"+id, nil)
	if err != nil {
		return p, fmt.Errorf("spotify: request: %w", err)
	}
	// The embed is served to browsers; a default Go agent gets a stub.
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return p, fmt.Errorf("spotify: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("spotify: HTTP %d (is the playlist public?)", resp.StatusCode)
	}

	body := make([]byte, 0, 128<<10)
	buf := make([]byte, 32<<10)
	for {
		n, err := resp.Body.Read(buf)
		body = append(body, buf[:n]...)
		if err != nil || len(body) > 8<<20 {
			break
		}
	}

	m := nextDataRe.FindSubmatch(body)
	if m == nil {
		return p, fmt.Errorf("spotify: no playlist data in the page (the embed format changed?)")
	}
	var payload embedPayload
	if err := json.Unmarshal(m[1], &payload); err != nil {
		return p, fmt.Errorf("spotify: decode: %w", err)
	}

	e := payload.Props.PageProps.State.Data.Entity
	p.Title = e.Name
	for _, t := range e.TrackList {
		if t.Title == "" {
			continue
		}
		p.Tracks = append(p.Tracks, Track{
			Title:       tidy(t.Title),
			Artist:      tidy(t.Subtitle),
			DurationSec: t.Duration / 1000,
		})
	}
	if len(p.Tracks) == 0 {
		return p, fmt.Errorf("spotify: playlist has no tracks (private or empty?)")
	}
	return p, nil
}

// tidy normalises the page's typography into something a search query
// and a track list can both use: Spotify separates artists with
// non-breaking spaces, which survive into the UI and into the query
// string looking like ordinary spaces while matching nothing.
func tidy(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", " ")
	return strings.Join(strings.Fields(s), " ")
}
