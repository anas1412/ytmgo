package tui

import (
	"fmt"
	"strings"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	ver "ytmgo/internal/version"

	"github.com/charmbracelet/lipgloss"
)

// ─── Extra inline styles (beyond what styles.go provides) ──────────

var (
	// App background
	styleApp = lipgloss.NewStyle().
			Background(colorBg)

	// Search input wrapper - inline style (no border, stays on 1 line)
	styleSearchBox = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgHover).
			Padding(0, 1).
			Width(28).
			Height(1)

	styleSearchBoxFocused = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Background(colorBgHover).
				Padding(0, 1).
				Width(28).
				Height(1)

	// Panel empty state
	styleEmpty = lipgloss.NewStyle().
			Foreground(colorTextDim).
			PaddingLeft(2).
			PaddingTop(1).
			Italic(true)

	// Header search label
	styleSearchLabel = lipgloss.NewStyle().
				Foreground(colorTextMid).
				PaddingLeft(1)
)

// ─── textinput styling (referenced from model.go) ──────────────────

var (
	textinputStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBgHover)

	textinputPlaceholder = lipgloss.NewStyle().
				Foreground(colorTextDim).
				Background(colorBgHover).
				Italic(true)
)

// ─── View ──────────────────────────────────────────────────────────

// View renders the complete TUI layout.
func (m Model) View() string {
	if !m.ready {
		return "Loading…"
	}

	var view string
	switch {
	case m.isConfirming() && m.confirmAction != confirmDeleteTrack:
		view = m.renderConfirmOverlay()
	default:
		switch m.activePage {
		case PageSettings:
			view = m.renderSettingsPage()
		default:
			view = m.renderPage()
		}
	}
	return m.fillHeight(view)
}

// fillHeight pads the output to exactly m.height lines so a previous taller
// render (e.g. before a terminal shrink) is fully overwritten. Without this,
// Bubble Tea's incremental renderer leaves stale content visible at the bottom.
func (m Model) fillHeight(view string) string {
	if m.height <= 0 || m.width <= 0 {
		return view
	}
	lines := strings.Count(view, "\n") + 1
	if lines >= m.height {
		return view
	}
	blank := strings.Repeat(" ", m.width)
	return view + strings.Repeat("\n"+blank, m.height-lines)
}

// renderPage renders the base page layout (shared by Stream and Library).
func (m Model) renderPage() string {
	header := m.renderHeader()
	panels := m.renderPanels()
	status := m.renderStatus()
	player := m.renderPlayerBar()
	help := m.renderHelpBar()

	// Build the layout with optional sections
	var elements []string
	elements = append(elements, header)
	elements = append(elements, panels)
	if status != "" {
		elements = append(elements, status)
	}
	elements = append(elements, player)
	elements = append(elements, help)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// renderSettingsPage renders the settings layout with a two-column panel:
// left = settings list, right = keyboard shortcuts.
func (m Model) renderSettingsPage() string {
	header := m.renderHeader()
	panels := m.renderSettingsPanels()
	status := m.renderStatus()
	player := m.renderPlayerBar()
	help := m.renderHelpBar()

	var elements []string
	elements = append(elements, header)
	elements = append(elements, panels)
	if status != "" {
		elements = append(elements, status)
	}
	elements = append(elements, player)
	elements = append(elements, help)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// renderSettingsPanels renders the left (settings) and right (shortcuts) panels.
func (m Model) renderSettingsPanels() string {
	outerWidth := (m.width - 2) / 2
	panelWidth := outerWidth - 2
	if panelWidth < 30 {
		panelWidth = 30
	}
	panelHeight := m.panelHeight()

	// Left panel: Settings list (always focused — arrows navigate it)
	leftBorder := panelBorderFocused
	settingsTitle := stylePanelTitle.Render("SETTINGS")
	settingsContent := m.renderSettingsList(panelWidth, panelHeight-3)
	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		settingsTitle,
		settingsContent,
	)
	leftPanel = leftBorder.
		Width(panelWidth).
		Height(panelHeight - 2).
		Render(leftPanel)

	// Right panel: Keyboard shortcuts (always visible, view-only)
	rightBorder := panelBorder
	helpTitle := stylePanelTitle.Render("KEYBOARD SHORTCUTS")
	helpContent := m.renderHelpPanel(panelWidth, panelHeight-3)
	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		helpTitle,
		helpContent,
	)
	rightPanel = rightBorder.
		Width(panelWidth).
		Height(panelHeight - 2).
		Render(rightPanel)

	// Horizontal spacer between columns
	leftover := m.width - lipgloss.Width(leftPanel) - lipgloss.Width(rightPanel)
	if leftover < 1 {
		leftover = 1
	}
	spacer := strings.Repeat(" ", leftover)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, spacer, rightPanel)
}

