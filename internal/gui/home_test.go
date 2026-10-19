package gui

import (
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

// Songs on the home carry a playable track; other cards carry the id
// their page opens by.
func TestHomeCards(t *testing.T) {
	a := &App{settings: settings.Defaults(), q: queue.New(), home: func(params string) (ytmusic.Home, error) {
		if params != "relax" {
			t.Errorf("params %q", params)
		}
		return ytmusic.Home{
			Moods: []ytmusic.Mood{{Title: "Relax", Params: "relax"}},
			Shelves: []ytmusic.HomeShelf{{Title: "Mixed", Items: []ytmusic.HomeItem{
				{Kind: "album", ID: "MPREb_x", Title: "Discovery"},
				{Kind: "song", ID: "abcdefghijk", Title: "One More Time", Artist: "Daft Punk"},
			}}},
		}, nil
	}}
	v, err := a.Home("relax")
	if err != nil || len(v.Moods) != 1 || len(v.Shelves) != 1 {
		t.Fatalf("%+v, %v", v, err)
	}
	album, song := v.Shelves[0].Cards[0], v.Shelves[0].Cards[1]
	if album.Track != nil || album.ID != "MPREb_x" {
		t.Errorf("album card %+v", album)
	}
	if song.Track == nil || song.Track.URL != "https://music.youtube.com/watch?v=abcdefghijk" || song.Track.Artist != "Daft Punk" {
		t.Errorf("song card %+v", song.Track)
	}
}

// A playlist opens by its page id, VL and all.
func TestPlaylistDropsVL(t *testing.T) {
	a := &App{settings: settings.Defaults(), playlist: func(ref search.PlaylistRef) (string, []search.Result, int, error) {
		if ref.ID != "RDCLAK5uy_x" || ref.Source != "youtube" {
			t.Errorf("ref %+v", ref)
		}
		return "Chill", []search.Result{{ID: "a", Title: "A", Duration: 60}, {ID: "b", Title: "B", Duration: 30}}, 0, nil
	}}
	p, err := a.Playlist("VLRDCLAK5uy_x")
	if err != nil || p.Title != "Chill" || len(p.Tracks) != 2 || p.Seconds != 90 {
		t.Fatalf("%+v, %v", p, err)
	}
}
