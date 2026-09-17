package search

import (
	"testing"

	"ytmgo/internal/ytmusic"
)

func TestParsePlaylist(t *testing.T) {
	for _, tc := range []struct {
		in     string
		source string
		id     string
	}{
		// YouTube, in the shapes people actually paste.
		{"https://www.youtube.com/playlist?list=PLFgquLnL59alCl_2TQvOiD5Vgm1hCaGSI", "youtube", "PLFgquLnL59alCl_2TQvOiD5Vgm1hCaGSI"},
		{"https://music.youtube.com/playlist?list=OLAK5uy_kmzoSOa_tCizE", "youtube", "OLAK5uy_kmzoSOa_tCizE"},
		{"https://m.youtube.com/playlist?list=PLabc-123_x", "youtube", "PLabc-123_x"},
		{"youtube.com/playlist?list=PLbare", "youtube", "PLbare"},
		// A watch link that happens to sit in a playlist.
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLmixed123", "youtube", "PLmixed123"},
		{"  https://www.youtube.com/playlist?list=PLpadded  ", "youtube", "PLpadded"},

		// Spotify: web link, share link with tracking, localised, and URI.
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M", "spotify", "37i9dQZF1DXcBWIGoYBM5M"},
		{"https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M?si=abc123", "spotify", "37i9dQZF1DXcBWIGoYBM5M"},
		{"https://open.spotify.com/intl-fr/playlist/37i9dQZF1DXcBWIGoYBM5M", "spotify", "37i9dQZF1DXcBWIGoYBM5M"},
		{"spotify:playlist:37i9dQZF1DXcBWIGoYBM5M", "spotify", "37i9dQZF1DXcBWIGoYBM5M"},
	} {
		got, ok := ParsePlaylist(tc.in)
		if !ok {
			t.Errorf("ParsePlaylist(%q) = not a playlist, want %s", tc.in, tc.source)
			continue
		}
		if got.Source != tc.source || got.ID != tc.id {
			t.Errorf("ParsePlaylist(%q) = %s/%s, want %s/%s", tc.in, got.Source, got.ID, tc.source, tc.id)
		}
	}
}

// Ordinary searches must never be mistaken for a playlist, or typing a
// query would fetch something instead of searching for it.
func TestParsePlaylistIgnoresQueries(t *testing.T) {
	for _, q := range []string{
		"", "charli xcx", "PLFgquLnL59alCl", "playlist", "my playlist list=PL123",
		"spotify", "open.spotify.com/track/abc", "youtube.com/watch?v=dQw4w9WgXcQ",
		"https://music.youtube.com/browse/MPREb_8En9jdKgx9I",
	} {
		if ref, ok := ParsePlaylist(q); ok {
			t.Errorf("ParsePlaylist(%q) = %s/%s, want a plain search", q, ref.Source, ref.ID)
		}
	}
}

func TestBestMatch(t *testing.T) {
	c := func(id string, dur int) ytmusic.Track {
		return ytmusic.Track{VideoID: id, Duration: dur}
	}
	for _, tc := range []struct {
		name  string
		want  int
		cands []ytmusic.Track
		pick  string
	}{
		{"skips the hour-long loop for the real length",
			200, []ytmusic.Track{c("loop", 3600), c("real", 198)}, "real"},
		{"relevance wins among candidates that all fit",
			200, []ytmusic.Track{c("top", 202), c("exact", 200)}, "top"},
		{"nothing in range falls back to the closest, not the top hit",
			200, []ytmusic.Track{c("loop", 3600), c("near", 260)}, "near"},
		{"unknown wanted length keeps the top hit",
			0, []ytmusic.Track{c("top", 3600), c("other", 200)}, "top"},
		{"candidates without a duration are ignored",
			200, []ytmusic.Track{c("nodur", 0), c("real", 201)}, "real"},
		{"a live take is rejected in favour of the album cut",
			215, []ytmusic.Track{c("live", 402), c("album", 213)}, "album"},
	} {
		if got := bestMatch(tc.cands, tc.want); got.VideoID != tc.pick {
			t.Errorf("%s: picked %q, want %q", tc.name, got.VideoID, tc.pick)
		}
	}
}

// TestLivePlaylistTracks runs both sources end to end: a YouTube link
// straight through, and a Spotify link resolved track by track.
func TestLivePlaylistTracks(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	for _, tc := range []struct{ name, link string }{
		{"youtube", "https://www.youtube.com/playlist?list=PLFgquLnL59alCl_2TQvOiD5Vgm1hCaGSI"},
		{"spotify", "https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M"},
	} {
		ref, ok := ParsePlaylist(tc.link)
		if !ok {
			t.Fatalf("%s: link was not recognised", tc.name)
		}
		title, results, missed, err := PlaylistTracks(ref)
		if err != nil {
			t.Skipf("%s unavailable: %v", tc.name, err)
		}
		if title == "" {
			t.Errorf("%s: no playlist title", tc.name)
		}
		if len(results) < 10 {
			t.Fatalf("%s: got %d tracks, want a full playlist", tc.name, len(results))
		}
		for i, r := range results {
			if !ytmusic.IsVideoID(r.ID) {
				t.Errorf("%s track %d: bad videoId %q", tc.name, i, r.ID)
			}
			if r.Title == "" || r.URL == "" {
				t.Errorf("%s track %d: not playable: %+v", tc.name, i, r)
			}
		}
		t.Logf("%s: %q — %d tracks (%d unresolved), first %q by %q",
			tc.name, title, len(results), missed, results[0].Title, results[0].Uploader)
	}
}
