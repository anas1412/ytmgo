package tui

import (
	"image"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestLiveArtistPageRenders drives the whole feature with a real
// request: fetch a real artist, feed the message to the handler the
// same way Update does, and read the rendered frame. Everything except
// the keystroke itself, which the unit tests cover.
func TestLiveArtistPageRenders(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := worstCaseModel(t, 150, 40)

	// The command the I key returns.
	msg := openArtistCmd("UCRr1xG_2WIDs18a6cIiCxeA", m.artistSeq+1)()
	loaded, ok := msg.(ArtistLoadedMsg)
	if !ok {
		t.Fatalf("openArtistCmd returned %T, want ArtistLoadedMsg", msg)
	}
	if loaded.Error != nil {
		t.Fatalf("fetch: %v", loaded.Error)
	}
	m.artistSeq++
	nm, _ := m.handleArtistLoaded(loaded)
	m = nm.(Model)

	if m.openArtist == nil {
		t.Fatal("the artist page did not open")
	}
	t.Logf("%s — %d songs, %d releases", m.openArtist.Name, len(m.artistSongs), len(m.streamAlbums()))

	// Songs mode: the frame names the artist and lists their songs.
	frame := m.View()
	if !strings.Contains(frame, m.openArtist.Name) {
		t.Error("the rendered frame does not name the artist")
	}
	if !strings.Contains(frame, "TOP SONGS") {
		t.Error("the panel title does not say what is listed")
	}
	if len(m.artistSongs) == 0 {
		t.Fatal("the artist page loaded no songs")
	}
	if want := onScreenPrefix(m.artistSongs[0].Title); !strings.Contains(frame, want) {
		t.Errorf("the first song %q is not on screen", m.artistSongs[0].Title)
	}

	// Releases mode.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = nm.(Model)
	frame = m.View()
	if !strings.Contains(frame, "RELEASES") {
		t.Error("A did not switch the panel to releases")
	}
	albums := m.streamAlbums()
	if len(albums) == 0 {
		t.Fatal("the artist page loaded no releases")
	}
	if want := onScreenPrefix(albums[0].Title); !strings.Contains(frame, want) {
		t.Errorf("the first release %q is not on screen", albums[0].Title)
	}
	// The header has to survive the switch — a grid of albums with no
	// name on it says nothing about whose they are.
	if !strings.Contains(frame, m.openArtist.Name) {
		t.Error("the releases view dropped the artist's name")
	}

	// Every line still fits, in both modes.
	for _, mode := range []string{"songs", "releases"} {
		for i, line := range strings.Split(m.View(), "\n") {
			if len(line) > 0 && lineWidth(line) > 150 {
				t.Errorf("%s: line %d is %d cells wide", mode, i, lineWidth(line))
			}
		}
		nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
		m = nm.(Model)
	}
}

func lineWidth(s string) int { return lipgloss.Width(s) }

// onScreenPrefix is the leading part of a title a row is sure to show.
// A title wider than the panel is truncated with an ellipsis, so
// matching the whole string really asserts that the title is short —
// and which song an artist is most played for changes on its own. This
// failed the day Daft Punk's became "Get Lucky (Radio Edit - feat.
// Pharrell Williams and Nile Rodgers)", with the row rendering exactly
// as it should.
//
// Runes, not bytes: a title can be CJK, and half a rune matches
// nothing.
func onScreenPrefix(title string) string {
	const safe = 20 // the panel shows about fifty at this width
	r := []rune(title)
	if len(r) > safe {
		r = r[:safe]
	}
	return string(r)
}

// TestLiveArtistAlbumRoundTrip: opening a release from an artist page
// and pressing esc must land back on that artist with everything
// intact. It did not — the artist borrowed the album view's fields, and
// leaving an album nils them, so the artist came back empty.
func TestLiveArtistAlbumRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := worstCaseModel(t, 150, 40)
	msg := openArtistCmd("UCRr1xG_2WIDs18a6cIiCxeA", m.artistSeq+1)()
	m.artistSeq++
	nm, _ := m.handleArtistLoaded(msg.(ArtistLoadedMsg))
	m = nm.(Model)

	songsBefore := len(m.artistSongs)
	releases := len(m.streamAlbums())
	name := m.openArtist.Name
	if songsBefore == 0 || releases == 0 {
		t.Fatalf("artist loaded with %d songs and %d releases", songsBefore, releases)
	}

	// Switch to releases and open the first one.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = nm.(Model)
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("enter on a release did not open it")
	}
	am, ok := cmd().(AlbumTracksMsg)
	if !ok {
		t.Fatalf("opening a release produced %T", cmd())
	}
	nm, _ = m.handleAlbumTracks(am)
	m = nm.(Model)
	if m.openAlbum == nil {
		t.Fatal("the release did not open")
	}
	if m.openArtist == nil {
		t.Fatal("opening a release lost the artist underneath it")
	}
	t.Logf("opened %q from %s", m.openAlbum.Title, name)

	// esc: back to the artist, not out of everything.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.openAlbum != nil {
		t.Error("esc did not close the release")
	}
	if m.openArtist == nil {
		t.Fatal("esc left the artist page as well as the release")
	}
	if got := len(m.artistSongs); got != songsBefore {
		t.Errorf("the artist came back with %d songs, had %d", got, songsBefore)
	}
	if got := len(m.streamAlbums()); got != releases {
		t.Errorf("the artist came back with %d releases, had %d", got, releases)
	}
	if !strings.Contains(m.View(), name) {
		t.Error("the artist's name is not on screen after backing out")
	}
	// And the switch still works after the round trip.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if got := nm.(Model); len(got.streamTracks()) != songsBefore {
		t.Errorf("A after the round trip listed %d songs, want %d", len(got.streamTracks()), songsBefore)
	}
}

