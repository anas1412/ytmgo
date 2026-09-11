package tui

import (
	"strings"
	"testing"

	"ytmgo/internal/search"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

// The footer names the two keys that get you anywhere from a track.
// They were reachable only by memory or by opening Settings.
func TestFooterShowsAlbumAndArtist(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	bar := m.renderHelpBar()
	for _, want := range []string{"album", "artist", "copy link", "quit"} {
		if !strings.Contains(bar, want) {
			t.Errorf("the footer does not offer %q:\n%s", want, bar)
		}
	}
	// The letters come from the bindings, so a rebind cannot leave the
	// footer advertising a key that does nothing.
	for _, b := range Keys.ShortHelp() {
		if len(b.Keys()) == 0 || b.Help().Key == "" {
			t.Errorf("footer entry %q has no key behind it", b.Help().Desc)
		}
	}
	if got := Keys.ShortHelp()[0].Keys()[0]; got != Keys.Album.Keys()[0] {
		t.Errorf("the footer's album entry is bound to %q, not %q", got, Keys.Album.Keys()[0])
	}
	t.Logf("footer: %s", strings.TrimSpace(bar))
}

// One album key, doing the album thing available where the user is
// standing: opening the release outside one, queuing the lot inside.
// It used to be i to open and a to queue, with a doing nothing at all
// outside an album.
func TestAlbumKeyOpensOutsideAndQueuesInside(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.results = []search.Result{{
		Title: "CHE.R.RY", Uploader: "YUI", AlbumBrowseID: "MPREb_GPU0kIvZxOF",
	}}

	// Outside an album: a asks for the release.
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd == nil {
		t.Fatalf("a on a search result did nothing (status: %q)", nm.(Model).statusMessage)
	}

	// Inside one: a queues every track, and says so.
	m.openAlbum = &ytmusic.Album{BrowseID: "MPREb_GPU0kIvZxOF", Title: "CAN'T BUY MY LOVE"}
	m.albumTracks = []search.Result{
		{ID: "a1", Title: "One"}, {ID: "a2", Title: "Two"}, {ID: "a3", Title: "Three"},
	}
	before := m.queue.Len()
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := nm.(Model)
	if added := got.queue.Len() - before; added != 3 {
		t.Errorf("a inside an album queued %d tracks, want 3", added)
	}

	// And the album view says what the key does there, in words, in
	// both places the user is looking: the panel's title row and the
	// header strip over the tracklist.
	rows := strings.Split(got.renderPanels(), "\n")
	if !strings.Contains(rows[0], "queue all songs") {
		t.Errorf("the ALBUM panel title does not say a queues all the songs:\n%s", rows[0])
	}
	strip := strings.Join(got.browseStrip(120, got.albumTracks), "\n")
	if !strings.Contains(strip, "queue all songs") {
		t.Errorf("the album strip does not say a queues all the songs:\n%s", strip)
	}
}