// ─── Header ────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	// Logo
	logo := styleLogo.Render("♫ ytmgo")

	// Search input
	var searchView string
	if m.searchFocused {
		searchView = styleSearchBoxFocused.Render(m.searchInput.View())
	} else {
		searchView = styleSearchBox.Render(m.searchInput.View())
	}

	// Build page tabs (right side) with inline key hints matching [h] / [l] style
	type tabDef struct {
		key   string
		label string
	}
	tabs := []tabDef{
		{"1", "Stream"},
		{"2", "Library"},
		{"3", "Settings"},
	}
	var renderedTabs []string
	for i, t := range tabs {
		hint := styleKeyHint.Render("[" + t.key + "]")
		label := styleNavTab.Render(t.label)
		if int(m.activePage) == i {
			label = styleNavTabActive.Render(t.label)
		}
		renderedTabs = append(renderedTabs, hint+" "+label)
	}
	tabsStr := strings.Join(renderedTabs, " ")

	// Tab hint — shown inline so users discover focus cycling without
	// glancing down at the help bar.
	tabHint := styleKeyHint.Render("[tab]") + styleTextDim.Render(" cycle")
	left := lipgloss.JoinHorizontal(lipgloss.Center, logo, "   ", searchView, "  ", tabHint)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(tabsStr) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	return styleHeader.Render(
		lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, tabsStr),
	)
}

// ─── Panels (Search Results | Queue + Downloads) ───────────────────
//
// Layout: left panel (search results or library) is full height.
// Right column is split into two stacked sub-panels:
//   - top    = QUEUE  (always)
//   - bottom = DOWNLOADS (always)
// Both sub-panels are always rendered on both Stream and Library tabs.

func (m Model) renderPanels() string {
	// Dynamically calculate column widths to perfectly span the layout width
	outerWidth := (m.width - 2) / 2
	panelWidth := outerWidth - 2
	if panelWidth < 30 {
		panelWidth = 30
	}

	panelHeight := m.panelHeight()

	leftBorder := panelBorder
	rightBorder := panelBorder
	// Only highlight a panel border when the search input is NOT focused —
	// a blinking cursor in the search box and a violet panel border would
	// compete as two focus indicators pointing in different directions.
	if !m.searchFocused {
		if m.activePanel == PanelSearch {
			leftBorder = panelBorderFocused
		}
		if m.activePanel == PanelQueue {
			rightBorder = panelBorderFocused
		}
	}

	// Search panel title
	panelLabel := "SEARCH RESULTS"
	if m.activePage == PageLibrary {
		dHint := styleKeyHint.Render("[d]")
		panelLabel = "LIBRARY  " + dHint + " delete file"
		q := m.searchInput.Value()
		if q != "" {
			panelLabel = "LIBRARY  🔍 \"" + q + "\"  " + dHint + " delete file"
		}
	} else if m.showingRecommendations {
		rHint := styleKeyHint.Render("[R]")
		xHint := styleKeyHint.Render("[x]")
		panelLabel = "RECOMMENDATIONS  " + rHint + " refresh  " + xHint + " download"
	}
	searchTitle := stylePanelTitle.Render(panelLabel)

	// Search panel content.
	// lipgloss Height(N) on a bordered style renders N+2 total lines (N content
	// + top + bottom border). The content we pass in is: title (1) +
	// renderSearchResults(panelWidth, contentH). So total rendered = (1 +
	// contentH) + 2 = contentH + 3. We want total = panelHeight, so
	// contentH = panelHeight - 3.
	searchContent := m.renderSearchResults(panelWidth, panelHeight-3)
	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		searchTitle,
		searchContent,
	)
	leftPanel = leftBorder.
		Width(panelWidth).
		Height(panelHeight - 2).
		Render(leftPanel)

	// Split right panel into queue (top) and downloads (bottom).
	// Each sub-panel renders as: border-top (1) + title (1) + content (N) + border-bottom (1)
	// = N + 3 total lines. Two sub-panels: 2N + 6 total. We want total = panelHeight,
	// so N + M = panelHeight - 6 (split roughly 50/50).
	totalSubContentH := panelHeight - 6
	if totalSubContentH < 0 {
		totalSubContentH = 0
	}
	queueContentH := totalSubContentH / 2
	downloadsContentH := totalSubContentH - queueContentH

	// Queue sub-panel (top of right column)
	dHint := styleKeyHint.Render("[d]")
	dCapHint := styleKeyHint.Render("[D]")
	reorderHint := styleKeyHint.Render("[ctrl+↑↓]")
	queueTitle := fmt.Sprintf("QUEUE  [%d]  %s remove  %s clear  %s reorder",
		m.queue.Len(), dHint, dCapHint, reorderHint)
	queueTitleStyled := stylePanelTitle.Render(queueTitle)
	queueContent := m.renderQueue(panelWidth-2, queueContentH)
	queuePanel := lipgloss.JoinVertical(lipgloss.Top,
		queueTitleStyled,
		queueContent,
	)
	queuePanel = rightBorder.
		Width(panelWidth).
		Height(queueContentH).
		Render(queuePanel)

	// Downloads sub-panel (bottom of right column)
	dlCount := 0
	if m.downloader != nil {
		dlCount = len(m.downloader.Jobs())
	}
	oHint := styleKeyHint.Render("[o]")
	downloadsTitle := fmt.Sprintf("DOWNLOADS  [%d]  %s open folder", dlCount, oHint)
	downloadsTitleStyled := stylePanelTitle.Render(downloadsTitle)
	downloadsContent := m.renderDownloadQueue(panelWidth-2, downloadsContentH)
	downloadsPanel := lipgloss.JoinVertical(lipgloss.Top,
		downloadsTitleStyled,
		downloadsContent,
	)
	// Bottom sub-panel uses unfocused border (queue owns the focus)
	downloadsPanel = panelBorder.
		Width(panelWidth).
		Height(downloadsContentH).
		Render(downloadsPanel)

	// Stack the two sub-panels vertically inside the right column
	rightPanel := lipgloss.JoinVertical(lipgloss.Top, queuePanel, downloadsPanel)

	// Calculate precise spaces to spread across the horizontal plane
	leftover := m.width - lipgloss.Width(leftPanel) - lipgloss.Width(rightPanel)
	if leftover < 1 {
		leftover = 1
	}
	spacer := strings.Repeat(" ", leftover)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, spacer, rightPanel)
}

