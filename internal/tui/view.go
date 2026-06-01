package tui

import (
	"fmt"
	"strings"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"

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

	// If help is shown, render overlay
	if m.showHelp {
		return m.helpView()
	}

	// If confirming a destructive action, show confirmation overlay
	if m.isConfirming() {
		return m.renderConfirmOverlay()
	}

	switch m.activePage {
	case PageStream:
		return m.renderPage()
	case PageLibrary:
		return m.renderPage()
	case PageSettings:
		return m.renderSettingsPage()
	default:
		return m.renderPage()
	}
}

// renderPage renders the base page layout (shared by Stream and Library).
func (m Model) renderPage() string {
	header := m.renderHeader()
	panels := m.renderPanels()
	dlBar := m.renderDownloadBar()
	player := m.renderPlayerBar()
	help := m.renderHelpBar()
	status := m.renderStatus()

	// Build the layout with optional sections
	var elements []string
	elements = append(elements, header)
	elements = append(elements, panels)
	if dlBar != "" {
		elements = append(elements, dlBar)
	}
	elements = append(elements, player)
	if status != "" {
		elements = append(elements, status)
	}
	elements = append(elements, help)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// renderSettingsPage renders the settings layout.
func (m Model) renderSettingsPage() string {
	header := m.renderHeader()
	body := m.renderSettingsList()
	player := m.renderPlayerBar()
	help := m.renderHelpBar()
	status := m.renderStatus()

	// Wrap settings in a bordered container for visual consistency
	// Border adds 2 chars (left + right), padding adds 4 (2 each side)
	// Inner content area = panelW - 6, so set panelW = width - 6 + 2 = width - 4
	// to account for border width setting being total width
	panelW := m.width - 4
	if panelW < 40 {
		panelW = 40
	}
	body = panelBorderSettings.Width(panelW).Render(body)

	var elements []string
	elements = append(elements, header)
	elements = append(elements, body)
	elements = append(elements, player)
	if status != "" {
		elements = append(elements, status)
	}
	elements = append(elements, help)

	return lipgloss.JoinVertical(lipgloss.Left, elements...)
}

// ─── Header ────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	// Logo & Version
	logo := styleLogo.Render("♫ ytmgo")
	version := styleVersion.Render("v0.1")
	title := lipgloss.JoinHorizontal(lipgloss.Left, logo, " ", version)

	// Search input
	var searchView string
	if m.searchFocused {
		searchView = styleSearchBoxFocused.Render(m.searchInput.View())
	} else {
		searchView = styleSearchBox.Render(m.searchInput.View())
	}

	// Build page tabs (right side)
	tabs := []string{"1 Stream", "2 Library", "3 Settings"}
	var renderedTabs []string
	for i, tab := range tabs {
		if int(m.activePage) == i {
			renderedTabs = append(renderedTabs, styleNavTabActive.Render(tab))
		} else {
			renderedTabs = append(renderedTabs, styleNavTab.Render(tab))
		}
	}
	tabsStr := strings.Join(renderedTabs, " ")

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, "   ", searchView)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(tabsStr) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	return styleHeader.Render(
		lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, tabsStr),
	)
}

// ─── Panels (Search Results | Queue) ───────────────────────────────

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
	if m.activePanel == PanelSearch {
		leftBorder = panelBorderFocused
	}
	if m.activePanel == PanelQueue {
		rightBorder = panelBorderFocused
	}

	// Search panel title
	panelLabel := "SEARCH RESULTS"
	if m.activePage == PageLibrary {
		panelLabel = "LIBRARY"
		q := m.searchInput.Value()
		if q != "" {
			panelLabel = "LIBRARY  🔍 \"" + q + "\""
		}
	} else if m.showingRecommendations {
		panelLabel = "RECOMMENDATIONS"
	}
	searchTitle := stylePanelTitle.Render(panelLabel)

	// Search panel content
	searchContent := m.renderSearchResults(panelWidth, panelHeight-2)
	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		searchTitle,
		searchContent,
	)
	leftPanel = leftBorder.
		Width(panelWidth).
		Height(panelHeight).
		Render(leftPanel)

	// Queue panel title (with count)
	var queueTitle string
	if m.activePage == PageLibrary {
		dlCount := 0
		if m.downloader != nil {
			dlCount = len(m.downloader.Jobs())
		}
		queueTitle = fmt.Sprintf("DOWNLOADS  [%d]", dlCount)
	} else {
		queueTitle = fmt.Sprintf("QUEUE  [%d]", m.queue.Len())
	}
	queueTitleStyled := stylePanelTitle.Render(queueTitle)

	// Queue panel content
	queueContent := m.renderQueue(panelWidth, panelHeight-2)
	rightPanel := lipgloss.JoinVertical(lipgloss.Top,
		queueTitleStyled,
		queueContent,
	)
	rightPanel = rightBorder.
		Width(panelWidth).
		Height(panelHeight).
		Render(rightPanel)

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
		lines = append(lines, m.formatResultRow(i-start, m.results[i], width-2, isSelected))
	}

	remaining := len(m.results) - end
	if remaining > 0 {
		scrollbar := fmt.Sprintf("  ↓ %d more  [cursor %d/%d]", remaining, m.searchCursor+1, len(m.results))
		lines = append(lines,
			lipgloss.NewStyle().Foreground(colorTextDim).Italic(true).PaddingLeft(1).Render(scrollbar),
		)
	}

	return strings.Join(lines, "\n")
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

	return strings.Join(lines, "\n")
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
	if m.activePage == PageLibrary {
		return m.renderDownloadQueue(width, height)
	}

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

	return strings.Join(lines, "\n")
}

