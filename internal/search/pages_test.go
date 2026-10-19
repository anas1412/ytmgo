package search

import (
	"testing"

	"ytmgo/internal/ytmusic"
)

// An album's tracks carry the album's title and ids, so each can lead
// back to it and on to the artist, and borrow its cover when they have
// none of their own.
func TestAlbumResults(t *testing.T) {
	album := ytmusic.Album{BrowseID: "MPREb_x", Title: "Skiptracing", ArtistBrowseID: "UCa", CoverURL: "album.jpg",
		Tracks: []ytmusic.Track{{VideoID: "aaaaaaaaaaa", Title: "Homage", Plays: "4.2M plays"}, {VideoID: "bbbbbbbbbbb", Title: "Own art", CoverURL: "own.jpg"}}}
	got := AlbumResults(album)
	if len(got) != 2 || got[0].CoverURL != "album.jpg" || got[1].CoverURL != "own.jpg" {
		t.Fatalf("covers: %+v", got)
	}
	r := got[0]
	if r.Album != "Skiptracing" || r.AlbumBrowseID != "MPREb_x" || r.ArtistBrowseID != "UCa" || r.Plays != "4.2M plays" || r.URL != ytmusic.WatchURL("aaaaaaaaaaa") {
		t.Errorf("first track: %+v", r)
	}
}

// An artist's top songs carry the artist's id and fall back to their
// photo, keeping each song's own album.
func TestArtistSongs(t *testing.T) {
	a := ytmusic.ArtistPage{BrowseID: "UCa", ThumbURL: "photo.jpg",
		TopSongs: []ytmusic.Track{{VideoID: "aaaaaaaaaaa", Title: "Homage", Album: "Skiptracing", AlbumBrowseID: "MPREb_x"}}}
	got := ArtistSongs(a)
	if len(got) != 1 || got[0].ArtistBrowseID != "UCa" || got[0].CoverURL != "photo.jpg" || got[0].AlbumBrowseID != "MPREb_x" {
		t.Errorf("%+v", got)
	}
}