// ─── Search Results List ───────────────────────────────────────────

func (m Model) renderSearchResults(width, height int) string {
	if m.activePage == PageLibrary {
		return m.renderLibrary(width, height)
	}
	if m.isSearching {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"⏳  Searching…",
		)
	}
	if len(m.results) == 0 {
		if m.showingRecommendations {
			return styleEmpty.Width(width - 2).Height(height).Render(
				"⏳  Loading recommendations…",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No results",
		)
	}

	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.searchOffset
	end := start + maxItems
	if end > len(m.results) {
		end = len(m.results)
	}

	for i := start; i < end; i++ {
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.searchCursor
		lines = append(lines, m.formatResultRow(i, m.results[i], width-2, isSelected))
	}

	remaining := len(m.results) - end
	if remaining > 0 {
		scrollbar := fmt.Sprintf("  ↓ %d more  [cursor %d/%d]", remaining, m.searchCursor+1, len(m.results))
		lines = append(lines,
			lipgloss.NewStyle().Foreground(colorTextDim).Italic(true).PaddingLeft(1).Render(scrollbar),
		)
	}

	// Pad each line to full width, then pad to full height — this
	// overwrites both horizontal and vertical stale content from the
	// previous frame's empty-state render ("No results", "Loading…").
	result := strings.Join(lines, "\n")
	paddedW := max(1, width-2)
	result = padToWidth(result, paddedW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(
			make([]string, height-cnt),
			"\n"+strings.Repeat(" ", paddedW),
		)
	}
	return result
}

