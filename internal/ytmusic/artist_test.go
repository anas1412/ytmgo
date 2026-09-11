package ytmusic

import (
	"strings"
	"testing"
)

// TestLiveArtist: an artist page must come back with a name, a
// discography whose ids AlbumTracks can actually open, and the full top
// songs list — not the five-row shelf, which carries no durations.
func TestLiveArtist(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	// Daft Punk. A stable, large catalogue.
	a, err := Artist("UCRr1xG_2WIDs18a6cIiCxeA")
	if err != nil {
		t.Fatalf("Artist: %v", err)
	}
	if a.Name == "" {
		t.Error("no artist name")
	}
	if a.Subscribers == "" {
		t.Error("no subscriber count")
	}
	if a.ThumbURL == "" {
		t.Error("no artist image")
	}
	t.Logf("%s — %s subscribers, %d songs, %d releases", a.Name, a.Subscribers, len(a.TopSongs), len(a.Albums))

	if len(a.Albums) < 5 {
		t.Errorf("only %d releases; the carousels should give far more", len(a.Albums))
	}
	for _, al := range a.Albums {
		if al.Title == "" {
			t.Errorf("release with no title: %+v", al)
		}
		// Every id here is handed to AlbumTracks, which only opens MPREb.
		if len(al.BrowseID) < 5 || al.BrowseID[:5] != "MPREb" {
			t.Errorf("release %q has id %q, which AlbumTracks cannot open", al.Title, al.BrowseID)
		}
	}

	// The point of following the playlist rather than reading the shelf.
	if len(a.TopSongs) < 20 {
		t.Errorf("got %d top songs; the shelf alone gives 5, so the playlist was not followed", len(a.TopSongs))
	}
	withDuration := 0
	for _, s := range a.TopSongs {
		if s.VideoID == "" {
			t.Errorf("song %q has no videoId, so it cannot be played", s.Title)
		}
		if s.Duration > 0 {
			withDuration++
		}
	}
	if withDuration < len(a.TopSongs)/2 {
		t.Errorf("only %d of %d songs carry a duration", withDuration, len(a.TopSongs))
	}
}

// TestLiveArtistDiscographyOpens: the ids really are openable, not just
// well-shaped.
func TestLiveArtistDiscographyOpens(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	a, err := Artist("UCRr1xG_2WIDs18a6cIiCxeA")
	if err != nil || len(a.Albums) == 0 {
		t.Fatalf("Artist: %v (%d albums)", err, len(a.Albums))
	}
	alb, err := AlbumTracks(a.Albums[0].BrowseID)
	if err != nil {
		t.Fatalf("AlbumTracks(%q): %v", a.Albums[0].BrowseID, err)
	}
	if len(alb.Tracks) == 0 {
		t.Errorf("%q opened with no tracks", alb.Title)
	}
	t.Logf("opened %q — %d tracks", alb.Title, len(alb.Tracks))
}

// TestLiveArtistSongColumns: an artist's top songs carry the play count
// in column 2 and the album in column 3. Reading 2 as the album put
// "1.2B plays" where the release name belongs — visible on the player
// bar under the track title.
func TestLiveArtistSongColumns(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	a, err := Artist("UCRr1xG_2WIDs18a6cIiCxeA")
	if err != nil || len(a.TopSongs) == 0 {
		t.Fatalf("Artist: %v (%d songs)", err, len(a.TopSongs))
	}
	withPlays, albumLooksLikePlays := 0, 0
	for _, s := range a.TopSongs {
		if s.Plays != "" {
			withPlays++
		}
		if strings.HasSuffix(s.Album, " plays") {
			albumLooksLikePlays++
		}
	}
	t.Logf("%d of %d songs carry a play count; first: plays=%q album=%q",
		withPlays, len(a.TopSongs), a.TopSongs[0].Plays, a.TopSongs[0].Album)
	if withPlays == 0 {
		t.Error("no song carried a play count")
	}
	if albumLooksLikePlays > 0 {
		t.Errorf("%d songs have a play count sitting in the album field", albumLooksLikePlays)
	}
}
