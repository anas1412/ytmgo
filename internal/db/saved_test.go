package db

import "testing"

// Kinds stay apart; newest first; saving again updates, not doubles;
// unsaving forgets.
func TestSaved(t *testing.T) {
	d := openTestDB(t)
	for _, s := range []Saved{
		{Kind: "artist", ID: "UCa", Title: "A"},
		{Kind: "album", ID: "MPREb_x", Title: "X", Subtitle: "A"},
		{Kind: "artist", ID: "UCb", Title: "B"},
		{Kind: "artist", ID: "UCa", Title: "A, renamed"},
	} {
		if err := d.Save(s); err != nil {
			t.Fatal(err)
		}
	}
	artists, err := d.LoadSaved("artist")
	if err != nil || len(artists) != 2 || artists[0].ID != "UCb" || artists[1].Title != "A, renamed" {
		t.Fatalf("artists %+v, %v", artists, err)
	}
	if albums, _ := d.LoadSaved("album"); len(albums) != 1 || albums[0].Subtitle != "A" {
		t.Errorf("albums %+v", albums)
	}
	if err := d.Unsave("artist", "UCb"); err != nil {
		t.Fatal(err)
	}
	if artists, _ := d.LoadSaved("artist"); len(artists) != 1 || artists[0].ID != "UCa" {
		t.Errorf("after unsave %+v", artists)
	}
	if err := d.Save(Saved{Kind: "album"}); err == nil {
		t.Error("an item with no id was kept")
	}
}