func (m Model) renderLibrary(width, height int) string {
	tracks := m.filteredLibrary()
	if len(tracks) == 0 {
		if m.searchInput.Value() != "" {
			return styleEmpty.Width(width - 2).Height(height).Render(
				"No tracks match \"" + m.searchInput.Value() + "\"",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No downloaded tracks yet",
		)
	}

	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.libraryOffset
	end := start + maxItems
	if end > len(tracks) {
		end = len(tracks)
	}

	for i := start; i < end; i++ {
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.libraryCursor
		t := tracks[i]
		prefix := fmt.Sprintf("%d. ", i+1)
		title := t.Title
		maxTitle := width - len(prefix) - 2
		if len(title) > maxTitle && maxTitle > 3 {
			title = title[:maxTitle-1] + "…"
		}
		line := prefix + title

		artist := t.Artist
		if artist == "" {
			artist = "Unknown artist"
		}
		dur := t.Duration
		if dur == "" {
			dur = "0:00"
		}
		leftInfo := "   " + artist
		rightInfo := dur + "  ✓"
		maxLeft := width - lipgloss.Width(rightInfo) - 2
		if len(leftInfo) > maxLeft && maxLeft > 3 {
			leftInfo = leftInfo[:maxLeft-1] + "…"
		}
		spacing := width - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
		if spacing < 1 {
			spacing = 1
		}
		info := leftInfo + strings.Repeat(" ", spacing) + rightInfo

		lines = append(lines, renderListItemBlock(line, info, isSelected, false, width))
	}

	remaining := len(tracks) - end
	if remaining > 0 {
		scrollbar := fmt.Sprintf("  ↓ %d more  [cursor %d/%d]", remaining, m.libraryCursor+1, len(tracks))
		lines = append(lines,
			lipgloss.NewStyle().Foreground(colorTextDim).Italic(true).PaddingLeft(1).Render(scrollbar),
		)
	}

	// Pad each line to full width, then pad to full height — overwrites
	// stale content from the empty-state render ("No tracks match…").
	result := strings.Join(lines, "\n")
	paddedW := max(1, width-2)
	result = padToWidth(result, paddedW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(
			make([]string, height-cnt),
			"\n"+strings.Repeat(" ", paddedW),
		)
	}
	return result
}

func (m Model) formatResultRow(idx int, r search.Result, width int, isSelected bool) string {
	title := r.Title
	artist := r.Uploader
	dur := formatDuration(r.Duration)

	prefix := fmt.Sprintf("%d. ", idx+1)
	maxTitle := width - len(prefix)
	if len(title) > maxTitle && maxTitle > 3 {
		title = title[:maxTitle-1] + "…"
	}
	line := prefix + title

	// Right-align track metadata details
	leftInfo := "   " + artist
	rightInfo := dur
	maxLeft := width - len(rightInfo) - 2
	if len(leftInfo) > maxLeft && maxLeft > 3 {
		leftInfo = leftInfo[:maxLeft-1] + "…"
	}

	spacing := width - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
	if spacing < 1 {
		spacing = 1
	}
	info := leftInfo + strings.Repeat(" ", spacing) + rightInfo

	return renderListItemBlock(line, info, isSelected, false, width)
}

// ─── Queue List ────────────────────────────────────────────────────

func (m Model) renderQueue(width, height int) string {
	tracks := m.queue.Tracks()
	if len(tracks) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"Queue is empty",
		)
	}

	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.queueOffset
	end := start + maxItems
	if end > len(tracks) {
		end = len(tracks)
	}

	for i := start; i < end; i++ {
		lines = append(lines, m.formatQueueRow(i-start, tracks[i], width-2))
	}

	if len(tracks) > end {
		scrollbar := fmt.Sprintf("  ↓ %d more  [cursor %d/%d]", len(tracks)-end, m.queueCursor+1, len(tracks))
		lines = append(lines,
			lipgloss.NewStyle().Foreground(colorTextDim).Italic(true).PaddingLeft(1).Render(scrollbar),
		)
	}

	// Pad each line to full width, then pad to full height — overwrites
	// stale content from the empty-state render ("Queue is empty").
	result := strings.Join(lines, "\n")
	paddedW := max(1, width-2)
	result = padToWidth(result, paddedW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(
			make([]string, height-cnt),
			"\n"+strings.Repeat(" ", paddedW),
		)
	}
	return result
}

func (m Model) formatQueueRow(idx int, t queue.Track, width int) string {
	// idx is the relative position within the visible window (0, 1, 2…).
	// Convert to absolute for comparisons with model-level indices.
	absIdx := m.queueOffset + idx

	indicator := "  "
	isPlaying := absIdx == m.queue.CurrentIndex()
	if isPlaying {
		indicator = "▶ "
	}

	// Absolute numbering: every track shows its real position (1..N)
	// so the display number always matches the scrollbar cursor position.
	prefix := fmt.Sprintf("%s%d. ", indicator, m.queueOffset+idx+1)
	maxTitle := width - len(prefix)
	title := t.Title
	if len(title) > maxTitle && maxTitle > 3 {
		title = title[:maxTitle-1] + "…"
	}
	line := prefix + title

	dlIndicator := ""
	if t.Downloaded {
		dlIndicator = "  ✓"
	}
	leftInfo := "   " + t.Artist
	rightInfo := t.Duration + dlIndicator
	maxLeft := width - lipgloss.Width(rightInfo) - 2
	if len(leftInfo) > maxLeft && maxLeft > 3 {
		leftInfo = leftInfo[:maxLeft-1] + "…"
	}

	spacing := width - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
	if spacing < 1 {
		spacing = 1
	}
	info := leftInfo + strings.Repeat(" ", spacing) + rightInfo

	isSelected := m.activePanel == PanelQueue && absIdx == m.queueCursor
	return renderListItemBlock(line, info, isSelected, isPlaying, width)
}

func renderListItemBlock(line, info string, isSelected, isPlaying bool, width int) string {
	var bgStyle lipgloss.Style
	var titleStyle lipgloss.Style
	var infoStyle lipgloss.Style

	if isSelected {
		bgStyle = lipgloss.NewStyle().Background(colorAccent).Width(width)
		titleStyle = lipgloss.NewStyle().Foreground(colorTitle).Bold(true)
		infoStyle = lipgloss.NewStyle().Foreground(colorBgHover)
	} else {
		bgStyle = lipgloss.NewStyle().Width(width)
		if isPlaying {
			titleStyle = lipgloss.NewStyle().Foreground(colorPlaying).Bold(true)
		} else {
			titleStyle = lipgloss.NewStyle().Foreground(colorText)
		}
		infoStyle = lipgloss.NewStyle().Foreground(colorTextDim)
	}

	return bgStyle.Render(titleStyle.Render(line) + "\n" + infoStyle.Render(info))
}

