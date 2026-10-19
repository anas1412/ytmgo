package gui

import (
	"errors"
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

func pagesApp() *App {
	return &App{
		settings: settings.Defaults(),
		album: func(id string) (ytmusic.Album, error) {
			if id != "MPREb_x" {
				return ytmusic.Album{}, errors.New("ytmusic album: no tracklist in response")
			}
			return ytmusic.Album{BrowseID: id, Title: "Skiptracing", Artist: "Mild High Club", ArtistBrowseID: "UCa", Year: "2016",
				Tracks: []ytmusic.Track{{VideoID: "aaaaaaaaaaa", Title: "Homage", Duration: 178, Plays: "4.2M plays"}, {VideoID: "bbbbbbbbbbb", Title: "Skiptracing", Duration: 200}}}, nil
		},
		artist: func(id string) (ytmusic.ArtistPage, error) {
			return ytmusic.ArtistPage{BrowseID: id, Name: "Mild High Club", Subscribers: "512K",
				TopSongs: []ytmusic.Track{{VideoID: "aaaaaaaaaaa", Title: "Homage"}},
				Albums:   []ytmusic.Album{{BrowseID: "MPREb_x", Title: "Skiptracing", Year: "2016"}}}, nil
		},
	}
}

// An album page: its tracks in order, the whole length, and the play
// counts without the word.
func TestAlbum(t *testing.T) {
	p, err := pagesApp().Album("MPREb_x")
	if err != nil {
		t.Fatal(err)
	}
	if p.Seconds != 378 || len(p.Tracks) != 2 || p.Tracks[0].Plays != "4.2M" || p.Tracks[0].Album != "Skiptracing" || p.Tracks[0].ArtistBrowseID != "UCa" {
		t.Errorf("%+v", p)
	}
	if _, err := pagesApp().Album(""); err == nil {
		t.Error("a song with no album id opened a page")
	}
}

func TestArtistAndArtistOfAlbum(t *testing.T) {
	a := pagesApp()
	v, err := a.Artist("UCa")
	if err != nil || len(v.Songs) != 1 || len(v.Releases) != 1 || v.Releases[0].BrowseID != "MPREb_x" {
		t.Fatalf("%+v, %v", v, err)
	}
	// Through the album, for a song that never carried an artist id.
	v, err = a.ArtistOfAlbum("MPREb_x")
	if err != nil || v.BrowseID != "UCa" {
		t.Errorf("via album: %+v, %v", v, err)
	}
}

// A pasted playlist link is read as the playlist, with its title and
// what was not found; anything else is searched.
func TestSearchPlaylistLink(t *testing.T) {
	a := pagesApp()
	var gotRef search.PlaylistRef
	a.playlist = func(ref search.PlaylistRef) (string, []search.Result, int, error) {
		gotRef = ref
		return "Late night", []search.Result{{ID: "aaaaaaaaaaa", Title: "Homage"}}, 2, nil
	}
	a.search = func(string, int) ([]search.Result, error) {
		t.Error("a playlist link was searched as text")
		return nil, nil
	}
	f, err := a.Search("https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M")
	if err != nil || f.Playlist != "Late night" || f.Missed != 2 || len(f.Tracks) != 1 || gotRef.Source != "spotify" {
		t.Errorf("%+v, %v, %+v", f, err, gotRef)
	}
}

// A song saved without page ids gets them from a search for it, made
// once: the result that is the same song, though not the first.
func TestLocate(t *testing.T) {
	searches := 0
	a := &App{settings: settings.Defaults(), search: func(q string, limit int) ([]search.Result, error) {
		searches++
		return []search.Result{
			{ID: "x", Title: "Other Song", Uploader: "Djo", AlbumBrowseID: "MPREb_other", ArtistBrowseID: "UCdjo"},
			{ID: "y", Title: "End of Beginning", Uploader: "Djo", Album: "DECIDE", AlbumBrowseID: "MPREb_decide", ArtistBrowseID: "UCdjo"},
		}, nil
	}}
	song := queue.Track{ID: "y", Title: "End of Beginning", Artist: "Djo"}
	for i := 0; i < 2; i++ {
		got, err := a.Locate(song)
		if err != nil || got.AlbumBrowseID != "MPREb_decide" || got.ArtistBrowseID != "UCdjo" || got.Album != "DECIDE" {
			t.Fatalf("got %+v, %v", got, err)
		}
	}
	if searches != 1 {
		t.Errorf("searched %d times, want once", searches)
	}
	// A song that has both ids asks nobody.
	if _, err := a.Locate(queue.Track{AlbumBrowseID: "a", ArtistBrowseID: "b"}); err != nil || searches != 1 {
		t.Errorf("searched for a song with its ids: %d, %v", searches, err)
	}
}
