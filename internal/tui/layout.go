package tui

import (
	"os/exec"
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
		m.downloader = downloader.New(m.downloadDir(), m.settings.DownloadFormat)
	}
}

// downloadDir returns the directory where downloaded tracks are stored.
// The resolution logic lives in the settings package so the CLI
// subcommands write to the same place.
func (m *Model) downloadDir() string {
	return m.settings.ResolveDownloadDir()
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

// visibleItems returns how many list rows fit in the search-results / library
// panel. Must stay in sync with renderSearchResults / renderLibrary, which
// receive height = panelHeight - 3 and then compute
// maxItems = (height - 1) / 2 = (panelHeight - 4) / 2.
func (m Model) visibleItems() int {
	resultsH, npH := m.leftPanelSplit()
	// Closed: the panel is rendered with content height panelHeight-3,
	// showing (h-1)/2 two-line rows. Open: the results sub-panel gets
	// resultsH content rows on the same basis.
	h := m.panelHeight() - 3
	if npH > 0 {
		h = resultsH
	}
	n := (h - 1) / 2
	if n < 1 {
		n = 1
	}
	return n
}

// rightPanelSplit returns the content heights of the queue (top) and
// downloads (bottom) sub-panels of the right column. Each sub-panel
// renders as title (1) + content (N) + borders (2) = N + 3 lines. When
// there are no download jobs the downloads panel collapses to a single
// content line so the queue gets the space. renderPanels and the mouse
// hit-testing both derive from this, so they can never disagree.
func (m Model) rightPanelSplit() (queueContentH, downloadsContentH int) {
	total := m.panelHeight() - 6
	if total < 0 {
		total = 0
	}
	hasDownloads := m.downloader != nil && len(m.downloader.Jobs()) > 0
	if !hasDownloads {
		// The "No downloads" empty state renders 2 lines (top padding +
		// text); anything smaller overflows the box and shifts every row
		// below it, breaking mouse hit-testing.
		downloadsContentH = 2
		if downloadsContentH > total {
			downloadsContentH = total
		}
		return total - downloadsContentH, downloadsContentH
	}
	queueContentH = total / 2
	return queueContentH, total - queueContentH
}

// Minimum content rows each half of the left column needs to stay
// usable. Below this the now-playing panel refuses to open rather than
// crushing the results list into two or three visible items.
const (
	npMinRows      = 6
	resultsMinRows = 7
)

// leftPanelSplit returns the Height() values for the results panel and
// the now-playing sub-panel beneath it, mirroring rightPanelSplit. A
// zero second value means the column is one full-height panel — either
// the panel is closed, or the terminal is too short to split.
func (m Model) leftPanelSplit() (resultsH, npH int) {
	full := m.panelHeight() - 2
	if !m.npOn {
		return full, 0
	}
	// Two stacked sub-panels cost 6 lines of chrome, exactly as the
	// queue and downloads pair does.
	total := m.panelHeight() - 6
	if total < npMinRows+resultsMinRows {
		return full, 0
	}
	npH = total * 45 / 100
	if npH < npMinRows {
		npH = npMinRows
	}
	if total-npH < resultsMinRows {
		npH = total - resultsMinRows
	}
	return total - npH, npH
}

// npFits reports whether the terminal is tall enough to open the
// now-playing panel.
func (m Model) npFits() bool {
	return m.panelHeight()-6 >= npMinRows+resultsMinRows
}

// queueVisibleItems returns how many list rows fit in the queue
// sub-panel. Must mirror renderQueue, which receives the queue content
// height and shows (height - 1) / 2 two-line rows.
func (m Model) queueVisibleItems() int {
	qh, _ := m.rightPanelSplit()
	n := (qh - 1) / 2
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

// clampFavoritesOffset adjusts favOffset so the cursor is visible.
func (m *Model) clampFavoritesOffset() {
	vis := m.visibleItems()
	n := len(m.favorites)
	if n == 0 {
		m.favCursor = 0
		m.favOffset = 0
		return
	}
	if m.favCursor >= n {
		m.favCursor = n - 1
	}
	if m.favCursor < 0 {
		m.favCursor = 0
	}
	if m.favCursor < m.favOffset {
		m.favOffset = m.favCursor
	}
	if m.favCursor >= m.favOffset+vis {
		m.favOffset = m.favCursor - vis + 1
	}
}

// clampHistoryOffset adjusts historyOffset so the cursor is visible.
func (m *Model) clampHistoryOffset() {
	vis := m.visibleItems()
	n := len(m.history)
	if n == 0 {
		m.historyCursor = 0
		m.historyOffset = 0
		return
	}
	if m.historyCursor >= n {
		m.historyCursor = n - 1
	}
	if m.historyCursor < 0 {
		m.historyCursor = 0
	}
	if m.historyCursor < m.historyOffset {
		m.historyOffset = m.historyCursor
	}
	if m.historyCursor >= m.historyOffset+vis {
		m.historyOffset = m.historyCursor - vis + 1
	}
}

// clampQueueOffset adjusts queueOffset so the cursor is visible.
func (m *Model) clampQueueOffset() {
	vis := m.queueVisibleItems()
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
	maxItem := len(settingDefs) - 1

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

// moveCursorToEdge jumps the focused list's cursor to its first or
// last item (vim-style g / G).
func (m *Model) moveCursorToEdge(bottom bool) {
	pick := func(n int) int {
		if bottom {
			return n - 1
		}
		return 0
	}
	if m.activePage == PageSettings {
		if !m.settingsEditField {
			m.settingsCursor = pick(len(settingDefs))
			m.clampSettingsOffset()
		}
		return
	}
	if m.activePanel == PanelQueue {
		if n := m.queue.Len(); n > 0 {
			m.queueCursor = pick(n)
			m.clampQueueOffset()
		}
		return
	}
	switch m.activePage {
	case PageHistory:
		if n := len(m.history); n > 0 {
			m.historyCursor = pick(n)
			m.clampHistoryOffset()
		}
	case PageFavorites:
		if n := len(m.favorites); n > 0 {
			m.favCursor = pick(n)
			m.clampFavoritesOffset()
		}
	case PageLibrary:
		if n := len(m.filteredLibrary()); n > 0 {
			m.libraryCursor = pick(n)
			m.clampLibraryOffset()
		}
	default:
		if n := len(m.results); n > 0 {
			m.searchCursor = pick(n)
			m.clampSearchOffset()
		}
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
	case PageFavorites:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = ""
		m.activePanel = PanelSearch
		m.favCursor = 0
		m.favOffset = 0
		m.settingsEditField = false
	case PageLibrary:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = "Filter library…"
		m.activePanel = PanelSearch
		m.libraryCursor = 0
		m.libraryOffset = 0
		m.settingsEditField = false
	case PageHistory:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = ""
		m.activePanel = PanelSearch
		m.historyCursor = 0
		m.historyOffset = 0
		m.settingsEditField = false
		m.historyLoaded = false
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
	if m.settingsCursor >= 0 && m.settingsCursor < len(settingDefs) {
		if get := settingDefs[m.settingsCursor].editGet; get != nil {
			current = get(m)
		}
	}
	m.settingsEditInput.SetValue(current)
	m.settingsEditInput.Focus()
}