// TestLiveArtistCoverIsItsOwn: opening an artist must not leave the
// previous album's cover under their name, and must load theirs.
func TestLiveArtistCoverIsItsOwn(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := worstCaseModel(t, 150, 40)
	// Pretend an album was open, with its art on screen.
	m.albumArtURL = "https://example/previous-album.jpg"
	m.albumArtImg = image.NewRGBA(image.Rect(0, 0, 544, 544))

	msg := openArtistCmd("UCRr1xG_2WIDs18a6cIiCxeA", m.artistSeq+1)()
	m.artistSeq++
	nm, cmd := m.handleArtistLoaded(msg.(ArtistLoadedMsg))
	m = nm.(Model)

	if m.albumArtImg != nil {
		t.Error("the previous album's cover is still on screen under the artist")
	}
	if m.artistArtURL == "" {
		t.Fatal("the artist page carried no photo")
	}
	if cmd == nil {
		t.Fatal("no fetch was started for the artist's photo")
	}
	// And it really loads.
	art, ok := cmd().(AlbumArtLoadedMsg)
	if !ok {
		t.Fatalf("the art fetch produced %T", cmd())
	}
	if art.Err != nil {
		t.Fatalf("loading %s: %v", m.artistArtURL, art.Err)
	}
	if art.URL != m.artistArtURL {
		t.Errorf("loaded %q, want the artist's own %q", art.URL, m.artistArtURL)
	}
	if art.Seq != m.albumSeq {
		t.Errorf("art seq %d will be dropped by the handler, which wants %d", art.Seq, m.albumSeq)
	}
	b := art.Img.Bounds()
	t.Logf("artist photo %dx%d", b.Dx(), b.Dy())
}

// TestLiveAlbumFromArtistSong: pressing i on a song listed under its
// artist opens the release that song came from, and esc steps back to
// the artist. Those rows had no album id at all, so i said "no album
// page" for every one of them.
func TestLiveAlbumFromArtistSong(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	m := worstCaseModel(t, 150, 40)
	msg := openArtistCmd("UCRr1xG_2WIDs18a6cIiCxeA", m.artistSeq+1)()
	m.artistSeq++
	nm, _ := m.handleArtistLoaded(msg.(ArtistLoadedMsg))
	m = nm.(Model)
	if len(m.artistSongs) == 0 {
		t.Fatal("no songs on the artist page")
	}

	withAlbum := 0
	for _, s := range m.artistSongs {
		if s.AlbumBrowseID != "" {
			withAlbum++
		}
	}
	t.Logf("%d of %d songs link to their release", withAlbum, len(m.artistSongs))
	if withAlbum == 0 {
		t.Fatal("no song on the artist page links to its release")
	}

	// Put the cursor on one that has a release, and press a.
	m.activePanel = PanelSearch
	for i, s := range m.artistSongs {
		if s.AlbumBrowseID != "" {
			m.searchCursor = i
			break
		}
	}
	want := m.artistSongs[m.searchCursor]
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = nm.(Model)
	if cmd == nil {
		t.Fatal("a on an artist's song opened nothing")
	}
	am, ok := cmd().(AlbumTracksMsg)
	if !ok {
		t.Fatalf("a produced %T", cmd())
	}
	nm, _ = m.handleAlbumTracks(am)
	m = nm.(Model)
	if m.openAlbum == nil {
		t.Fatal("the release did not open")
	}
	t.Logf("a on %q opened %q (%d tracks)", want.Title, m.openAlbum.Title, len(m.albumTracks))
	if m.openArtist == nil {
		t.Error("opening the release lost the artist underneath")
	}

	// esc returns to the artist, not out of everything.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.openAlbum != nil || m.openArtist == nil {
		t.Error("esc did not step back to the artist")
	}
	if len(m.artistSongs) == 0 {
		t.Error("the artist's songs did not survive the round trip")
	}
}
