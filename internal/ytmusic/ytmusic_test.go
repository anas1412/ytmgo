package ytmusic

import (
	"strings"
	"testing"
)

func TestParseClock(t *testing.T) {
	cases := map[string]int{
		"4:58":    298,
		"0:07":    7,
		"1:02:33": 3753,
		"2016":    0,
		"":        0,
		"live":    0,
	}
	for in, want := range cases {
		if got := parseClock(in); got != want {
			t.Errorf("parseClock(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestIsVideoID(t *testing.T) {
	if !IsVideoID("sZxzPcT1Meg") {
		t.Error("valid videoId rejected")
	}
	for _, bad := range []string{"12345678", "/home/user/file.m4a", "", "way-too-long-to-be-an-id"} {
		if IsVideoID(bad) {
			t.Errorf("IsVideoID(%q) = true, want false", bad)
		}
	}
}

// TestLiveSearchAndRadio hits the real InnerTube endpoints. Skipped
// automatically when the network or the endpoint is unavailable, so CI
// on a bot-walled runner doesn't flake.
func TestLiveSearchAndRadio(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}

	tracks, err := Search("kazumasa oda love story", 5)
	if err != nil {
		t.Skipf("live search unavailable: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("live search returned zero tracks")
	}
	first := tracks[0]
	if !IsVideoID(first.VideoID) {
		t.Fatalf("bad videoId %q", first.VideoID)
	}
	if first.Title == "" || first.Artist == "" {
		t.Fatalf("missing metadata: %+v", first)
	}
	if first.Duration <= 0 {
		t.Fatalf("missing duration: %+v", first)
	}
	if first.CoverURL != "" && !strings.Contains(first.CoverURL, "=w544-h544") {
		t.Fatalf("thumbnail not upscaled: %s", first.CoverURL)
	}

	radio, err := Radio(first.VideoID, 5)
	if err != nil {
		t.Skipf("live radio unavailable: %v", err)
	}
	if len(radio) == 0 {
		t.Fatal("radio returned zero tracks")
	}
	for _, r := range radio {
		if r.VideoID == first.VideoID {
			t.Fatal("radio included the seed track")
		}
		if !IsVideoID(r.VideoID) || r.Title == "" {
			t.Fatalf("bad radio item: %+v", r)
		}
	}
	t.Logf("search: %q by %q (%ds), radio: %d tracks, first: %q by %q",
		first.Title, first.Artist, first.Duration, len(radio), radio[0].Title, radio[0].Artist)
}

// TestLiveAlbums exercises album search and the album page against the
// real endpoints. Skipped when the network or endpoint is unavailable.
func TestLiveAlbums(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	albums, err := SearchAlbums("mild high club timeline", 5)
	if err != nil {
		t.Skipf("live album search unavailable: %v", err)
	}
	if len(albums) == 0 {
		t.Fatal("album search returned nothing")
	}
	a := albums[0]
	if a.BrowseID == "" || a.Title == "" {
		t.Fatalf("incomplete album: %+v", a)
	}

	full, err := AlbumTracks(a.BrowseID)
	if err != nil {
		t.Skipf("live album page unavailable: %v", err)
	}
	if len(full.Tracks) == 0 {
		t.Fatal("album page returned no tracks")
	}
	for i, tr := range full.Tracks {
		if !IsVideoID(tr.VideoID) {
			t.Fatalf("track %d has bad videoId %q", i+1, tr.VideoID)
		}
		if tr.Title == "" {
			t.Fatalf("track %d has no title", i+1)
		}
	}
	withDur := 0
	for _, tr := range full.Tracks {
		if tr.Duration > 0 {
			withDur++
		}
	}
	if withDur == 0 {
		t.Error("no track had a duration")
	}
	t.Logf("album %q by %q (%s): %d tracks, %d with durations; first: %q (%ds)",
		full.Title, full.Artist, full.Year, len(full.Tracks), withDur,
		full.Tracks[0].Title, full.Tracks[0].Duration)
}
