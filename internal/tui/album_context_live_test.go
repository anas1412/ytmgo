package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ytmgo/internal/search"
)

// i on a track that carries no artist id — a queue row saved before
// ArtistBrowseID existed, which is what "No artist page for YUI" was —
// must still land inside that artist's page: esc steps back to the
// releases, A flips to the top songs.
func TestOpenAlbumStacksTheArtistBehindIt(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := Model{activePage: PageStream}
	m.results = []search.Result{{
		Title:         "CHE.R.RY",
		Uploader:      "YUI",
		Album:         "CAN'T BUY MY LOVE",
		AlbumBrowseID: "MPREb_GPU0kIvZxOF", // no ArtistBrowseID, on purpose
	}}

	cmd := m.openAlbumOfSelected()
	if cmd == nil {
		t.Fatal("i did nothing")
	}
	alb, ok := cmd().(AlbumTracksMsg)
	if !ok {
		t.Fatalf("i returned %T, want AlbumTracksMsg", cmd())
	}
	if alb.Album.ArtistBrowseID == "" {
		t.Fatal("the album page did not name its artist, so nothing can sit behind it")
	}
	nm, batch := m.handleAlbumTracks(alb)
	m = nm.(Model)
	if m.openAlbum == nil {
		t.Fatal("the album did not open")
	}
	if m.openArtist != nil {
		t.Fatal("the artist was set before its fetch returned")
	}
	if batch == nil {
		t.Fatal("no artist fetch was started behind the album")
	}

	// Run the batch and feed back the artist it produces.
	var art *ArtistLoadedMsg
	for _, sub := range batch().(tea.BatchMsg) {
		if a, ok := sub().(ArtistLoadedMsg); ok {
			art = &a
		}
	}
	if art == nil {
		t.Fatal("the batch produced no ArtistLoadedMsg")
	}
	if art.ForAlbum != alb.Album.BrowseID {
		t.Errorf("artist tagged for album %q, want %q", art.ForAlbum, alb.Album.BrowseID)
	}
	nm, _ = m.handleArtistLoaded(*art)
	m = nm.(Model)

	if m.openArtist == nil {
		t.Fatal("the artist did not fill in behind the album")
	}
	if m.openArtist.Name != "YUI" {
		t.Errorf("artist behind the album is %q, want YUI", m.openArtist.Name)
	}
	if m.openAlbum == nil || m.openAlbum.Title == "" {
		t.Fatal("the album on screen was replaced by the artist")
	}
	if len(m.artistSongs) == 0 {
		t.Error("A would have no top songs to flip to")
	}
	if len(m.openArtist.Albums) == 0 {
		t.Error("esc would step back to an empty release list")
	}
	t.Logf("i on a YUI track with no artist id → album %q (%d tracks) inside %s (%d songs, %d releases)",
		m.openAlbum.Title, len(m.albumTracks), m.openArtist.Name,
		len(m.artistSongs), len(m.openArtist.Albums))
}

// A on the same idless row resolves the artist through the album
// rather than reporting "No artist page for YUI".
func TestArtistKeyResolvesThroughTheAlbum(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := Model{activePage: PageStream}
	m.results = []search.Result{{
		Title: "CHE.R.RY", Uploader: "YUI", AlbumBrowseID: "MPREb_GPU0kIvZxOF",
	}}
	cmd := m.openArtistOfSelected()
	if cmd == nil {
		t.Fatalf("A refused the track: %q", m.statusMessage)
	}
	msg, ok := cmd().(ArtistLoadedMsg)
	if !ok {
		t.Fatalf("A returned %T, want ArtistLoadedMsg", cmd())
	}
	if msg.Error != nil {
		t.Fatalf("A failed: %v", msg.Error)
	}
	if msg.Artist.Name != "YUI" {
		t.Errorf("A opened %q, want YUI", msg.Artist.Name)
	}
	t.Logf("A with no artist id → %s, %d songs", msg.Artist.Name, len(msg.Songs))
}
