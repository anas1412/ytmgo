package db

import (
	"slices"
	"testing"

	"ytmgo/internal/queue"
)

func TestPlaylists(t *testing.T) {
	d := openTestDB(t)
	if _, err := d.CreatePlaylist("  ", nil); err == nil {
		t.Error("a playlist with no name was made")
	}
	chill, err := d.CreatePlaylist("Chill", []queue.Track{{ID: "a", DurationSec: 100, CoverURL: "https://a"}, {ID: "b", DurationSec: 50}})
	if err != nil {
		t.Fatal(err)
	}
	gym, _ := d.CreatePlaylist("Gym", nil)

	list, err := d.LoadPlaylists()
	if err != nil || len(list) != 2 || list[0].ID != gym {
		t.Fatalf("list %+v, %v", list, err)
	}
	if c := list[1]; c.Name != "Chill" || c.Count != 2 || c.Seconds != 150 || len(c.Covers) != 1 || c.Covers[0] != "https://a" {
		t.Errorf("summary %+v", c)
	}

	// Changing a playlist brings it to the top.
	if err := d.SetPlaylistTracks(chill, []queue.Track{{ID: "b"}, {ID: "a"}, {ID: "c", AlbumBrowseID: "MPREb_c"}}); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.LoadPlaylists(); list[0].ID != chill || list[0].Count != 3 {
		t.Errorf("after change %+v", list)
	}
	p, err := d.LoadPlaylist(chill)
	if err != nil || p.Name != "Chill" || len(p.Tracks) != 3 || p.Tracks[0].ID != "b" || p.Tracks[2].AlbumBrowseID != "MPREb_c" {
		t.Fatalf("playlist %+v, %v", p, err)
	}
	if empty, _ := d.LoadPlaylist(gym); empty.Tracks == nil {
		t.Error("an empty playlist's songs read as null")
	}

	if err := d.RenamePlaylist(chill, "Chill out"); err != nil {
		t.Fatal(err)
	}
	if err := d.RenamePlaylist(chill, ""); err == nil {
		t.Error("renamed to nothing")
	}
	if err := d.DeletePlaylist(gym); err != nil {
		t.Fatal(err)
	}
	if list, _ := d.LoadPlaylists(); len(list) != 1 || list[0].Name != "Chill out" {
		t.Errorf("after rename and delete %+v", list)
	}
	if err := d.SetPlaylistTracks(gym, nil); err == nil {
		t.Error("changed a deleted playlist")
	}
}

// Covers: four different ones at most, in order, none without.
func TestCovers(t *testing.T) {
	ts := []queue.Track{{CoverURL: "a"}, {CoverURL: "a"}, {}, {CoverURL: "b"}, {CoverURL: "c"}, {CoverURL: "d"}, {CoverURL: "e"}}
	if got := Covers(ts); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("got %v", got)
	}
	if got := Covers(nil); got == nil || len(got) != 0 {
		t.Errorf("empty: %#v", got)
	}
}
