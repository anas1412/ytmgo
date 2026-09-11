package tui

import (
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
	t.Logf("%s — %d songs, %d releases", m.openArtist.Name, len(m.albumTracks), len(m.albums))

	// Songs mode: the frame names the artist and lists their songs.
	frame := m.View()
	if !strings.Contains(frame, m.openArtist.Name) {
		t.Error("the rendered frame does not name the artist")
	}
	if !strings.Contains(frame, "TOP SONGS") {
		t.Error("the panel title does not say what is listed")
	}
	if len(m.albumTracks) > 0 && !strings.Contains(frame, m.albumTracks[0].Title) {
		t.Errorf("the first song %q is not on screen", m.albumTracks[0].Title)
	}

	// Releases mode.
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	m = nm.(Model)
	frame = m.View()
	if !strings.Contains(frame, "RELEASES") {
		t.Error("A did not switch the panel to releases")
	}
	if len(m.albums) > 0 && !strings.Contains(frame, m.albums[0].Title) {
		t.Errorf("the first release %q is not on screen", m.albums[0].Title)
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
