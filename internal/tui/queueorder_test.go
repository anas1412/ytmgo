package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
)

func queueTitles(m Model) string {
	order, _ := m.queueOrder()
	ts := m.queue.Tracks()
	var out []string
	for _, i := range order {
		out = append(out, ts[i].Title)
	}
	return strings.Join(out, " ")
}

// The panel lists the track playing, then what comes next, then what
// has played, newest first; the cursor's place maps to the right track.
func TestQueuePanelOrder(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.queue.Clear()
	for _, s := range []string{"a", "b", "c", "d"} {
		m.queue.Add(queue.Track{ID: s, Title: s})
	}
	m.queue.SetCurrentIndex(2)
	if got := queueTitles(m); got != "c d b a" {
		t.Fatalf("panel %q, want c d b a", got)
	}
	m.queueCursor = 1
	if idx := m.selectedQueueIndex(); idx != 3 {
		t.Errorf("cursor on d is queue index %d, want 3", idx)
	}
	if pos := m.queuePlaceOf(1); pos != 2 {
		t.Errorf("b sits at place %d, want 2", pos)
	}
	// The drawn panel says the same, with ▶ on c.
	if out := m.renderQueue(60, 20); !strings.Contains(out, "▶ c") || strings.Index(out, "▶ c") > strings.Index(out, "1. d") {
		t.Errorf("panel drawn as:\n%s", out)
	}
}

// A song added while one plays comes next, after those added before.
func TestEnqueueGoesNext(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.queue.Clear()
	m.queue.Add(queue.Track{ID: "a", Title: "a"})
	m.queue.Add(queue.Track{ID: "b", Title: "b"})
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying // so nothing starts
	m.enqueueAndMaybePlay(queue.Track{ID: "x", Title: "x"})
	m.enqueueAndMaybePlay(queue.Track{ID: "y", Title: "y"})
	if got := queueTitles(m); got != "a x y b" {
		t.Errorf("panel %q, want a x y b", got)
	}
}

// ctrl+↑ moves a song up among what comes next, and never above the
// track playing.
func TestQueueReorderStaysInNext(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage, m.activePanel = PageStream, PanelQueue
	m.queue.Clear()
	for _, s := range []string{"a", "b", "c"} {
		m.queue.Add(queue.Track{ID: s, Title: s})
	}
	m.queue.SetCurrentIndex(0)
	m.queueCursor = 2 // c
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlUp})
	m = nm.(Model)
	if got := queueTitles(m); got != "a c b" || m.queueCursor != 1 {
		t.Fatalf("after ctrl+up: %q, cursor %d", got, m.queueCursor)
	}
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyCtrlUp})
	m = nm.(Model)
	if got := queueTitles(m); got != "a c b" {
		t.Errorf("moved over the track playing: %q", got)
	}
}
