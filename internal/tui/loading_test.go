package tui

import (
	"image"
	"strings"
	"testing"

	"ytmgo/internal/coverart"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

// artistOnScreen is a settled artist page with one of its songs under
// the cursor, built without touching the network.
func artistOnScreen(t *testing.T) Model {
	t.Helper()
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.openArtist = &ytmusic.ArtistPage{
		BrowseID: "UCRr1xG_2WIDs18a6cIiCxeA", Name: "Daft Punk",
		Albums: []ytmusic.Album{{BrowseID: "MPREb_K8qWMWVqXGi", Title: "Discovery"}},
	}
	m.artistSongs = []search.Result{{
		ID: "65-UZY0mN3o", Title: "One More Time", Uploader: "Someone Else",
		ArtistBrowseID: "UC_someone_else", AlbumBrowseID: "MPREb_other",
	}}
	m.searchCursor = 0
	return m
}

// Both waits look the same and both name what they are fetching. They
// used to be opposites: an album blanked the panel for a bare "Loading
// album…", and an artist showed nothing at all — the previous artist's
// songs stayed put for the whole fetch, so the key looked dead.
func TestBothBrowseLoadsLookTheSame(t *testing.T) {
	artist := artistOnScreen(t)

	loadingArtist := artist
	loadingArtist.openArtistOfSelected()

	loadingAlbum := artist
	loadingAlbum.openAlbumOfSelected()

	for _, tc := range []struct {
		name string
		m    Model
		want string
	}{
		{"artist", loadingArtist, "Someone Else"},
		{"album", loadingAlbum, "album"},
	} {
		body := tc.m.renderStreamList(80, 20)
		if !strings.Contains(body, "Loading") {
			t.Errorf("%s: the panel does not say it is loading:\n%s", tc.name, body)
		}
		if !strings.Contains(body, tc.want) {
			t.Errorf("%s: the wait does not name what it is fetching (%q):\n%s", tc.name, tc.want, body)
		}
		// The stale list must be gone — that was the artist bug.
		if strings.Contains(body, "One More Time") {
			t.Errorf("%s: the previous page is still on screen while loading:\n%s", tc.name, body)
		}
	}

	// And the titles match, rather than one keeping a title whose keys
	// do nothing yet.
	at := strings.Split(loadingArtist.renderPanels(), "\n")[0]
	bt := strings.Split(loadingAlbum.renderPanels(), "\n")[0]
	if at != bt {
		t.Errorf("the two loads carry different titles:\n%s\n%s", at, bt)
	}
	if !strings.Contains(at, "LOADING") {
		t.Errorf("the title does not say a load is in progress:\n%s", at)
	}
}

// esc during a load cancels it. Without the sequence bump the response
// arrived a second later and opened the page anyway.
func TestEscCancelsABrowseLoad(t *testing.T) {
	base := artistOnScreen(t)

	// An artist load, cancelled: the late response is dropped and the
	// artist already open is untouched.
	m := base
	m.openArtistOfSelected()
	if !m.isLoadingArtist {
		t.Fatal("A did not start a fetch")
	}
	late := ArtistLoadedMsg{
		Artist: ytmusic.ArtistPage{Name: "Somebody Else"},
		Songs:  []search.Result{{Title: "x"}},
		Seq:    m.artistSeq,
	}
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.isLoadingArtist {
		t.Error("esc left the fetch running")
	}
	nm, _ = m.handleArtistLoaded(late)
	if got := nm.(Model).openArtist.Name; got != "Daft Punk" {
		t.Errorf("the cancelled fetch opened %q anyway", got)
	}

	// An album load, cancelled the same way.
	m2 := base
	m2.openAlbumOfSelected()
	if !m2.isLoadingAlbum {
		t.Fatal("a did not start a fetch")
	}
	lateAlb := AlbumTracksMsg{
		Album:  ytmusic.Album{BrowseID: "MPREb_other", Title: "Some Album"},
		Tracks: []search.Result{{Title: "y"}},
		Seq:    m2.albumSeq,
	}
	nm, _ = m2.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m2 = nm.(Model)
	if m2.isLoadingAlbum {
		t.Error("esc left the album fetch running")
	}
	nm, _ = m2.handleAlbumTracks(lateAlb)
	if got := nm.(Model); got.openAlbum != nil {
		t.Errorf("the cancelled fetch opened album %q anyway", got.openAlbum.Title)
	}
}

// The cover is a kitty overlay the terminal keeps until something
// deletes it. The wait replaces the strip that draws it, so starting a
// fetch has to owe the terminal that delete — otherwise the old
// artist's photo hangs over the loading message, hiding it.
func TestLoadingClearsTheStaleCover(t *testing.T) {
	m := artistOnScreen(t)
	m.albumArtImg = image.NewRGBA(image.Rect(0, 0, 8, 8))
	m.albumArtURL = "https://example.invalid/old.jpg"
	if !m.albumArtOnScreen() {
		t.Fatal("the settled artist page is not showing its cover")
	}

	// A with the cursor in the browse list flips to the releases, so
	// the artist case is the one that actually fetches: a queue track
	// by somebody else, which is where this was seen.
	fromQueue := m
	fromQueue.queue.Clear()
	fromQueue.queue.Add(queue.Track{
		ID: "65-UZY0mN3o", Title: "CHE.R.RY", Artist: "YUI",
		ArtistBrowseID: "UC_yui", AlbumBrowseID: "MPREb_GPU0kIvZxOF",
	})
	fromQueue.queueCursor = 0
	fromQueue.activePanel = PanelQueue

	for _, tc := range []struct {
		name string
		key  rune
		from Model
	}{{"artist", 'A', fromQueue}, {"album", 'a', m}} {
		nm, _ := tc.from.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		after := nm.(Model)
		if after.albumArtOnScreen() {
			t.Errorf("%s: the cover still counts as on screen while loading", tc.name)
		}
		if after.albumArtClearN == 0 {
			t.Errorf("%s: no delete was scheduled, so the old cover stays over the wait", tc.name)
		}
		if after.albumArtSendN != 0 {
			t.Errorf("%s: the cover is still queued for transmit while loading", tc.name)
		}
		// And the escape that removes it is emitted, on a terminal
		// that has images at all.
		if esc := after.clearCoverImage(); coverartKitty() && !strings.Contains(esc, "\x1b_G") {
			t.Errorf("%s: no kitty delete emitted: %q", tc.name, esc)
		}
	}

	// When the page arrives, the cover is owed a transmit again.
	nm, _ := fromQueue.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	loading := nm.(Model)
	done := ArtistLoadedMsg{
		Artist: ytmusic.ArtistPage{Name: "Someone Else", ThumbURL: m.albumArtURL},
		Songs:  []search.Result{{Title: "a song"}},
		Seq:    loading.artistSeq,
	}
	nm, _ = loading.Update(done)
	if settled := nm.(Model); !settled.albumArtOnScreen() {
		t.Error("the cover did not come back once the page arrived")
	} else if settled.albumArtSendN == 0 {
		t.Error("the cover is on screen again but was never transmitted")
	}
}

// coverartKitty reports whether this terminal draws images at all; the
// delete escape is empty without one.
func coverartKitty() bool { return coverart.KittySupported() }