// ─── Download Queue (right panel on Library page) ─────────────────

func (m Model) renderDownloadQueue(width, height int) string {
	if m.downloader == nil {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No downloads",
		)
	}
	jobs := m.downloader.Jobs()
	if len(jobs) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No downloads",
		)
	}

	var sections []string

	// Active downloads
	for _, j := range jobs {
		if j.Status != downloader.StatusDownloading {
			continue
		}
		// The worker flips status to Downloading the moment it picks
		// up a job, but yt-dlp takes a beat before emitting its first
		// `[download] X%` line. During that window j.Progress is 0,
		// which is misleading to render as "0%" — the user reads it
		// as "the download is broken." Show a spinner + "Starting…"
		// instead, so the UI communicates the actual state.
		var line string
		if j.Progress > 0 {
			bar := renderProgressBar(j.Progress, max(10, width-20))
			line = fmt.Sprintf("⬇ %s  %s  %.0f%%",
				truncate(j.Title, max(1, width-25)),
				bar,
				j.Progress,
			)
		} else {
			// Braille spinner — 6 frames, advances every 500ms tick,
			// so a full rotation every 3s.
			spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴"}
			spinner := spinnerFrames[m.tickCount%len(spinnerFrames)]
			line = fmt.Sprintf("⬇ %s  %s  Starting…",
				truncate(j.Title, max(1, width-25)),
				spinner,
			)
		}
		sections = append(sections, styleDownloadLabel.Render(line))
	}

	// Pending
	pendingHeader := false
	for _, j := range jobs {
		if j.Status != downloader.StatusPending {
			continue
		}
		if !pendingHeader {
			sections = append(sections, stylePanelTitle.Render("Pending"))
			pendingHeader = true
		}
		line := fmt.Sprintf("  ⏳ %s", truncate(j.Title, max(1, width-10)))
		sections = append(sections, styleTextDim.Render(line))
	}

	// Completed
	doneHeader := false
	for _, j := range jobs {
		if j.Status != downloader.StatusDone && j.Status != downloader.StatusSkipped {
			continue
		}
		if !doneHeader {
			sections = append(sections, stylePanelTitle.Render("Completed"))
			doneHeader = true
		}
		line := fmt.Sprintf("  ✓ %s", truncate(j.Title, max(1, width-10)))
		sections = append(sections, styleDoneLabel.Render(line))
	}

	// Failed
	failHeader := false
	for _, j := range jobs {
		if j.Status != downloader.StatusFailed {
			continue
		}
		if !failHeader {
			sections = append(sections, stylePanelTitle.Render("Failed"))
			failHeader = true
		}
		errStr := ""
		if j.Err != nil {
			errStr = j.Err.Error()
		}
		line := fmt.Sprintf("  ✗ %s", truncate(j.Title, max(1, width-10)))
		if errStr != "" {
			line += " — " + truncate(errStr, max(1, width-20))
		}
		sections = append(sections, styleErrorLabel.Render(line))
	}

	// Pad each line to full width, then pad to full height — overwrites
	// stale content from the empty-state render ("No downloads").
	result := strings.Join(sections, "\n")
	paddedW := max(1, width-2)
	result = padToWidth(result, paddedW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(
			make([]string, height-cnt),
			"\n"+strings.Repeat(" ", paddedW),
		)
	}
	return result
}

// ─── Settings List ────────────────────────────────────────────────

