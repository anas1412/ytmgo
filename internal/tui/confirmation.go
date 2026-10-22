package tui

import (
	"os"

	"ytmgo/internal/player"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Confirmation State ─────────────────────────────────────────────

// confirmAction values
const (
	confirmNone           = ""
	confirmClearQueue     = "clear-queue"
	confirmDeleteTrack    = "delete-track"
	confirmUpdate         = "update"
	confirmClearHistory   = "clear-history"
	confirmDeletePlaylist = "delete-playlist"
	confirmQuit           = "quit"
)

// isConfirming returns true when a destructive action is awaiting confirmation.
func (m Model) isConfirming() bool {
	return m.confirmAction != confirmNone
}

// startConfirm sets the confirmation state for a destructive action.
func (m *Model) startConfirm(action, data string) {
	m.confirmAction = action
	m.confirmData = data
}

// clearConfirm resets the confirmation state.
func (m *Model) clearConfirm() {
	m.confirmAction = confirmNone
	m.confirmData = ""
}

// confirmStatus renders the styled confirmation prompt shown in the status bar.
// action and prompt are caller-styled: action is the bold white verb (e.g.
// "Delete"), and prompt is optional extra context (a track title, an update
// version) shown after it. The trailing "[Enter] yes  [Esc] no" hint is shared
// by every confirmation.
func confirmStatus(action, prompt string) string {
	bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
	enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
	enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
	escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
	escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
	msg := bullet + " " + action
	if prompt != "" {
		msg += " " + prompt
	}
	return msg + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc
}

// executeConfirmedAction runs the confirmed destructive action and returns
// the resulting model and command. Called after the user pressed the key a second time.
func (m *Model) executeConfirmedAction() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	m.clearConfirm()

	switch action {
	case confirmClearQueue:
		m.queue.Clear()
		m.queueCursor = 0
		m.playerState = player.StateStopped
		m.position = 0
		m.duration = 0
		if m.player != nil {
			m.player.Stop()
		}
		m.updatePresence()
		m.setStatus("Queue cleared")
		return m, saveQueueCmd(m.db, m.queue)

	case confirmDeleteTrack:
		tracks := m.filteredLibrary()
		if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
			t := tracks[m.libraryCursor]
			if t.FilePath != "" {
				if err := os.Remove(t.FilePath); err != nil {
					m.setStatus("Failed to delete: " + err.Error())
					return m, nil
				}
			}
			idx := -1
			for i, lt := range m.library {
				if lt.ID == t.ID {
					idx = i
					break
				}
			}
			if idx >= 0 {
				m.library = append(m.library[:idx], m.library[idx+1:]...)
			}
			m.clampLibraryOffset()
			m.setStatus("Deleted: " + t.Title)
		}
		return m, nil

	case confirmUpdate:
		m.setStatus("⬇  Updating ytmgo…")
		return m, runUpdateCmd()

	case confirmDeletePlaylist:
		// Page keys skip the confirmation, so the list may be gone.
		if m.activePage == PagePlaylists && m.plOpen == nil {
			m.deleteSelectedPlaylist()
		}
		return m, nil

	case confirmClearHistory:
		if m.db != nil {
			if err := m.db.ClearPlayHistory(); err != nil {
				m.err = err
				m.setStatus("Failed to clear history")
				return m, nil
			}
		}
		m.history = nil
		m.historyLoaded = false
		m.loadPlayHistory()
		m.setStatus("History cleared")
		return m, nil

	case confirmQuit:
		m.quitting = true
		m.Shutdown()
		return m, tea.Quit
	}

	return m, nil
}
