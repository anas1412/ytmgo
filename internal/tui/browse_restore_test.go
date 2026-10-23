package tui

import (
	"testing"

	"ytmgo/internal/search"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

func toModel(nm tea.Model) Model {
	if m, ok := nm.(Model); ok {
		return m
	}
	return *nm.(*Model)
}

// TestBrowseRestoreNested verifies that opening an artist from results (row 6)
// and then opening one of their releases (row 2) does not overwrite the
// stashed results cursor. Stepping back with esc from the release to the
// artist and then esc from the artist to results must restore row 6, not row 2.
func TestBrowseRestoreNested(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.searchCursor = 6
	m.searchOffset = 0

	// Leaving results to open artist:
	m.saveBrowseRestore()

	// Simulate artist page loaded:
	artist := ytmusic.ArtistPage{
		Name: "Test Artist",
		Albums: []ytmusic.Album{
			{Title: "Album 0", BrowseID: "alb0"},
			{Title: "Album 1", BrowseID: "alb1"},
			{Title: "Album 2", BrowseID: "alb2"},
		},
	}
	m.openArtist = &artist
	m.artistShowsAlbums = true
	m.albumMode = true
	m.searchCursor = 2 // cursor is on row 2 (Album 2) in artist's releases

	// Opening Album 2 from the artist page calls saveBrowseRestore:
	m.saveBrowseRestore()

	// Ensure preBrowseCursor was NOT overwritten with 2:
	if m.preBrowseCursor != 6 {
		t.Fatalf("saveBrowseRestore overwrote preBrowseCursor with %d, want 6", m.preBrowseCursor)
	}

	// Album loads:
	album := artist.Albums[2]
	m.openAlbum = &album
	m.albumTracks = []search.Result{
		{ID: "track1", Title: "Track 1"},
	}
	m.searchCursor = 0

	// First Esc: leaves album view back to the artist page
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = toModel(nm)

	if m.openAlbum != nil {
		t.Fatalf("first Esc should have closed the album")
	}
	if m.openArtist == nil {
		t.Fatalf("first Esc should keep the artist open")
	}

	// Second Esc: leaves artist view back to results
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = toModel(nm)

	if m.openArtist != nil {
		t.Fatalf("second Esc should have closed the artist")
	}
	if m.searchCursor != 6 {
		t.Fatalf("searchCursor after nested browse esc-esc = %d, want 6", m.searchCursor)
	}
}

// TestBrowseRestoreClampsOffset verifies that restoreBrowseRestore calls
// clampSearchOffset so the restored cursor is visible even if the visible
// item count changed or offset would leave it scrolled off screen.
func TestBrowseRestoreClampsOffset(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.searchCursor = 15
	m.searchOffset = 0 // cursor 15 would be off screen if vis < 16
	m.saveBrowseRestore()

	// Simulate being in an artist
	artist := ytmusic.ArtistPage{Name: "Artist"}
	m.openArtist = &artist

	// Set small height so visible items is small
	m.height = 15
	m.restoreBrowseRestore()

	if m.searchCursor != 15 {
		t.Fatalf("searchCursor = %d, want 15", m.searchCursor)
	}
	vis := m.visibleItems()
	if m.searchCursor < m.searchOffset || m.searchCursor >= m.searchOffset+vis {
		t.Errorf("searchCursor %d not within [offset=%d, offset+vis=%d)",
			m.searchCursor, m.searchOffset, m.searchOffset+vis)
	}
}

// TestCtrlCQuitsImmediately verifies that ctrl+c always shuts down and quits
// without confirmation.
func TestCtrlCQuitsImmediately(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c should return tea.Quit cmd")
	}
	if !toModel(nm).quitting {
		t.Fatal("ctrl+c should set quitting = true")
	}
}

// TestCtrlCQuitsWhileConfirming verifies that ctrl+c quits even when a
// confirmation dialog is active.
func TestCtrlCQuitsWhileConfirming(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	// Press 'q' to trigger quit confirmation
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = toModel(nm)
	if !m.isConfirming() {
		t.Fatal("q should enter confirmation mode")
	}

	// Press ctrl+c while confirmation is active
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c while confirming should return tea.Quit cmd")
	}
	if !toModel(nm).quitting {
		t.Fatal("ctrl+c while confirming should set quitting = true")
	}
}

// TestQuitConfirmationWithQ verifies that 'q' requests confirmation, Enter
// confirms, and Esc cancels.
func TestQuitConfirmationWithQ(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	// Press 'q'
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = toModel(nm)
	if cmd != nil {
		t.Fatal("q should not quit immediately")
	}
	if !m.isConfirming() || m.confirmAction != confirmQuit {
		t.Fatal("q should prompt for quit confirmation")
	}

	// Press 'esc' to cancel
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = toModel(nm)
	if m.isConfirming() {
		t.Fatal("esc should cancel confirmation")
	}

	// Press 'q' then 'enter' to confirm
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = toModel(nm)
	nm, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should quit when confirming quit")
	}
	if !toModel(nm).quitting {
		t.Fatal("quitting should be true after Enter")
	}
}
