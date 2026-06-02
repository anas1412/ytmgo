package tui

import (
	"os"
	"strings"

	"ytmgo/internal/player"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Confirmation State ─────────────────────────────────────────────

// confirmAction values
const (
	confirmNone        = ""
	confirmClearQueue  = "clear-queue"
	confirmDeleteTrack = "delete-track"
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
		m.setStatus("Queue cleared")
		return m, nil

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
	}

	return m, nil
}

// ─── Confirmation Overlay ──────────────────────────────────────────

func (m Model) renderConfirmOverlay() string {
	// Render the base page first
	baseContent := m.renderPage()

	// Build confirmation dialog content
	var b strings.Builder
	b.WriteString(styleConfirmTitle.Render("⚠  CONFIRM"))
	b.WriteString("\n\n")

	switch m.confirmAction {
	case confirmClearQueue:
		b.WriteString(styleConfirmText.Render("Clear the entire queue?"))
		b.WriteString("\n")
		b.WriteString(styleConfirmText.Render("This cannot be undone."))
	case confirmDeleteTrack:
		title := m.confirmData
		if title == "" {
			title = "this track"
		}
		b.WriteString(styleConfirmText.Render("Delete \"" + title + "\" from disk?"))
		b.WriteString("\n")
		b.WriteString(styleConfirmText.Render("This permanently removes the file."))
	}

	b.WriteString("\n\n")
	b.WriteString(styleConfirmHint.Render("Press the same key again to confirm · Esc to cancel"))

	dialogWidth := 50
	if m.width-8 < dialogWidth {
		dialogWidth = m.width - 8
	}
	dialog := styleConfirmBorder.
		Width(dialogWidth).
		Render(b.String())

	// Center the dialog over the base content
	lines := strings.Split(baseContent, "\n")
	dialogLines := strings.Split(dialog, "\n")

	// Find center position
	centerY := len(lines)/2 - len(dialogLines)/2
	if centerY < 0 {
		centerY = 0
	}

	// Insert dialog at center
	var result []string
	for i := 0; i < len(lines); i++ {
		if i >= centerY && i-centerY < len(dialogLines) {
			dialogLine := dialogLines[i-centerY]
			// Center horizontally
			pad := (m.width - lipgloss.Width(dialogLine) - 2) / 2
			if pad < 0 {
				pad = 0
			}
			result = append(result, strings.Repeat(" ", pad)+dialogLine)
		} else {
			result = append(result, lines[i])
		}
	}

	return strings.Join(result, "\n")
}
