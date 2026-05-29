package tui

import (
	"fmt"
	"strings"

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

	// Search input wrapper - configured with explicit dimensions to prevent collapsing
	styleSearchBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 1).
	Width(30).
	Height(1).
	Background(colorBgHover)

	styleSearchBoxFocused = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorAccent).
	Padding(0, 1).
	Width(30).
	Height(1).
	Background(colorBgHover)

	// Panel empty state
	styleEmpty = lipgloss.NewStyle().
	Foreground(colorTextDim).
	PaddingLeft(2).
	PaddingTop(1).
	Italic(true)

	// Status / error bar
	styleStatus = lipgloss.NewStyle().
	Foreground(colorAccent2).
	PaddingLeft(1)

	styleStatusErr = lipgloss.NewStyle().
	Foreground(colorError).
	PaddingLeft(1)

	// Player bar container
	stylePlayerBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 2, 0, 2)

	stylePlayerBoxFocused = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorderFoc).
	Padding(0, 2, 0, 2)

	// Download bar container
	styleDownloadBox = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder).
	Padding(0, 2, 0, 2)

	// Control button styles - refined to be clean, flat text buttons
	styleCtrlBtn = lipgloss.NewStyle().
	Foreground(colorTextMid).
	Bold(true)

	styleCtrlBtnActive = lipgloss.NewStyle().
	Foreground(colorAccent2).
	Bold(true)

	styleCtrlBtnPlaying = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)

	// Progress bar text style
	styleProgressTime = lipgloss.NewStyle().
	Foreground(colorTextDim).
	Width(10).
	Align(lipgloss.Right)

	// Mode indicators
	styleModeActive = lipgloss.NewStyle().
	Foreground(colorAccent2).
	Bold(true)

	styleModeInactive = lipgloss.NewStyle().
	Foreground(colorTextDim)

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

	header := m.renderHeader()
	panels := m.renderPanels()
	download := m.renderDownloadBar()
	player := m.renderPlayerBar()
	help := m.renderHelpBar()
	status := m.renderStatus()

	// Use lipgloss.JoinVertical to reliably layout components without inflating vertical line heights
	var elements []string
	elements = append(elements, header)
	elements = append(elements, panels)
	elements = append(elements, download)
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

	// Right side: focus hint
	searchHint := styleSearchLabel.Render("[Tab] focus search")
	if m.isSearching {
		searchHint = styleSearchLabel.Render("⏳ searching…")
	}

	// Vertically center content inside the header to keep elements aligned with the search bar
	left := lipgloss.JoinHorizontal(lipgloss.Center, title, "   ", searchView)
	right := searchHint

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	return styleHeader.Render(
		lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, right),
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
	// Only highlight the panel border when actually navigating it,
	// not when the search input is focused for typing.
	if m.activePanel == PanelSearch && !m.searchFocused {
		leftBorder = panelBorderFocused
	}
	if m.activePanel == PanelQueue {
		rightBorder = panelBorderFocused
	}

	// Search panel title
	panelLabel := "SEARCH RESULTS"
	if m.showingLibrary {
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
	queueTitle := fmt.Sprintf("QUEUE  [%d]", m.queue.Len())
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
	if m.showingLibrary {
		return m.renderLibrary(width, height)
	}
	if m.isSearching {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"⏳ Searching…",
		)
	}
	if len(m.results) == 0 {
		if m.showingRecommendations {
			return styleEmpty.Width(width - 2).Height(height).Render(
				"Loading recommendations…",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No results.\nSearch for something above.",
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
				"No tracks match \"" + m.searchInput.Value() + "\".",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No downloaded tracks yet.\nSearch, add to queue, and download.",
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
	tracks := m.queue.Tracks()
	if len(tracks) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"Queue is empty.\nSearch & add songs to play.",
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
		bgStyle = lipgloss.NewStyle().Background(colorBgHover).Width(width)
		if isPlaying {
			titleStyle = lipgloss.NewStyle().Foreground(colorPlaying).Bold(true)
		} else {
			titleStyle = lipgloss.NewStyle().Foreground(colorTitle).Bold(true)
		}
		infoStyle = lipgloss.NewStyle().Foreground(colorTextMid)
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

// ─── Download Bar ──────────────────────────────────────────────────

func (m Model) renderDownloadBar() string {
	var content string

	if m.downloading {
		maxBarWidth := m.width - 45
		if maxBarWidth < 10 {
			maxBarWidth = 10
		}
		bar := renderProgressBar(m.downloadPct, maxBarWidth)
		content = fmt.Sprintf("%s %s  %s  %.0f%%",
				      styleDownloadLabel.Render("⬇"),
				      styleDownloadTitle.Render(m.downloadTitle),
				      bar,
			m.downloadPct,
		)
	} else if m.downloadDone {
		content = styleDoneLabel.Render("✓  " + m.downloadTitle + " downloaded")
	} else if m.downloadErr != nil {
		content = styleErrorLabel.Render("✗  Download failed: " + m.downloadErr.Error())
	} else {
		content = styleDownloadLabel.Render("⬇  No active downloads")
	}

	return styleDownloadBox.Width(m.width - 6).Render(content)
}

// ─── Player Bar ────────────────────────────────────────────────────

func (m Model) renderPlayerBar() string {
	var nowPlaying string
	var progress string
	var controls string

	nowPlayingIdx := m.queue.CurrentIndex()
	tracks := m.queue.Tracks()

	if m.queue.Len() == 0 || nowPlayingIdx < 0 || nowPlayingIdx >= len(tracks) {
		nowPlaying = styleNowPlaying.Render("⏹  Stopped")
		bar := renderProgressBar(0, m.width-26)
		progress = fmt.Sprintf("%s  %s / %s",
				       bar,
			 styleTime.Render("0:00"),
				       styleTime.Render("0:00"),
		)
		controls = m.renderControls()
	} else {
		t := tracks[nowPlayingIdx]

		trackLabel := t.Title + " — " + t.Artist
		maxLabelWidth := m.width - 12
		if len(trackLabel) > maxLabelWidth && maxLabelWidth > 5 {
			trackLabel = trackLabel[:maxLabelWidth-1] + "…"
		}

		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
						     styleNowPlaying.Render("🎝"),
						     " ",
				       styleNowTitle.Render(trackLabel),
		)

		currentStr := formatDuration(int(m.position))
		totalStr := t.Duration
		if totalStr == "" {
			totalStr = formatDuration(t.DurationSec)
		}
		timeInfo := fmt.Sprintf("%s / %s", currentStr, totalStr)

		barWidth := m.width - lipgloss.Width(timeInfo) - 10
		if barWidth < 10 {
			barWidth = 10
		}
		bar := renderProgressBar(m.percentage(), barWidth)

		progress = lipgloss.JoinHorizontal(lipgloss.Left,
						   bar,
				     "  ",
				     styleTime.Render(timeInfo),
		)

		controls = m.renderControls()
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
					 nowPlaying,
				  progress,
				  controls,
	)

	boxStyle := stylePlayerBox
	if m.activePanel != PanelSearch && m.activePanel != PanelQueue {
		boxStyle = stylePlayerBoxFocused
	}

	return boxStyle.Width(m.width - 6).Render(content)
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
	volLabel := fmt.Sprintf("VOL %s", volBar)

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

	left := lipgloss.JoinHorizontal(lipgloss.Left, prevBtn, "   ", playBtn, "   ", nextBtn)
	middle := stylePlayerCtrl.Render(volLabel)
	right := lipgloss.JoinHorizontal(lipgloss.Left, shuffleLabel, "    ", repeatLabel)

	leftWidth := lipgloss.Width(left)
	middleWidth := lipgloss.Width(middle)
	rightWidth := lipgloss.Width(right)

	totalInnerWidth := m.width - 6

	// Added a strict 2-character safety margin to completely prevent line wrap issues on odd widths
	gap := (totalInnerWidth - leftWidth - middleWidth - rightWidth) / 2
	gap = gap - 1
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	return lipgloss.JoinHorizontal(lipgloss.Left,
				       left,
				spacer,
				middle,
				spacer,
				right,
	)
}

// ─── Help Bar ──────────────────────────────────────────────────────

func (m Model) renderHelpBar() string {
	bindings := Keys.ShortHelp()
	var parts []string
	for _, b := range bindings {
		key := styleHelpKey.Render(b.Keys()[0])
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
