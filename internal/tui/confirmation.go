package tui

import (
	"os"

	"ytmgo/internal/player"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Confirmation State ─────────────────────────────────────────────

// confirmAction values
const (
	confirmNone        = ""
	confirmClearQueue  = "clear-queue"
	confirmDeleteTrack = "delete-track"
	confirmUpdate      = "update"
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
		m.updateDiscordRPC()
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
	}

	return m, nil
}
