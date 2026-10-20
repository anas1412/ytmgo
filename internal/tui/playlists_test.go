package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ytmgo/internal/search"
)

// pressPl is pressKeys with enter and esc as the keys they are.
func pressPl(m Model, keys ...string) Model {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		}
		nm, _ := m.handleKey(msg)
		if pm, ok := nm.(*Model); ok { // what a confirmation returns
			nm = *pm
		}
		m = nm.(Model)
	}
	return m
}

// M saves a pasted playlist under its name; m adds a song to it, once;
// page 6 lists it, Enter opens it, d takes a song out, esc steps back,
// and d then deletes the playlist after asking.
func TestPlaylists(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	if m.db == nil {
		t.Skip("no database")
	}
	m.switchPage(PageStream)
	m.showingRecommendations = false
	m.results = []search.Result{{ID: "aaaaaaaaaaa", Title: "One"}, {ID: "bbbbbbbbbbb", Title: "Two"}}
	m.resultsTitle = "Road Trip"

	m = pressPl(m, "M")
	if m.plPick == nil || !m.plPick.naming || m.plPick.name.Value() != "Road Trip" {
		t.Fatalf("M: %+v", m.plPick)
	}
	m = pressPl(m, "enter")
	if m.plPick != nil || len(m.playlists) != 1 || m.playlists[0].Count != 2 {
		t.Fatalf("saved: %+v", m.playlists)
	}

	m = pressPl(m, "m", "down", "enter")
	if !strings.Contains(m.statusMessage, "Already in Road Trip") {
		t.Errorf("added twice: %q", m.statusMessage)
	}
	m.results = append(m.results, search.Result{ID: "ccccccccccc", Title: "Three"})
	m.searchCursor = 2
	m = pressPl(m, "m", "down", "enter")
	if m.playlists[0].Count != 3 {
		t.Fatalf("m: %+v, %q", m.playlists, m.statusMessage)
	}

	m = pressPl(m, "6")
	if m.activePage != PagePlaylists || !strings.Contains(m.renderPlaylists(80, 20), "Road Trip") {
		t.Fatalf("page 6: %v\n%s", m.activePage, m.renderPlaylists(80, 20))
	}
	m = pressPl(m, "enter", "d")
	if m.plOpen == nil || len(m.plOpen.Tracks) != 2 || m.plOpen.Tracks[0].Title != "Two" {
		t.Fatalf("open and d: %+v", m.plOpen)
	}
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlDown})
	m = nm.(Model)
	if m.plOpen.Tracks[1].Title != "Two" || m.plCursor != 1 {
		t.Fatalf("ctrl+down: %+v, cursor %d", m.plOpen.Tracks, m.plCursor)
	}
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlUp})
	m = nm.(Model)
	if m.plOpen.Tracks[0].Title != "Two" || m.plCursor != 0 {
		t.Fatalf("ctrl+up: %+v, cursor %d", m.plOpen.Tracks, m.plCursor)
	}
	m = pressPl(m, "esc")
	if m.plOpen != nil || m.playlists[0].Count != 2 {
		t.Fatalf("esc: %+v", m.playlists)
	}
	m = pressPl(m, "d")
	if len(m.playlists) != 1 {
		t.Fatal("deleted without asking")
	}
	m = pressPl(m, "enter")
	if len(m.playlists) != 0 {
		t.Errorf("not deleted: %+v, %q", m.playlists, m.statusMessage)
	}
}