func (m Model) renderSettingsList(panelWidth, panelHeight int) string {
	var lines []string

	settingsItems := []struct {
		label string
		value string
		desc  string
	}{
		{"Stream Mode", boolStr(m.settings.StreamMode), "Play via URL instead of forcing download"},
		{"Auto-Download", boolStr(m.settings.AutoDownload), "Auto-download queued tracks for offline"},
		{"Default Volume", fmt.Sprintf("%d", m.settings.DefaultVolume), "0-100  (+/- adjust)"},
		{"Search Limit", fmt.Sprintf("%d", m.settings.SearchLimit), "Results per search  (+/- adjust)"},
		{"Download Dir", truncate(m.settings.DownloadDir, 40), "Path for downloaded files  (press 'o' to open)"},
		{"Cookie Browser", truncate(m.settings.CookieBrowser, 20), "Browser for YouTube cookies"},
		{"User-Agent", truncate(m.settings.UserAgent, 30), "Custom UA for yt-dlp (empty = default)"},
	}

	// Each item uses ~4 lines (label, value, desc, blank).
	// Reserve 2 lines for scroll indicator + help text at bottom.
	vis := (panelHeight - 2) / 4
	if vis < 1 {
		vis = 1
	}
	offset := m.settingsOffset
	end := offset + vis
	if end > len(settingsItems) {
		end = len(settingsItems)
	}

	innerW := max(1, panelWidth-2)

	for i, item := range settingsItems[offset:end] {
		idx := offset + i
		cursor := "  "
		if idx == m.settingsCursor && !m.settingsEditField {
			cursor = "▶ "
		}

		label := styleSettingsLabel.Render(cursor + item.label)
		value := styleSettingsValue.Render(item.value)
		desc := styleSettingsDesc.Render(item.desc)

		// Show an inline [Open] button when the cursor is on the Download
		// Dir row and we're not editing — makes the 'o' shortcut discoverable.
		if idx == 4 && !m.settingsEditField {
			openBtn := "  " + styleSettingsOpenBtn.Render("[Open]")
			value = value + openBtn
		}

		// When editing a string field, show the input
		if m.settingsEditField && idx == m.settingsCursor {
			value = styleSettingsValue.Render(m.settingsEditInput.View())
		}

		lines = append(lines, label)
		lines = append(lines, "  "+value)
		lines = append(lines, desc)
		lines = append(lines, "")
	}

	// Scroll indicator
	if end < len(settingsItems) {
		lines = append(lines, styleSettingsDesc.Render("  ↓ more items below"))
	} else if offset > 0 {
		lines = append(lines, styleSettingsDesc.Render("  ↑ more items above"))
	}

	// Help text at bottom
	lines = append(lines, styleSettingsDesc.Render("↑↓ navigate · Enter toggle/edit · Esc cancel edit · 1/2/3 switch page"))

	// Pad each line to full width and full height — overwrites any stale
	// content from prior taller frames.
	result := strings.Join(lines, "\n")
	result = padToWidth(result, innerW)
	if cnt := strings.Count(result, "\n") + 1; cnt < panelHeight {
		result += "\n" + strings.Join(
			make([]string, panelHeight-cnt),
			"\n"+strings.Repeat(" ", innerW),
		)
	}
	return result
}

// ─── Helpers ───────────────────────────────────────────────────────

func boolStr(v bool) string {
	if v {
		return styleSettingsBoolOn.Render("● ON")
	}
	return styleSettingsBoolOff.Render("○ OFF")
}

// padToWidth ensures every line of s is at least width visible characters
// wide by appending trailing spaces. This is essential for Bubble Tea's
// incremental renderer: when a previous frame had longer lines, characters
// beyond the new line's end would remain visible as stale ghost text.
func padToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if w := lipgloss.Width(line); w < width {
			lines[i] = line + strings.Repeat(" ", width-w)
		}
	}
	return strings.Join(lines, "\n")
}

func truncate(s string, maxLen int) string {
	if maxLen < 1 {
		return ""
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

var styleTextDim = lipgloss.NewStyle().Foreground(colorTextDim)

// ─── Player Bar ────────────────────────────────────────────────────

func (m Model) renderPlayerBar() string {
	var nowPlaying, progress, controls string
	innerW := m.width - 6 // box width(m.width) - doubleBorder(2) - padding(4) = content width

	nowPlayingIdx := m.queue.CurrentIndex()
	tracks := m.queue.Tracks()

	if m.queue.Len() == 0 || nowPlayingIdx < 0 || nowPlayingIdx >= len(tracks) || m.playerState == player.StateStopped {
		// ── Stopped / idle ──────────────────────────────────────
		msg := "Ready — search and add tracks"
		if m.queue.Len() > 0 {
			msg = "Playback finished"
		}
		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
			styleTime.Render("⏹"),
			"  ",
			styleTime.Render(msg),
		)

		progress = renderProgressBar(0, innerW)

		controls = m.renderControls()
	} else {
		// ── Playing track ───────────────────────────────────────
		t := tracks[nowPlayingIdx]

		trackLabel := t.Title + " — " + t.Artist
		// Title line gets the full inner width now (time appears on the progress row).
		if len(trackLabel) > innerW && innerW > 5 {
			trackLabel = trackLabel[:innerW-1] + "…"
		}

		// Smooth progress: glide the bar from the last reported position
		// using elapsed wall-clock time, so it moves continuously between
		// coarse IPC updates instead of jumping every 500ms. Bounded to
		// 2× the tick interval so a missed IPC update doesn't make the
		// bar race ahead of reality.
		displayPos := m.position
		if m.playerState == player.StatePlaying {
			elapsed := time.Since(m.lastPositionAt).Seconds()
			if elapsed < 1.0 && elapsed >= 0 {
				displayPos = m.lastPosition + elapsed
				if m.duration > 0 && displayPos > m.duration {
					displayPos = m.duration
				}
			}
		}
		currentStr := formatTime(displayPos)
		totalStr := t.Duration
		if totalStr == "" {
			totalStr = formatDuration(t.DurationSec)
		}
		// Pad the total to match the current's tabular width so the
		// time display stays column-aligned as the song progresses.
		if currentStr != "" && totalStr != "" && len(currentStr) < len(totalStr) {
			currentStr = strings.Repeat(" ", len(totalStr)-len(currentStr)) + currentStr
		}
		timeInfo := currentStr + " / " + totalStr

		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
			styleNowIndicator.Render("▶"),
			"  ",
			styleNowTitle.Render(trackLabel),
		)

		rightPart := styleTime.Render(timeInfo)
		hHint := styleKeyHint.Render("[h]")
		lHint := styleKeyHint.Render("[l]")
		barWidth := innerW - lipgloss.Width(rightPart) - lipgloss.Width(hHint) - lipgloss.Width(lHint) - 5
		if barWidth < 10 {
			barWidth = 10
		}
		displayPct := 0.0
		if m.duration > 0 {
			displayPct = (displayPos / m.duration) * 100.0
		}
		bar := renderProgressBar(displayPct, barWidth)
		progress = lipgloss.JoinHorizontal(lipgloss.Left,
			hHint, " ",
			bar,
			"  ",
			rightPart, " ",
			lHint,
		)

		controls = m.renderControls()
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		nowPlaying,
		progress,
		controls,
	)

	boxStyle := stylePlayerBox
	if m.playerState != player.StatePlaying {
		boxStyle = stylePlayerBoxStopped
	}

	return boxStyle.Render(content)
}

