package tui

import (
	"testing"

	"ytmgo/internal/db"

	tea "github.com/charmbracelet/bubbletea"
)

func pressKeys(m Model, keys ...tea.KeyMsg) Model {
	for _, k := range keys {
		nm, _ := m.handleKey(k)
		m = nm.(Model)
	}
	return m
}

func keyRune(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

var keyEnter = tea.KeyMsg{Type: tea.KeyEnter}

// A page key pressed during "Clear queue?" used to switch page and
// leave the prompt armed, so the next Enter, on another page, cleared
// the queue anyway.
func TestPageKeyCancelsClearQueue(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageFavorites
	m.favorites = nil // Enter on an empty list does nothing
	before := m.queue.Len()

	m = pressKeys(m, keyRune('D'))
	if !m.isConfirming() {
		t.Fatal("D did not ask to clear the queue")
	}
	m = pressKeys(m, keyRune('1'))
	if m.isConfirming() || m.statusMessage != "Cancelled" {
		t.Fatalf("1 left the prompt armed (status %q)", m.statusMessage)
	}
	m = pressKeys(m, keyEnter)
	if got := m.queue.Len(); got != before {
		t.Fatalf("queue has %d tracks after D, 1, Enter; want %d", got, before)
	}
}

// Same for "Clear history?".
func TestPageKeyCancelsClearHistory(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageHistory
	m.history = []db.PlayHistoryEntry{{TrackID: "sZxzPcT1Meg", Title: "Love Story"}}

	m = pressKeys(m, keyRune('C'))
	if !m.isConfirming() {
		t.Fatal("C did not ask to clear history")
	}
	m = pressKeys(m, keyRune('1'), keyEnter)
	if len(m.history) != 1 {
		t.Fatalf("history has %d entries after C, 1, Enter; want 1", len(m.history))
	}
}

// A tab clicked with the mouse goes through switchPage, so it must not
// carry a prompt to the new page either.
func TestSwitchPageClearsConfirm(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.startConfirm(confirmClearQueue, "")
	m.switchPage(PageLibrary)
	if m.isConfirming() {
		t.Fatal("switchPage left a confirmation armed")
	}
}
