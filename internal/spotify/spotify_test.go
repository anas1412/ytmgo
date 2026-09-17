package spotify

import "testing"

// TestLivePlaylist hits the real embed endpoint. Skipped when the
// network or the endpoint is unavailable, so CI never flakes on it.
func TestLivePlaylist(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	// Today's Top Hits — Spotify's own editorial playlist, always public.
	p, err := PlaylistTracks("37i9dQZF1DXcBWIGoYBM5M")
	if err != nil {
		t.Skipf("live playlist unavailable: %v", err)
	}
	if p.Title == "" {
		t.Error("playlist has no title")
	}
	if len(p.Tracks) < 10 {
		t.Fatalf("got %d tracks, want a full playlist", len(p.Tracks))
	}
	for i, tr := range p.Tracks {
		if tr.Title == "" || tr.Artist == "" {
			t.Errorf("track %d: missing metadata: %+v", i, tr)
		}
		if tr.DurationSec <= 0 {
			t.Errorf("track %d (%q): no duration", i, tr.Title)
		}
	}
	t.Logf("playlist %q: %d tracks, first %q by %q (%ds)",
		p.Title, len(p.Tracks), p.Tracks[0].Title, p.Tracks[0].Artist, p.Tracks[0].DurationSec)
}

// A private or bogus id must fail with a usable message, not a panic or
// an empty playlist that looks like success.
func TestBadPlaylistFails(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	if _, err := PlaylistTracks("0000000000000000000000"); err == nil {
		t.Error("a nonexistent playlist came back without an error")
	}
}

func TestTidy(t *testing.T) {
	for in, want := range map[string]string{
		"KAROL G, Judeline, rusowsky": "KAROL G, Judeline, rusowsky",
		"  spaced   out  ":            "spaced out",
		"plain":                       "plain",
		"":                            "",
	} {
		if got := tidy(in); got != want {
			t.Errorf("tidy(%q) = %q, want %q", in, got, want)
		}
	}
}