// renderControls renders the bottom row of the player bar.
//
// Layout (asymmetric, single separator):
//
//	[p] ⏮ Prev  [space] ▶ Play  [n] ⏭ Next  │  [s] SHFL  [r] REPT  [-] ▰▰▰▰▰ 80% [+]
//	           ↑ transport (left)            hairline  ↑ modes + volume (right, flush-right)
//
// The transport cluster is a unit of *action* (what to do next).
// The right cluster is a unit of *state* (shuffle / repeat / volume).
// One hairline rule divides them; the right cluster is flush against
// the right edge of the bar. Negative space is *intentional padding*
// between two groups, not evenly-distributed filler, so wide terminals
// don't turn the bar into three stranded clusters.
//
// Mode labels briefly flash bright for ~250ms after `s` or `r` is
// pressed, so the keypress feels acknowledged in the bar itself,
// not only in the status row.
func (m Model) renderControls() string {
	// ── Transport cluster (left, no leading separator) ─────────
	pHint := styleKeyHint.Render("[p]")
	prevLabel := styleCtrlBtn.Render("⏮ Prev")
	spaceHint := styleKeyHint.Render("[space]")
	var playLabel string
	if m.playerState == player.StatePlaying {
		playLabel = styleCtrlBtnActive.Render("⏸ Pause")
	} else {
		playLabel = styleCtrlBtn.Render("▶ Play")
	}
	nHint := styleKeyHint.Render("[n]")
	nextLabel := styleCtrlBtn.Render("⏭ Next")
	transport := lipgloss.JoinHorizontal(lipgloss.Left,
		pHint, " ", prevLabel, "  ",
		spaceHint, " ", playLabel, "  ",
		nHint, " ", nextLabel,
	)

	// ── Right cluster: modes + volume (tight unit) ─────────────
	flashActive := time.Now().Before(m.modeFlashUntil)

	var shuffleStyle lipgloss.Style
	if flashActive && m.modeFlashTarget == "shuffle" {
		shuffleStyle = styleModeFlash
	} else if m.queue.IsShuffle() {
		shuffleStyle = styleModeActive
	} else {
		shuffleStyle = styleModeInactive
	}
	sHint := styleKeyHint.Render("[s]")
	shuffleLabel := sHint + " " + shuffleStyle.Render("🔀 SHFL")

	// Repeat text follows the same state→label mapping as before, but
	// style selection routes through the flash override.
	var repeatText string
	var repeatOn bool
	switch {
	case m.queue.IsRepeat():
		repeatText, repeatOn = "🔁 ONE", true
	case m.queue.IsRepeatAll():
		repeatText, repeatOn = "🔁 ALL", true
	default:
		repeatText, repeatOn = "🔁 OFF", false
	}
	var repeatStyle lipgloss.Style
	if flashActive && m.modeFlashTarget == "repeat" {
		repeatStyle = styleModeFlash
	} else if repeatOn {
		repeatStyle = styleModeActive
	} else {
		repeatStyle = styleModeInactive
	}
	rHint := styleKeyHint.Render("[r]")
	repeatLabel := rHint + " " + repeatStyle.Render(repeatText)

	volBar := renderVolumeBar(m.volume, 8)
	volDownHint := styleKeyHint.Render("[-]")
	volUpHint := styleKeyHint.Render("[+]")
	volLabel := volDownHint + " " + volBar + styleVolumeLabel.Render(fmt.Sprintf(" %d%%", m.volume)) + " " + volUpHint

	right := lipgloss.JoinHorizontal(lipgloss.Left,
		shuffleLabel, "  ", repeatLabel, "  ", volLabel,
	)

	// ── Asymmetric composition: left flush-left, right flush-right,
	// one hairline separator centered in the gap ────────────────
	contentW := m.width - 6 // box width(m.width) - doubleBorder(2) - padding(4)
	if contentW < 20 {
		contentW = 20
	}

	sep := styleCtrlSep.Render("│")
	transportW := lipgloss.Width(transport)
	rightW := lipgloss.Width(right)
	sepW := lipgloss.Width(sep)

	gap := contentW - transportW - rightW - sepW
	if gap < 2 {
		gap = 2
	}
	leftPad := gap / 2
	rightPad := gap - leftPad

	return lipgloss.JoinHorizontal(lipgloss.Left,
		transport,
		strings.Repeat(" ", leftPad),
		sep,
		strings.Repeat(" ", rightPad),
		right,
	)
}

