package tui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// The folder picker is how Library Folders are added: Enter on the row
// opens a directory browser in the settings panel, arrows and Enter walk
// into folders, and space adds the folder currently open. Typing a path
// into a one-line field is the fallback nobody should need — it is what
// this replaces.

// pickerChromeRows is what renderFolderPicker draws above the list: the
// current path, the key hints, and a blank line.
const pickerChromeRows = 3

// pickerHeight is how many entries the list can show in the panel.
func (m Model) pickerHeight() int {
	h := m.panelHeight() - 2 - pickerChromeRows
	if h < 3 {
		h = 3
	}
	return h
}

// openFolderPicker starts browsing. It opens on the last folder added,
// or home, which is where a music folder is most likely to be near.
func (m *Model) openFolderPicker() tea.Cmd {
	fp := filepicker.New()
	fp.DirAllowed = false // Enter walks into a folder; space adds it
	fp.FileAllowed = false
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.Height = m.pickerHeight()
	// Audio shows up, greyed, so a folder with music in it looks like
	// one; everything else is left out of the listing entirely.
	fp.AllowedTypes = []string{".mp3", ".m4a", ".flac", ".ogg", ".opus", ".wav"}
	// Esc belongs to cancelling the picker, not to going up a level.
	fp.KeyMap.Back = key.NewBinding(key.WithKeys("h", "left", "backspace"), key.WithHelp("backspace", "up"))
	fp.Cursor = "▶"

	start, _ := os.UserHomeDir()
	if dirs := m.settings.ResolveLibraryDirs(); len(dirs) > 0 {
		if st, err := os.Stat(dirs[len(dirs)-1]); err == nil && st.IsDir() {
			start = filepath.Dir(dirs[len(dirs)-1])
		}
	}
	fp.CurrentDirectory = start

	m.folderPicker = fp
	m.pickingFolder = true
	m.setStatus("Browse to a folder and press space to add it  (esc cancels)")
	return fp.Init()
}

// handlePickerKey owns the keyboard while the picker is open.
func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.pickingFolder = false
		m.setStatus("Cancelled")
		return m, nil
	case " ":
		dir := m.folderPicker.CurrentDirectory
		m.pickingFolder = false
		return m, m.addLibraryFolder(dir)
	}
	var cmd tea.Cmd
	m.folderPicker, cmd = m.folderPicker.Update(msg)
	return m, cmd
}

// addLibraryFolder appends dir to the setting, saves, and rescans. A
// folder already listed is left alone rather than listed twice.
func (m *Model) addLibraryFolder(dir string) tea.Cmd {
	dir = filepath.Clean(dir)
	for _, have := range m.settings.ResolveLibraryDirs() {
		if have == dir {
			m.setStatus("Already in the library: " + shortenHome(dir))
			return nil
		}
	}
	if m.settings.LibraryDirs == "" {
		m.settings.LibraryDirs = dir
	} else {
		m.settings.LibraryDirs += ", " + dir
	}
	m.setStatus("Added " + shortenHome(dir) + " — scanning…")
	return m.libraryRescanCmd()
}

// removeLastLibraryFolder drops the most recently added folder.
func (m *Model) removeLastLibraryFolder() tea.Cmd {
	dirs := m.settings.ResolveLibraryDirs()
	if len(dirs) == 0 {
		m.setStatus("No folders to remove")
		return nil
	}
	gone := dirs[len(dirs)-1]
	dirs = dirs[:len(dirs)-1]
	m.settings.LibraryDirs = strings.Join(dirs, ", ")
	m.setStatus("Removed " + shortenHome(gone) + " — rescanning…")
	return m.libraryRescanCmd()
}

// libraryRescanCmd persists the folder list and scans it again, so the
// Library page reflects a change without a restart.
func (m *Model) libraryRescanCmd() tea.Cmd {
	m.libraryLoaded = false
	return tea.Batch(saveSettingsCmd(m.db, m.settings),
		scanLibraryCmd(m.downloadDir(), m.settings.ResolveLibraryDirs(), m.db))
}

// libraryFoldersLabel is the row's value: the folders, home shortened.
func (m Model) libraryFoldersLabel() string {
	dirs := m.settings.ResolveLibraryDirs()
	if len(dirs) == 0 {
		return "none — downloads only"
	}
	short := make([]string, len(dirs))
	for i, d := range dirs {
		short[i] = shortenHome(d)
	}
	return strings.Join(short, ", ")
}

// shortenHome writes /home/you/Music as ~/Music.
func shortenHome(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			return "~"
		}
		if strings.HasPrefix(p, home+string(filepath.Separator)) {
			return "~" + p[len(home):]
		}
	}
	return p
}

// renderFolderPicker draws the browser in the settings panel: where you
// are, what the keys do, then the listing.
func (m Model) renderFolderPicker(width, height int) string {
	w := max(1, width-2)
	// Both lines sit in the list's cursor column, so they line up with
	// the entries beneath them rather than hugging the border.
	lines := []string{
		" " + styleNowTitle.Render(truncate(shortenHome(m.folderPicker.CurrentDirectory)+"/", w-1)),
		" " + styleTextDim.Render(truncate("space add this folder · enter open · backspace up · esc cancel", w-1)),
		"",
	}
	for _, l := range strings.Split(strings.TrimRight(m.folderPicker.View(), "\n"), "\n") {
		lines = append(lines, truncate(l, w))
	}
	// Exact height, like every panel: the layout and the mouse both
	// count on it.
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return padToWidth(strings.Join(lines, "\n"), w)
}
