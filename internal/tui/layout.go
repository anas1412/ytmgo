package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
)

// ensurePlayer creates the player if it doesn't exist yet.
func (m *Model) ensurePlayer() {
	if m.player == nil {
		m.player = player.New()
	}
}

// ensureDownloader creates the downloader if it doesn't exist yet.
func (m *Model) ensureDownloader() {
	if m.downloader == nil {
		m.downloader = downloader.New(m.downloadDir(), m.settings.CookieBrowser, m.settings.UserAgent)
	}
}

// downloadDir returns the directory where downloaded tracks are stored.
//
// Resolution order:
//  1. If the user has set a custom path via the Settings page, use it.
//  2. Otherwise, fall back to the platform-appropriate user data directory
//     (XDG_DATA_HOME/ytmgo/downloads on Linux,
//     ~/Library/Application Support/ytmgo/downloads on macOS).
//
// The legacy default value "downloads" is treated as "unset" so existing
// users get the new XDG location instead of a stray "downloads" folder
// next to the binary after upgrading.
func (m *Model) downloadDir() string {
	if dir := m.settings.DownloadDir; dir != "" && dir != "downloads" {
		os.MkdirAll(dir, 0755)
		return dir
	}
	base, err := userDataDir()
	if err != nil {
		return "downloads" // last-ditch fallback
	}
	dir := filepath.Join(base, "ytmgo", "downloads")
	os.MkdirAll(dir, 0755)
	return dir
}

// userDataDir returns the platform-appropriate base directory for app data
// (NOT configuration — for that, see settings.configPath).
//   - Linux:   $XDG_DATA_HOME, or ~/.local/share if unset
//   - macOS:   ~/Library/Application Support
func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	return filepath.Join(home, ".local", "share"), nil
}

// openInOS opens the given path in the system's default file manager
// (xdg-open on Linux/BSD, open on macOS). Uses Start, not Run, so it
// returns immediately without waiting for the launched process to exit.
func openInOS(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// panelHeight returns how many terminal lines the panel area occupies.
// Total layout: header(1) + panels(h) + player(5) + status(1) + help(1).
// lipgloss Height(N) renders N+2 lines (border adds 2), so panels(h) actually
// consumes h+2 lines. To keep the total exactly m.height, we subtract 2.
func (m Model) panelHeight() int {
	// Fixed overhead: header(1) + status(1) + player(5) + help(1) + border(2) = 10
	overhead := 10
	h := m.height - overhead
	if h < 1 {
		h = 1
	}
	return h
}

// visibleItems returns how many list rows fit in the panel.
// Must stay in sync with renderSearchResults / renderLibrary / renderQueue
// which use (height - 1) / 2 where height = panelHeight - 2.
func (m Model) visibleItems() int {
	n := (m.panelHeight() - 3) / 2
	if n < 1 {
		n = 1
	}
	return n
}

// settingsVisibleItems returns how many settings items fit in the visible area.
// Uses the same panel-height calculation as renderSettingsList.
func (m Model) settingsVisibleItems() int {
	// Panel content height minus 2 lines of overhead (scroll indicator + help text),
	// divided by 4 lines per item.
	contentH := m.panelHeight() - 3
	vis := (contentH - 2) / 4
	if vis < 1 {
		return 1
	}
	return vis
}

// ─── Clamp functions ─────────────────────────────────────────────────

// clampSearchOffset adjusts searchOffset so the cursor is visible.
func (m *Model) clampSearchOffset() {
	vis := m.visibleItems()
	if m.searchCursor < m.searchOffset {
		m.searchOffset = m.searchCursor
	}
	if m.searchCursor >= m.searchOffset+vis {
		m.searchOffset = m.searchCursor - vis + 1
	}
}

// clampLibraryOffset adjusts libraryOffset so the cursor is visible.
func (m *Model) clampLibraryOffset() {
	vis := m.visibleItems()
	n := len(m.filteredLibrary())
	if n == 0 {
		m.libraryCursor = 0
		m.libraryOffset = 0
		return
	}
	if m.libraryCursor >= n {
		m.libraryCursor = n - 1
	}
	if m.libraryCursor < 0 {
		m.libraryCursor = 0
	}
	if m.libraryCursor < m.libraryOffset {
		m.libraryOffset = m.libraryCursor
	}
	if m.libraryCursor >= m.libraryOffset+vis {
		m.libraryOffset = m.libraryCursor - vis + 1
	}
}

// clampQueueOffset adjusts queueOffset so the cursor is visible.
func (m *Model) clampQueueOffset() {
	vis := m.visibleItems()
	n := m.queue.Len()
	if n == 0 {
		m.queueCursor = 0
		m.queueOffset = 0
		return
	}
	if m.queueCursor >= n {
		m.queueCursor = n - 1
	}
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	if m.queueCursor < m.queueOffset {
		m.queueOffset = m.queueCursor
	}
	if m.queueCursor >= m.queueOffset+vis {
		m.queueOffset = m.queueCursor - vis + 1
	}
}

// clampSettingsOffset adjusts settingsOffset so the cursor is visible.
func (m *Model) clampSettingsOffset() {
	vis := m.settingsVisibleItems()
	maxItem := 8 // 9 items indexed 0-8
	if m.settingsCursor > maxItem {
		m.settingsCursor = maxItem
	}
	if m.settingsCursor < 0 {
		m.settingsCursor = 0
	}
	if m.settingsCursor < m.settingsOffset {
		m.settingsOffset = m.settingsCursor
	}
	if m.settingsCursor >= m.settingsOffset+vis {
		m.settingsOffset = m.settingsCursor - vis + 1
	}
}

// ─── Page navigation ─────────────────────────────────────────────────

// switchPage transitions to a new page and resets page-local state.
func (m *Model) switchPage(page Page) {
	m.activePage = page
	m.searchFocused = false

	switch page {
	case PageStream:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = "Search"
		m.activePanel = PanelSearch
		m.searchCursor = 0
		m.searchOffset = 0
		m.settingsEditField = false
	case PageLibrary:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = "Filter library…"
		m.activePanel = PanelSearch
		m.libraryCursor = 0
		m.libraryOffset = 0
		m.settingsEditField = false
	case PageSettings:
		m.searchInput.Blur()
		m.activePanel = PanelSearch
		m.settingsCursor = 0
		m.settingsOffset = 0
		m.settingsEditField = false
	}
}

// startSettingsEdit prepares the inline text input for editing a string setting.
func (m *Model) startSettingsEdit() {
	m.settingsEditField = true
	current := ""
	switch m.settingsCursor {
	case 4:
		current = m.settings.DownloadDir
	case 5:
		current = m.settings.CookieBrowser
	case 6:
		current = m.settings.UserAgent
	}
	m.settingsEditInput.SetValue(current)
	m.settingsEditInput.Focus()
}