// ─── Help Bar ──────────────────────────────────────────────────────

func (m Model) renderHelpBar() string {
	width := m.width
	if width < 10 {
		width = 10
	}

	margin := 2
	innerWidth := width - 2*margin

	// Left: version or update status
	var left string
	switch m.updateAvailable {
	case "":
		left = styleVersion.Render("ytmgo " + ver.Version)
	case "latest":
		left = styleVersion.Render("✓ " + ver.Version + " — up to date")
	default:
		left = styleUpdateAvail.Render("⬆ " + ver.Version + " → " + m.updateAvailable)
	}

	// Right: help shortcuts
	bindings := Keys.ShortHelp()
	var parts []string
	for _, b := range bindings {
		key := styleHelpKey.Render(b.Help().Key)
		desc := styleHelp.Render(b.Help().Desc)
		parts = append(parts, fmt.Sprintf("%s %s", key, desc))
	}
	right := styleHelp.Render(strings.Join(parts, "  •  "))

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := innerWidth - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	return strings.Repeat(" ", margin) + left + strings.Repeat(" ", gap) + right + strings.Repeat(" ", margin)
}

// ─── Status ────────────────────────────────────────────────────────

func (m Model) renderStatus() string {
	if m.err != nil {
		return styleStatusErr.Render("✗ Error: " + m.err.Error())
	}
	if m.isConfirming() && m.confirmAction == confirmDeleteTrack {
		// Delete-track confirmation is fully styled inline in the message
		// itself — return it raw so the ANSI codes aren't wrapped.
		return m.statusMessage
	}
	if m.statusMessage != "" {
		return styleStatus.Render("● " + m.statusMessage)
	}
	// Nothing actionable to report — show a rotating tip so the bar is
	// never empty. Tips are dimmer than action status to keep the visual
	// hierarchy clear.
	return styleStatusIdle.Render("▸ " + m.currentTip())
}

// ─── Help Overlay ──────────────────────────────────────────────────

// renderHelpPanel renders keyboard shortcuts inside a bordered panel.
func (m Model) renderHelpPanel(panelWidth, panelHeight int) string {
	var b strings.Builder
	innerW := max(1, panelWidth-2)
	keyCol := styleHelpKey.Width(18)
	for i, group := range Keys.FullHelp() {
		if i > 0 {
			b.WriteString("\n")
		}
		for _, kb := range group {
			keys := strings.Join(kb.Keys(), ", ")
			desc := kb.Help().Desc
			b.WriteString("  " + keyCol.Render(keys) + "  " + styleHelp.Render(desc) + "\n")
		}
	}

	result := b.String()
	result = padToWidth(result, innerW)
	if cnt := strings.Count(result, "\n") + 1; cnt < panelHeight {
		result += "\n" + strings.Join(
			make([]string, panelHeight-cnt),
			"\n"+strings.Repeat(" ", innerW),
		)
	}
	return result
}

// ─── Helpers ───────────────────────────────────────────────────────

func (m Model) percentage() float64 {
	if m.duration <= 0 {
		return 0
	}
	return (m.position / m.duration) * 100.0
}

func formatDuration(secs int) string {
	if secs <= 0 {
		return "0:00"
	}
	m := secs / 60
	s := secs % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

// formatTime renders seconds as a zero-padded "MM:SS" string. Unlike
// formatDuration it always pads the minutes too, so current and total
// time share a tabular width that stays column-aligned in the player
// bar as the track progresses.
func formatTime(secs float64) string {
	if secs <= 0 {
		return "00:00"
	}
	total := int(secs)
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