func (m Model) formatQueueRow(idx int, t queue.Track, width int) string {
	indicator := "  "
	isPlaying := idx == m.queue.CurrentIndex()
	if isPlaying {
		indicator = "▶ "
	}

	prefix := fmt.Sprintf("%s%d. ", indicator, idx+1)
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

	isSelected := m.activePanel == PanelQueue && idx == m.queueCursor
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
		bar := renderProgressBar(j.Progress, max(10, width-20))
		line := fmt.Sprintf("⬇ %s  %s  %.0f%%",
			truncate(j.Title, max(1, width-25)),
			bar,
			j.Progress,
		)
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

	return strings.Join(sections, "\n")
}

// ─── Settings List ────────────────────────────────────────────────

func (m Model) renderSettingsList() string {
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

	// Scroll window: only render items within [offset, offset+visible)
	vis := m.settingsVisibleItems()
	offset := m.settingsOffset
	end := offset + vis
	if end > len(settingsItems) {
		end = len(settingsItems)
	}

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

	return strings.Join(lines, "\n")
}

// ─── Helpers ───────────────────────────────────────────────────────

func boolStr(v bool) string {
	if v {
		return styleSettingsBoolOn.Render("● ON")
	}
	return styleSettingsBoolOff.Render("○ OFF")
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

// ─── Download Bar ──────────────────────────────────────────────────

// renderDownloadBar shows active download progress above the player bar.
// Returns empty string if no download is in progress.
func (m Model) renderDownloadBar() string {
	if !m.downloading && !m.downloadDone && m.downloadErr == nil {
		return ""
	}

	var line string
	barWidth := m.width - 30
	if barWidth < 10 {
		barWidth = 10
	}

	switch {
	case m.downloadErr != nil:
		line = fmt.Sprintf(" %s %s  ✗ %v",
			styleErrorLabel.Render("⬇"),
			truncate(m.downloadTitle, 30),
			m.downloadErr,
		)
	case m.downloadDone:
		line = fmt.Sprintf(" %s %s  ✓ done",
			styleDoneLabel.Render("⬇"),
			truncate(m.downloadTitle, 30),
		)
	default:
		bar := renderProgressBar(m.downloadPct, barWidth)
		line = fmt.Sprintf(" %s %s  %s  %.0f%%",
			styleDownloadLabel.Render("⬇"),
			truncate(m.downloadTitle, 30),
			bar,
			m.downloadPct,
		)
	}

	// Render with a separator line above
	sep := renderSeparator("═", m.width-2)
	return sep + "\n" + line
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

// ─── Player Bar ────────────────────────────────────────────────────

func (m Model) renderPlayerBar() string {
	var nowPlaying, progress, controls string
	innerW := m.width - 10 // box width(m.width-4) - doubleBorder(2) - padding(4) = content width

	nowPlayingIdx := m.queue.CurrentIndex()
	tracks := m.queue.Tracks()

	if m.queue.Len() == 0 || nowPlayingIdx < 0 || nowPlayingIdx >= len(tracks) {
		// ── Stopped / idle ──────────────────────────────────────
		msg := "Ready — search and add tracks"
		if m.queue.Len() > 0 {
			msg = "Track ended — queue is empty"
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
		maxLabel := innerW - 16 // leave room for "▶ " + time + gaps
		if len(trackLabel) > maxLabel && maxLabel > 5 {
			trackLabel = trackLabel[:maxLabel-1] + "…"
		}

		currentStr := formatDuration(int(m.position))
		totalStr := t.Duration
		if totalStr == "" {
			totalStr = formatDuration(t.DurationSec)
		}
		timeInfo := fmt.Sprintf("%s / %s", currentStr, totalStr)

		leftPart := lipgloss.JoinHorizontal(lipgloss.Left,
			styleNowIndicator.Render("▶"),
			"  ",
			styleNowTitle.Render(trackLabel),
		)
		rightPart := styleTime.Render(timeInfo)
		gap := innerW - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
		if gap < 1 {
			gap = 1
		}
		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
			leftPart,
			strings.Repeat(" ", gap),
			rightPart,
		)

		barWidth := innerW - lipgloss.Width(rightPart) - 3
		if barWidth < 10 {
			barWidth = 10
		}
		bar := renderProgressBar(m.percentage(), barWidth)
		progress = lipgloss.JoinHorizontal(lipgloss.Left,
			bar,
			"  ",
			rightPart,
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

	return boxStyle.Width(m.width - 4).Render(content)
}

func (m Model) renderControls() string {
	// Replaced standard text characters with modern crisp unicode icons
	prevBtn := styleCtrlBtn.Render("⏮")

	var playBtn string
	if m.playerState == player.StatePlaying {
		playBtn = styleCtrlBtnPlaying.Render("⏸")
	} else {
		playBtn = styleCtrlBtnActive.Render("▶")
	}

	nextBtn := styleCtrlBtn.Render("⏭")

	volBar := renderVolumeBar(m.volume, 10)
	volLabel := volBar + fmt.Sprintf(" %d%%", m.volume)

	var shuffleLabel string
	if m.queue.IsShuffle() {
		shuffleLabel = styleModeActive.Render("🔀 SHFL")
	} else {
		shuffleLabel = styleModeInactive.Render("🔀 SHFL")
	}

	var repeatLabel string
	switch {
	case m.queue.IsRepeat():
		repeatLabel = styleModeActive.Render("🔁 ONE")
	case m.queue.IsRepeatAll():
		repeatLabel = styleModeActive.Render("🔁 ALL")
	default:
		repeatLabel = styleModeInactive.Render("🔁 OFF")
	}

	sep := styleCtrlSep.Render(" │ ")

	left := lipgloss.JoinHorizontal(lipgloss.Left, prevBtn, "  ", playBtn, "  ", nextBtn)
	middle := stylePlayerCtrl.Render(volLabel)
	right := lipgloss.JoinHorizontal(lipgloss.Left, shuffleLabel, "  ", repeatLabel)

	leftWidth := lipgloss.Width(left)
	middleWidth := lipgloss.Width(middle)
	rightWidth := lipgloss.Width(right)

	totalInnerWidth := m.width - 10

	gap := (totalInnerWidth - leftWidth - middleWidth - rightWidth - 6) / 3
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		left,
		spacer,
		sep,
		spacer,
		middle,
		spacer,
		sep,
		spacer,
		right,
	)
}

// ─── Help Bar ──────────────────────────────────────────────────────

func (m Model) renderHelpBar() string {
	bindings := Keys.ShortHelp()
	var parts []string
	for _, b := range bindings {
		key := styleHelpKey.Render(b.Help().Key)
		desc := styleHelp.Render(b.Help().Desc)
		parts = append(parts, fmt.Sprintf("%s %s", key, desc))
	}
	return styleHelp.Render(strings.Join(parts, "  •  "))
}

// ─── Status ────────────────────────────────────────────────────────

func (m Model) renderStatus() string {
	if m.err != nil {
		return styleStatusErr.Render("✗ Error: " + m.err.Error())
	}
	if m.statusMessage != "" {
		return styleStatus.Render("● " + m.statusMessage)
	}
	return ""
}

// ─── Help Overlay ──────────────────────────────────────────────────

func (m Model) helpView() string {
	helpContent := m.buildHelpContent()
	helpHeight := strings.Count(helpContent, "\n") + 2
	helpWidth := 60

	if m.width < helpWidth+4 {
		helpWidth = m.width - 4
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colorAccent).
		Padding(1, 2, 1, 2).
		Width(helpWidth).
		Background(colorBg)

	rendered := box.Render(helpContent)

	vPad := (m.height - helpHeight) / 2
	if vPad < 0 {
		vPad = 0
	}
	hPad := (m.width - lipgloss.Width(rendered)) / 2
	if hPad < 0 {
		hPad = 0
	}

	padding := strings.Repeat("\n", vPad)
	indent := strings.Repeat(" ", hPad)

	lines := strings.Split(rendered, "\n")
	for i := range lines {
		if len(lines[i]) < m.width {
			lines[i] = indent + lines[i]
		}
	}

	return padding + strings.Join(lines, "\n")
}

func (m Model) buildHelpContent() string {
	var b strings.Builder
	b.WriteString(stylePanelTitle.Render("KEYBOARD SHORTCUTS"))
	b.WriteString("\n\n")

	for _, group := range Keys.FullHelp() {
		for _, kb := range group {
			keys := strings.Join(kb.Keys(), ", ")
			desc := kb.Help().Desc
			b.WriteString(fmt.Sprintf("  %-18s  %s\n",
				styleHelpKey.Render(keys),
				styleHelp.Render(desc),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString(styleHelp.Render("Press ? or Esc to close this help screen."))
	return b.String()
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
