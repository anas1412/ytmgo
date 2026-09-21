package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

func pickerModel(t *testing.T) Model {
	t.Helper()
	m := worstCaseModel(t, 150, 40)
	m.switchPage(PageSettings)
	for i, d := range settingDefs {
		if d.label == "Library Folders" {
			m.settingsCursor = i
		}
	}
	return m
}

func press(t *testing.T, m Model, k string) (Model, tea.Cmd) {
	t.Helper()
	var msg tea.KeyMsg
	switch k {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEscape}
	case "backspace":
		msg = tea.KeyMsg{Type: tea.KeyBackspace}
	case " ":
		msg = tea.KeyMsg{Type: tea.KeySpace}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
	nm, cmd := m.handleKey(msg)
	return nm.(Model), cmd
}

// Enter on the row opens the browser; space adds the folder it is in,
// saves, rescans and closes; the same folder is never added twice.
func TestPickerAddsTheOpenFolder(t *testing.T) {
	m := pickerModel(t)
	m, cmd := press(t, m, "enter")
	if !m.pickingFolder || cmd == nil {
		t.Fatal("Enter on Library Folders did not open the picker")
	}
	dir := t.TempDir()
	m.folderPicker.CurrentDirectory = dir

	m, cmd = press(t, m, " ")
	if m.pickingFolder {
		t.Fatal("space left the picker open")
	}
	if cmd == nil {
		t.Fatal("adding a folder did not save and rescan")
	}
	if got := m.settings.ResolveLibraryDirs(); len(got) != 1 || got[0] != filepath.Clean(dir) {
		t.Fatalf("library dirs = %v, want [%s]", got, dir)
	}

	// Again, same folder: no duplicate.
	m, _ = press(t, m, "enter")
	m.folderPicker.CurrentDirectory = dir
	m, _ = press(t, m, " ")
	if got := m.settings.ResolveLibraryDirs(); len(got) != 1 {
		t.Fatalf("same folder added twice: %v", got)
	}
}

// Esc closes without touching the setting.
func TestPickerEscCancels(t *testing.T) {
	m := pickerModel(t)
	m.settings.LibraryDirs = "/keep/me"
	m, _ = press(t, m, "enter")
	m.folderPicker.CurrentDirectory = t.TempDir()
	m, _ = press(t, m, "esc")
	if m.pickingFolder {
		t.Fatal("esc left the picker open")
	}
	if m.settings.LibraryDirs != "/keep/me" {
		t.Fatalf("esc changed the setting to %q", m.settings.LibraryDirs)
	}
}

// Backspace on the row removes the last folder; on an empty list it is
// harmless. Other rows ignore it.
func TestBackspaceRemovesLastFolder(t *testing.T) {
	m := pickerModel(t)
	m.settings.LibraryDirs = "/a, /b"
	m, cmd := press(t, m, "backspace")
	if m.settings.LibraryDirs != "/a" || cmd == nil {
		t.Fatalf("after backspace: %q (cmd nil=%v), want /a with a rescan", m.settings.LibraryDirs, cmd == nil)
	}
	m, _ = press(t, m, "backspace")
	m, cmd = press(t, m, "backspace")
	if m.settings.LibraryDirs != "" || cmd != nil {
		t.Fatal("backspace on an empty list did something")
	}
	m.settingsCursor = 0 // Playback Mode
	before := m.settings.PlaybackMode
	m, _ = press(t, m, "backspace")
	if m.settings.PlaybackMode != before {
		t.Fatal("backspace on an unrelated row changed it")
	}
}

// With the picker open the page still renders to the exact size, and
// no line runs past the terminal — the browser lists whatever names a
// folder happens to hold.
func TestPickerKeepsGeometry(t *testing.T) {
	for _, size := range [][2]int{{150, 40}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.switchPage(PageSettings)
		m.openFolderPicker()
		m.folderPicker.CurrentDirectory = t.TempDir()
		lines := strings.Split(m.View(), "\n")
		if len(lines) != h {
			t.Errorf("%dx%d: picker page rendered %d lines, want %d", w, h, len(lines), h)
		}
		for i, line := range lines {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("%dx%d: line %d is %d cells wide (max %d)", w, h, i, lw, w)
			}
		}
		if !strings.Contains(m.View(), "ADD A LIBRARY FOLDER") {
			t.Errorf("%dx%d: the panel does not say it is picking a folder", w, h)
		}
	}
}
