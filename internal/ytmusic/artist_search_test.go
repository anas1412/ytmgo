package ytmusic

import "testing"

func TestLiveSearchCarriesArtistID(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	tracks, err := Search("daft punk instant crush", 5)
	if err != nil {
		t.Fatal(err)
	}
	withID := 0
	for _, tr := range tracks {
		if tr.ArtistBrowseID != "" {
			withID++
			if tr.ArtistBrowseID[:2] != "UC" {
				t.Errorf("%q has artist id %q, not a channel id", tr.Title, tr.ArtistBrowseID)
			}
		}
	}
	t.Logf("%d of %d results carry an artist id", withID, len(tracks))
	if withID == 0 {
		t.Error("no result carried an artist id")
	}
}
