package ytmusic

import "testing"

// TestLiveRadioCarriesArtistID: recommendations come from Radio, not
// Search, and are the first list ytmgo shows. They parse through a
// different function, so the artist link has to be picked up there too.
func TestLiveRadioCarriesArtistID(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	tracks, err := Radio("m9SMT5ipbxk", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) == 0 {
		t.Fatal("radio returned nothing")
	}
	withArtist, withAlbum := 0, 0
	for _, tr := range tracks {
		if tr.ArtistBrowseID != "" {
			withArtist++
		}
		if tr.AlbumBrowseID != "" {
			withAlbum++
		}
	}
	t.Logf("%d of %d carry an artist id, %d an album id", withArtist, len(tracks), withAlbum)
	if withArtist == 0 {
		t.Error("no radio track carried an artist id — [I] would do nothing on recommendations")
	}
}
