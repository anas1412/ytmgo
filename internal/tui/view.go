package tui

import (
	"fmt"
	"strings"
	"time"

	"image"

	"ytmgo/internal/coverart"
	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	ver "ytmgo/internal/version"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ─── Extra inline styles (beyond what styles.go provides) ──────────

var (
	styleApp, styleSearchBox, styleSearchBoxFocused lipgloss.Style
	styleEmpty, styleSearchLabel                    lipgloss.Style
)

// The search field's wrapper is a fixed width, and the textinput inside
// it has a width of its own. They have to agree: lipgloss drops whatever
// overflows a fixed-width box, so an input allowed to render wider than
// its wrapper loses the text it renders past the edge — the field went
// blank once the query passed the wrapper's width.
const (
	searchBoxWidth   = 28
	searchBoxPadding = 1 // each side
	// A textinput renders its Width plus three cells: the "> " prompt
	// and one for the cursor. Measured, not assumed — guessing here is
	// what produced a field that wrapped and then swallowed itself.
	searchInputChrome = 3
	// searchInputWidth leaves the rendered input exactly filling the box.
	searchInputWidth = searchBoxWidth - 2*searchBoxPadding - searchInputChrome
)

// buildInlineStyles is the view-local half of buildStyles.
func buildInlineStyles() {
	// The search field's well is a filled block. That reads as an input
	// only when the rest of the UI is filled too — on the two themes
	// that leave the terminal's own backdrop alone it is an opaque
	// island in an otherwise transparent header. There it goes without
	// a fill, and the prompt, placeholder and cursor carry the affordance.
	well := func(st lipgloss.Style) lipgloss.Style {
		if !paintBackground {
			return st
		}
		return st.Background(colorBgHover)
	}

	textinputStyle = well(lipgloss.NewStyle().
		Foreground(colorText))
	textinputPlaceholder = well(lipgloss.NewStyle().
		Foreground(colorTextDim).
		Italic(true))
	styleTextDim = lipgloss.NewStyle().Foreground(colorTextDim)

	// App background
	styleApp = lipgloss.NewStyle().
		Background(colorBg)

	// Search input wrapper - inline style (no border, stays on 1 line)
	styleSearchBox = well(lipgloss.NewStyle().
		Foreground(colorText).
		Padding(0, searchBoxPadding).
		Width(searchBoxWidth).
		Height(1))

	styleSearchBoxFocused = well(lipgloss.NewStyle().
		Foreground(colorAccent2).
		Padding(0, searchBoxPadding).
		Width(searchBoxWidth).
		Height(1))

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
}

// ─── textinput styling (referenced from model.go) ──────────────────

var (
	textinputStyle       lipgloss.Style
	textinputPlaceholder lipgloss.Style
)

// ─── View ──────────────────────────────────────────────────────────

// View renders the complete TUI layout.
func (m Model) View() string {
	if !m.ready {
		return "Loading…"
	}

	var view string
	switch m.activePage {
	case PageSettings:
		view = m.renderSettingsPage()
	default:
		view = m.renderPage()
	}
	return m.paintBg(m.fillHeight(view))
}

// hints returns s when inline key hints are on, and nothing when the
// user has hidden them (z) — the footer keeps the permanent set, so the
// way back never disappears.
func (m Model) hints(s string) string {
	if m.settings.ShowHints {
		return s
	}
	return ""
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

// paintBg lays the theme's background under every line. Only the named
// schemes do this: their foregrounds are chosen against their own
// backdrop, so without it a dark scheme on a light terminal would put
// pale text on white. terminal and ytmgo leave the backdrop alone, which
// is what keeps terminal transparency working on those two.
//
// Each line is padded to the full width first, or the fill would stop
// wherever the content happened to end and leave a ragged edge.
func (m Model) paintBg(view string) string {
	if !paintBackground || m.width <= 0 {
		return view
	}
	bgSeq, fgSeq := sgrColor(colorBg, 48), sgrColor(colorText, 38)
	if bgSeq == "" {
		return view
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		if pad := m.width - lipgloss.Width(line); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
		// Lipgloss closes every styled run with 39/49 — "back to the
		// terminal's default" — which would punch a hole through an
		// outer background and let the terminal show through for the
		// rest of the line. When a scheme owns the backdrop its own
		// colours are the default, so those resets are rewritten to
		// them rather than to the terminal's.
		line = strings.NewReplacer(
			"\x1b[49m", bgSeq,
			"\x1b[39m", fgSeq,
			"\x1b[0m", "\x1b[0m"+bgSeq+fgSeq,
		).Replace(line)
		lines[i] = bgSeq + fgSeq + line + "\x1b[0m"
	}
	return strings.Join(lines, "\n")
}

// sgrColor renders a colour as a truecolour SGR sequence for the given
// layer (38 foreground, 48 background).
func sgrColor(c lipgloss.TerminalColor, layer int) string {
	if c == nil {
		return ""
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("\x1b[%d;2;%d;%d;%dm", layer, r>>8, g>>8, b>>8)
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
	if panelWidth < 10 {
		panelWidth = 10
	}
	panelHeight := m.panelHeight()

	// Left panel: Settings list (always focused — arrows navigate it)
	leftBorder := panelBorderFocused
	settingsTitle := stylePanelTitle.Render(truncate("SETTINGS", max(1, panelWidth-2)))
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
	helpTitle := stylePanelTitle.Render(truncate("KEYBOARD SHORTCUTS", max(1, panelWidth-2)))
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

	// Search input.
	//
	// The rendered input is truncated to the box before the box styles
	// it. A fixed lipgloss Width wraps rather than truncates, and it
	// wraps on word boundaries — a long query is one unbroken token with
	// nowhere to break, so the whole thing moved to a second line that
	// Height(1) then discarded, leaving a field showing only its prompt.
	inner := truncate(m.searchInput.View(), searchBoxWidth-2*searchBoxPadding)
	var searchView string
	if m.searchFocused {
		searchView = styleSearchBoxFocused.Render(inner)
	} else {
		searchView = styleSearchBox.Render(inner)
	}

	// Build page tabs (right side) with inline key hints matching [h] / [l] style
	type tabDef struct {
		key   string
		label string
	}
	tabs := []tabDef{
		{"1", "Stream"},
		{"2", "Favs"},
		{"3", "Library"},
		{"4", "History"},
		{"5", "Downloads"},
		{"6", "Settings"},
	}
	var renderedTabs []string
	for i, t := range tabs {
		hint := styleKeyHint.Render("[" + t.key + "]")
		if !m.settings.ShowHints {
			hint = ""
		}
		label := styleNavTab.Render(t.label)
		if int(m.activePage) == i {
			label = styleNavTabActive.Render(t.label)
		}
		rendered := label
		if hint != "" {
			rendered = hint + " " + label
		}
		renderedTabs = append(renderedTabs, rendered)
	}
	tabsStr := strings.Join(renderedTabs, " ")

	// Tab hint — shown inline so users discover focus cycling without
	// glancing down at the help bar.
	tabHint := m.hints(styleKeyHint.Render("[tab]") + styleTextDim.Render(" cycle"))
	// [v] used to sit here too; it lives in the help bar now, next to
	// [X], so the two panel toggles are advertised together in one place.
	left := lipgloss.JoinHorizontal(lipgloss.Center, logo, "   ", searchView, "  ", tabHint)

	gap := m.width - lipgloss.Width(left) - lipgloss.Width(tabsStr) - 2
	if gap < 1 {
		gap = 1
	}
	spacer := strings.Repeat(" ", gap)

	// Hard-truncate: a header wider than the terminal wraps to a second
	// physical row and shifts every mouse hit zone below it.
	return truncate(styleHeader.Render(
		lipgloss.JoinHorizontal(lipgloss.Center, left, spacer, tabsStr),
	), m.width)
}

// ─── Panels (Search Results | Queue + Downloads) ───────────────────
//
// Layout: left panel (search results or library) is full height.
// Right column is split into two stacked sub-panels:
//   - top    = QUEUE  (always)
//   - bottom = DOWNLOADS (always)
// Both sub-panels are always rendered on both Stream and Library tabs.

func (m Model) renderPanels() string {
	// Dynamically calculate column widths to span the layout width.
	// The formula yields exactly 2 columns + 2 spare columns (one per side
	// of the 1-char spacer), no matter the terminal width. Do NOT clamp
	// to a hard minimum — that would overflow when the terminal is narrow.
	outerWidth := (m.width - 2) / 2
	panelWidth := outerWidth - 2
	if panelWidth < 10 {
		panelWidth = 10
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
	fHint := styleKeyHint.Render("[f]")
	xHint := styleKeyHint.Render("[x]")
	panelLabel := "SEARCH RESULTS"
	switch m.activePage {
	case PageHistory:
		cHint := styleKeyHint.Render("[C]")
		panelLabel = "HISTORY  " + xHint + " download  " + cHint + " clear"
	case PageFavorites:
		panelLabel = "FAVORITES" + m.hints("  "+fHint+" unfav  "+xHint+" download")
	case PageDownloads:
		oHint := styleKeyHint.Render("[o]")
		n := 0
		if m.downloader != nil {
			n = len(m.downloader.Jobs())
		}
		panelLabel = fmt.Sprintf("DOWNLOADS  [%d]", n) + m.hints("  "+oHint+" open folder")
	case PageLibrary:
		dHint := styleKeyHint.Render("[d]")
		panelLabel = "LIBRARY" + m.hints("  "+dHint+" delete  "+fHint+" add to fav")
		q := m.searchInput.Value()
		if q != "" {
			panelLabel = "LIBRARY  🔍 \"" + q + "\"" + m.hints("  "+dHint+" delete  "+fHint+" add to fav")
		}
	case PageStream:
		switch {
		case m.openAlbum != nil:
			aHint := styleKeyHint.Render("[a]")
			escHint := styleKeyHint.Render("[esc]")
			panelLabel = "ALBUM" +
				m.hints("  "+aHint+" queue all  "+xHint+" download  "+escHint+" back")
		case m.albumMode:
			albHint := styleKeyHint.Render("[A]")
			panelLabel = "ALBUMS" + m.hints("  "+albHint+" songs  "+xHint+" download album")
		case m.showingRecommendations:
			rHint := styleKeyHint.Render("[R]")
			albHint := styleKeyHint.Render("[A]")
			panelLabel = "RECOMMENDATIONS" + m.hints("  "+rHint+" refresh  "+albHint+" albums  "+xHint+" download  "+fHint+" fav")
		default:
			albHint := styleKeyHint.Render("[A]")
			panelLabel = "SEARCH RESULTS" + m.hints("  "+albHint+" albums  "+xHint+" download  "+fHint+" add to fav")
		}
	default:
		if m.showingRecommendations {
			rHint := styleKeyHint.Render("[R]")
			panelLabel = "RECOMMENDATIONS" + m.hints("  "+rHint+" refresh  "+xHint+" download  "+fHint+" add to fav")
		} else {
			panelLabel = "SEARCH RESULTS" + m.hints("  "+xHint+" download  "+fHint+" add to fav")
		}
	}
	// Truncate every panel title to the panel width: a wrapped title
	// grows the box a full row and breaks mouse hit-testing below it.
	titleW := max(1, panelWidth-2)
	searchTitle := stylePanelTitle.Render(truncate(panelLabel, titleW))

	// Search panel content.
	// lipgloss Height(N) on a bordered style renders N+2 total lines (N content
	// + top + bottom border). The content we pass in is: title (1) +
	// renderSearchResults(panelWidth, contentH). So total rendered = (1 +
	// contentH) + 2 = contentH + 3. We want total = panelHeight, so
	// contentH = panelHeight - 3.
	// The left column splits like the right one when the now-playing
	// panel is open: results on top, art + spectrum beneath.
	resultsH, npH := m.leftPanelSplit()
	contentH := panelHeight - 3
	if npH > 0 {
		contentH = resultsH
	}
	searchContent := m.renderSearchResults(panelWidth, contentH)
	leftPanel := lipgloss.JoinVertical(lipgloss.Top,
		searchTitle,
		indentBlock(searchContent),
	)
	leftHeight := panelHeight - 2
	if npH > 0 {
		leftHeight = resultsH
	}
	leftPanel = leftBorder.
		Width(panelWidth).
		Height(leftHeight).
		Render(leftPanel)

	if npH > 0 {
		npTitle := stylePanelTitle.Render(truncate(m.npPanelTitle(), titleW))
		npPanel := lipgloss.JoinVertical(lipgloss.Top,
			npTitle,
			indentBlock(m.renderNowPlayingPanel(panelWidth, npH)),
		)
		leftPanel = lipgloss.JoinVertical(lipgloss.Top, leftPanel,
			panelBorder.Width(panelWidth).Height(npH).Render(npPanel))
	}

	// Split the right column into queue (top) and lyrics (bottom).
	// Each sub-panel renders as: border-top (1) + title (1) + content (N)
	// + border-bottom (1) = N + 3 total lines. The split lives in
	// rightPanelSplit so the mouse hit-testing stays in sync; the queue
	// takes the whole column while the lyrics pane is off.
	queueContentH, lyricsContentH := m.rightPanelSplit()

	// Queue sub-panel (top of right column)
	dHint := styleKeyHint.Render("[d]")
	dCapHint := styleKeyHint.Render("[D]")
	reorderHint := styleKeyHint.Render("[ctrl+↑↓]")
	queueCount := fmt.Sprintf("[%d]", m.queue.Len())
	if total := m.queueTotalSecs(); total > 0 {
		queueCount = fmt.Sprintf("[%d · %s]", m.queue.Len(), formatTotalDuration(total))
	}
	queueTitle := fmt.Sprintf("QUEUE  %s", queueCount) +
		m.hints(fmt.Sprintf("  %s remove  %s clear  %s reorder", dHint, dCapHint, reorderHint))
	queueTitleStyled := stylePanelTitle.Render(truncate(queueTitle, titleW))
	queueContent := m.renderQueue(panelWidth, queueContentH)
	queuePanel := lipgloss.JoinVertical(lipgloss.Top,
		queueTitleStyled,
		indentBlock(queueContent),
	)
	queueBoxH := queueContentH
	if lyricsContentH == 0 {
		queueBoxH = panelHeight - 2 // one full-height box, as the left column uses
	}
	queuePanel = rightBorder.
		Width(panelWidth).
		Height(queueBoxH).
		Render(queuePanel)

	// Lyrics sub-panel (bottom of right column), when open.
	rightPanel := queuePanel
	if lyricsContentH > 0 {
		yHint := styleKeyHint.Render("[y]")
		lyricsTitle := "LYRICS" + m.hints("  "+yHint+" hide")
		if t, ok := m.queue.Current(); ok && t.Title != "" {
			lyricsTitle += "  " + t.Title
		}
		lyricsTitleStyled := stylePanelTitle.Render(truncate(lyricsTitle, titleW))
		rows := m.renderLyricsPane(max(1, panelWidth-2), lyricsContentH)
		lyricsContent := padPanel(strings.Join(rows, "\n"), panelWidth, lyricsContentH)
		lyricsPanel := lipgloss.JoinVertical(lipgloss.Top,
			lyricsTitleStyled,
			indentBlock(lyricsContent),
		)
		// Bottom sub-panel uses unfocused border (queue owns the focus)
		lyricsPanel = panelBorder.
			Width(panelWidth).
			Height(lyricsContentH).
			Render(lyricsPanel)
		rightPanel = lipgloss.JoinVertical(lipgloss.Top, queuePanel, lyricsPanel)
	}

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
	switch m.activePage {
	case PageFavorites:
		return m.renderFavorites(width, height)
	case PageLibrary:
		return m.renderLibrary(width, height)
	case PageHistory:
		return m.renderHistory(width, height)
	case PageDownloads:
		return m.renderDownloadQueue(width, height)
	}
	return m.renderStreamList(width, height)
}

// clearCover removes a kitty cover image, or "" when there can't be one.
func (m Model) clearCover() string {
	if coverart.KittySupported() {
		return coverart.KittyClear()
	}
	return ""
}

// renderStreamList draws whichever list the Stream page is showing.
func (m Model) renderStreamList(width, height int) string {
	if m.openAlbum != nil {
		return m.renderAlbumTracks(width, height)
	}
	if m.albumMode {
		return m.renderAlbums(width, height)
	}
	if m.isSearching {
		return styleEmpty.Width(width - 2).Height(height).Render(
			m.spinner() + "  Searching…",
		)
	}
	if len(m.results) == 0 {
		if m.showingRecommendations {
			if !m.recsLoaded {
				return styleEmpty.Width(width - 2).Height(height).Render(
					m.spinner() + "  Loading recommendations…",
				)
			}
			// Recommendations come from what has been played, and there
			// is no filler, so an empty list means there is nothing to
			// go on yet — say that instead of spinning forever.
			return styleEmpty.Width(width - 2).Height(height).Render(
				"Play something and recommendations will follow  ([R] retry)",
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

	if ind := scrollIndicator(start, len(m.results)-end, m.searchCursor+1, len(m.results)); ind != "" {
		lines = append(lines, ind)
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

// vizBars is how many spectrum columns fit the left panel. Each bar is
// drawn two cells wide with a gap, so the panel width divides by three.
// vizBars picks how many bars to ask cava for. The count is fixed for
// the life of the process, so it is sized for the steady state: the
// panel open with artwork loaded beside it. renderSpectrum stretches
// whatever arrives to the real width, so an imperfect guess here costs
// nothing but bar thickness.
func (m Model) vizBars() int {
	// The spectrum owns the whole panel now — the artwork moved to the
	// player bar, so nothing is subtracted for it any more.
	avail := (m.width-2)/2 - 4
	n := avail / 3 // roughly two cells per bar plus a gap
	if n < 4 {
		n = 4
	}
	if n > 128 {
		n = 128
	}
	return n
}

// coverFitCells sizes the art in whole cells, preserving its aspect
// ratio on screen (a cell is CellAspect times taller than it is wide),
// so a square cover stays square.
func coverFitCells(img image.Image, maxCols, maxRows int) (cols, rows int) {
	if img == nil || maxCols < 1 || maxRows < 1 {
		return 0, 0
	}
	b := img.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return 0, 0
	}
	ratio := float64(b.Dy()) / float64(b.Dx()) // height / width
	rows = maxRows
	// Round to the nearest cell, not down: 4 rows of a square cover
	// want 9.6 columns, and truncating to 9 rendered it 6% narrow.
	cols = int(float64(rows)*coverart.CellAspect/ratio + 0.5)
	if cols > maxCols {
		cols = maxCols
		rows = int(float64(cols)*ratio/coverart.CellAspect + 0.5)
		if rows > maxRows {
			rows = maxRows
		}
	}
	if cols < 1 || rows < 1 {
		return 0, 0
	}
	return cols, rows
}

// displayPosition returns the playback position to render: the last
// IPC position glided forward by wall-clock time while playing, so
// both the progress bar and the lyrics highlight move smoothly
// between the player's coarse updates.
func (m Model) displayPosition() float64 {
	pos := m.position
	if m.playerState == player.StatePlaying {
		elapsed := time.Since(m.lastPositionAt).Seconds()
		if elapsed < 1.0 && elapsed >= 0 {
			pos = m.lastPosition + elapsed
			if m.duration > 0 && pos > m.duration {
				pos = m.duration
			}
		}
	}
	return pos
}

// npPanelTitle labels the now-playing panel with whatever is playing.
func (m Model) npPanelTitle() string {
	return "VISUALIZER" + m.hints("  "+styleKeyHint.Render("[v]")+" hide")
}

// renderNowPlayingPanel is the visualizer: the spectrum, full width.
// The album art lives in the player bar and the lyrics in their own
// pane, so this panel is the bars and nothing else.
func (m Model) renderNowPlayingPanel(width, height int) string {
	rows := m.renderSpectrum(max(1, width-2), height)
	return padPanel(strings.Join(rows, "\n"), width, height)
}

// artRenderCache memoises one drawn half-block image. Rebuilding costs
// 11-17ms — fine when the artwork changes, ruinous at the spectrum's
// frame rate. One cache per on-screen image, so the player cover and
// the album cover don't evict each other every frame.
type artRenderCache struct {
	key   string
	lines []string
}

var coverRender, albumArtRender artRenderCache

// renderArtBlock returns an image's rows, vertically centred in height.
// Uses kitty's graphics protocol where available (under the given image
// id) and half-blocks otherwise. sendN>0 means Update still owes the
// terminal the transmit.
func renderArtBlock(img image.Image, url string, cols, rows, height, sendN, kittyID int, cache *artRenderCache) []string {
	if cols < 1 || rows < 1 {
		return nil
	}
	top := max(0, (height-rows)/2)
	out := make([]string, 0, height)
	for i := 0; i < top; i++ {
		out = append(out, "")
	}

	if coverart.KittySupported() {
		esc := ""
		if sendN > 0 {
			// The encode is cached, so repeating it costs nothing.
			if t, err := coverart.KittyTransmitCached(img, url, cols, rows, kittyID); err == nil {
				esc = t
			}
		}
		esc += coverart.KittyDisplayID(cols, rows, kittyID)
		out = append(out, esc+coverart.Blank(cols))
		for i := 1; i < rows; i++ {
			out = append(out, coverart.Blank(cols))
		}
		return out
	}

	key := fmt.Sprintf("blocks|%s|%d|%d", url, cols, rows)
	if cache.key != key {
		lines := make([]string, 0, rows)
		for _, row := range coverart.Grid(img, cols, rows) {
			var b strings.Builder
			for _, c := range row {
				b.WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color(c.Top.Hex())).
					Background(lipgloss.Color(c.Bottom.Hex())).
					Render(coverart.HalfBlock))
			}
			lines = append(lines, b.String())
		}
		cache.key, cache.lines = key, lines
	}
	return append(out, cache.lines...)
}

// renderCoverBlock is the player bar's cover column.
func (m Model) renderCoverBlock(cols, rows, height int) []string {
	if cols < 1 || rows < 1 {
		msg := ""
		switch {
		case m.coverErr != "":
			msg = "no art"
		case m.coverLoading:
			msg = m.spinner()
		}
		if msg == "" {
			return nil
		}
		return []string{styleTextDim.Render(msg)}
	}
	return renderArtBlock(m.coverImg, m.coverURL, cols, rows, height, m.coverSendN, coverart.CoverImageID, &coverRender)
}

// clearCoverImage returns the delete escape while Update says one is
// owed to the terminal. Pure: the countdown lives in the model.
func (m Model) clearCoverImage() string {
	if !coverart.KittySupported() {
		return ""
	}
	esc := ""
	if m.coverClearN > 0 {
		esc += coverart.KittyClear()
	}
	if m.albumArtClearN > 0 {
		esc += coverart.KittyClearID(coverart.AlbumImageID)
	}
	return esc
}

// ─── Lyrics pane ─────────────────────────────────────────────────────

// activeLyricLine returns the index of the lyric line that is current
// at the interpolated playback position, or -1 for plain lyrics.
func (m Model) activeLyricLine() int {
	if !m.lyricsSynced {
		return -1
	}
	pos := m.displayPosition()
	idx := -1
	for i, ln := range m.lyricLines {
		if ln.Time <= pos+0.05 {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// renderLyricsPane draws the lyrics column of the now-playing panel:
// loading / empty states when there is nothing to show, plain lines
// otherwise, with the current line highlighted when the lyrics are
// synced. While lyricsFollow is set (until the user scrolls the pane
// themselves) the window keeps the active line near its middle.
func (m Model) renderLyricsPane(width, height int) []string {
	if width < 4 || height < 1 {
		return nil
	}
	switch {
	case m.lyricsLoading:
		return []string{styleTextDim.Render(truncate(m.spinner()+"  Loading lyrics…", width))}
	case m.lyricsErr != "":
		return []string{styleTextDim.Render(truncate(m.lyricsErr, width))}
	case len(m.lyricLines) == 0:
		msg := "No lyrics found"
		if m.lyricsTrackID == "" {
			msg = "Play a track to see its lyrics"
		}
		return []string{styleTextDim.Render(truncate(msg, width))}
	}

	active := m.activeLyricLine()
	offset := m.lyricsOffset
	if m.lyricsFollow && active >= 0 {
		offset = active - height/2
	}
	offset = max(0, min(offset, max(0, len(m.lyricLines)-height)))

	rows := make([]string, 0, height)
	for i := offset; i < min(offset+height, len(m.lyricLines)); i++ {
		ln := m.lyricLines[i]
		text := truncate(strings.TrimSpace(ln.Text), width)
		switch {
		case text == "":
			rows = append(rows, "") // LRC spacer between stanzas
		case i == active:
			rows = append(rows, styleNowTitle.Render(text))
		default:
			rows = append(rows, styleTextDim.Render(text))
		}
	}
	return rows
}

// renderSpectrum returns the bar rows, stretched to fill the width it
// is given. The bar count is fixed when cava starts, but the available
// width is not — the artwork loads afterwards and takes its share, and
// the terminal can be resized — so the bars are spread across whatever
// space they actually get rather than assuming a fixed cell each.
func (m Model) renderSpectrum(width, height int) []string {
	if width < 4 || height < 1 {
		return nil
	}
	frame := m.vizFrame
	if len(frame) == 0 {
		msg := m.spinner() + "  Listening…"
		if m.viz == nil {
			msg = "spectrum off"
		}
		return []string{styleTextDim.Render(truncate(msg, width))}
	}

	n := len(frame)
	heights := make([]int, n)
	for i, val := range frame {
		heights[i] = max(0, min(val*height/100, height))
	}

	// Each bar owns the slice of columns between these boundaries, so
	// the row always adds up to exactly the available width.
	edge := func(i int) int { return i * width / n }

	rows := make([]string, 0, height)
	for row := 0; row < height; row++ {
		remaining := height - row
		var b strings.Builder
		for i := 0; i < n; i++ {
			span := edge(i+1) - edge(i)
			if span < 1 {
				continue
			}
			// The gap separates this bar from the next, so the last
			// bar does not need one — without this the spectrum always
			// stopped a column short of its own right edge.
			fill := span
			if i < n-1 {
				fill = max(1, span-1)
			}
			gap := span - fill
			if heights[i] >= remaining {
				style := styleVizLow
				switch {
				case remaining > height*2/3:
					style = styleVizHigh
				case remaining > height/3:
					style = styleVizMid
				}
				b.WriteString(style.Render(strings.Repeat("█", fill)))
			} else {
				b.WriteString(strings.Repeat(" ", fill))
			}
			b.WriteString(strings.Repeat(" ", gap))
		}
		rows = append(rows, b.String())
	}
	return rows
}

// renderAlbums draws the album search results.
func (m Model) renderAlbums(width, height int) string {
	if m.isSearching {
		return styleEmpty.Width(width - 2).Height(height).Render(m.spinner() + "  Searching albums…")
	}
	if len(m.albums) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"Type an album name and press Enter  ([A] back to songs)")
	}

	// Rows are two columns narrower than the panel, the same as the
	// stream list and the queue, so a page switch does not shift the
	// right-hand column.
	rowW := max(1, width-2)
	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.searchOffset
	end := min(start+maxItems, len(m.albums))

	for i := start; i < end; i++ {
		a := m.albums[i]
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.searchCursor
		prefix := fmt.Sprintf("%d. ", i+1)
		title := truncate(a.Title, max(4, rowW-lipgloss.Width(prefix)-2))

		artist := a.Artist
		if artist == "" {
			artist = "Unknown artist"
		}
		leftInfo := "   " + artist
		rightInfo := a.Year
		maxLeft := rowW - lipgloss.Width(rightInfo) - 2
		if maxLeft > 3 {
			leftInfo = truncate(leftInfo, maxLeft)
		}
		spacing := max(1, rowW-lipgloss.Width(leftInfo)-lipgloss.Width(rightInfo))
		info := leftInfo + strings.Repeat(" ", spacing) + rightInfo
		lines = append(lines, renderListItemBlock(prefix+title, info, isSelected, false, rowW))
	}

	if ind := scrollIndicator(start, len(m.albums)-end, m.searchCursor+1, len(m.albums)); ind != "" {
		lines = append(lines, ind)
	}
	return padPanel(strings.Join(lines, "\n"), width, height)
}

// renderAlbumTracks draws the tracklist of the open album.
func (m Model) renderAlbumTracks(width, height int) string {
	if m.isLoadingAlbum {
		return styleEmpty.Width(width - 2).Height(height).Render(m.spinner() + "  Loading album…")
	}
	if len(m.albumTracks) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render("This album has no playable tracks")
	}

	// Rows are two columns narrower than the panel, the same as the
	// stream list and the queue, so a page switch does not shift the
	// right-hand column.
	rowW := max(1, width-2)
	var lines []string

	// A compact header strip above the tracklist: the album's own line,
	// then its stats, so the panel title stays short and untruncated.
	// The album's cover sits right-aligned across the strip's rows.
	// albumStripRows tells the scroll clamp these rows are spoken for.
	if m.openAlbum != nil {
		total := 0
		for _, r := range m.albumTracks {
			total += r.Duration
		}
		stats := m.openAlbum.Artist
		if m.openAlbum.Year != "" {
			stats += " · " + m.openAlbum.Year
		}

		// Art on the left, text beside it — the same shape as the
		// player bar's card, at album-page size.
		artH := albumStripRows - 1 // last strip row is the separator
		artCols, artRows := coverFitCells(m.albumArtImg, albumArtSlotCols, artH)
		art := renderArtBlock(m.albumArtImg, m.albumArtURL, artCols, artRows, artH,
			m.albumArtSendN, coverart.AlbumImageID, &albumArtRender)
		inset := 0
		if artCols > 0 {
			inset = artCols + 2
		}
		textW := max(4, rowW-inset)
		trackWord := "tracks"
		if len(m.albumTracks) == 1 {
			trackWord = "track"
		}
		strip := []string{
			styleNowTitle.Render(truncate(m.openAlbum.Title, textW)),
			styleTextDim.Render(truncate(stats, textW)),
			styleTextDim.Render(truncate(fmt.Sprintf("%d %s · %s",
				len(m.albumTracks), trackWord, formatTotalDuration(total)), textW)),
			"",
			"",
		}
		for i := range strip {
			if inset == 0 {
				lines = append(lines, strip[i])
				continue
			}
			a := ""
			if i < len(art) {
				a = art[i]
			}
			if pad := inset - 2 - lipgloss.Width(a); pad > 0 {
				a += strings.Repeat(" ", pad)
			}
			lines = append(lines, a+"  "+strip[i])
		}
	}

	maxItems := (height - albumStripRows - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.searchOffset
	end := min(start+maxItems, len(m.albumTracks))

	// Track numbers are absolute positions in the album.
	numW := len(fmt.Sprintf("%d", len(m.albumTracks)))
	for i := start; i < end; i++ {
		r := m.albumTracks[i]
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.searchCursor
		prefix := fmt.Sprintf("%0*d. ", numW, i+1)
		title := truncate(r.Title, max(4, rowW-lipgloss.Width(prefix)-2))

		leftInfo := "   " + m.openAlbum.Artist
		heart := ""
		if m.favoriteSet[r.ID] {
			heart = "♥  "
		}
		rightInfo := heart + formatDuration(r.Duration)
		maxLeft := rowW - lipgloss.Width(rightInfo) - 2
		if maxLeft > 3 {
			leftInfo = truncate(leftInfo, maxLeft)
		}
		spacing := max(1, rowW-lipgloss.Width(leftInfo)-lipgloss.Width(rightInfo))
		info := leftInfo + strings.Repeat(" ", spacing) + rightInfo
		lines = append(lines, renderListItemBlock(prefix+title, info, isSelected, false, rowW))
	}

	if ind := scrollIndicator(start, len(m.albumTracks)-end, m.searchCursor+1, len(m.albumTracks)); ind != "" {
		lines = append(lines, ind)
	}
	return padPanel(strings.Join(lines, "\n"), width, height)
}

// padPanel pads rendered rows to the panel's full width and height so
// no stale content from a previous frame shows through.
// indentBlock shifts a panel's content one column right so it lines up
// under the panel title, which the border style already insets by one.
// Content blocks are two columns narrower than the box, so this spends
// one of those columns on the left and leaves the other on the right —
// the same gutter on both sides instead of all of it on the right.
func indentBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = " " + line
	}
	return strings.Join(lines, "\n")
}

func padPanel(s string, width, height int) string {
	paddedW := max(1, width-2)
	out := padToWidth(s, paddedW)
	if cnt := strings.Count(out, "\n") + 1; cnt < height {
		out += "\n" + strings.Join(make([]string, height-cnt), "\n"+strings.Repeat(" ", paddedW))
	}
	return out
}

func (m Model) renderLibrary(width, height int) string {
	tracks := m.filteredLibrary()
	if len(tracks) == 0 {
		if !m.libraryLoaded {
			return styleEmpty.Width(width - 2).Height(height).Render(
				m.spinner() + "  Scanning library…",
			)
		}
		if m.searchInput.Value() != "" {
			return styleEmpty.Width(width - 2).Height(height).Render(
				"No tracks match \"" + m.searchInput.Value() + "\"",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No downloaded tracks yet",
		)
	}

	// Rows are two columns narrower than the panel, the same as the
	// stream list and the queue, so a page switch does not shift the
	// right-hand column.
	rowW := max(1, width-2)
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
		maxTitle := rowW - lipgloss.Width(prefix) - 2
		if maxTitle > 3 {
			title = truncate(title, maxTitle)
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
		// Same fixed slot as the queue — every library track is on disk.
		leftInfo := " ✓ " + artist
		heart := ""
		if m.favoriteSet[t.ID] {
			heart = "♥  "
		}
		rightInfo := heart + dur
		maxLeft := rowW - lipgloss.Width(rightInfo) - 2
		if maxLeft > 3 {
			leftInfo = truncate(leftInfo, maxLeft)
		}
		spacing := rowW - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
		if spacing < 1 {
			spacing = 1
		}
		info := leftInfo + strings.Repeat(" ", spacing) + rightInfo

		lines = append(lines, renderListItemBlock(line, info, isSelected, false, rowW))
	}

	if ind := scrollIndicator(start, len(tracks)-end, m.libraryCursor+1, len(tracks)); ind != "" {
		lines = append(lines, ind)
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

func (m Model) renderFavorites(width, height int) string {
	tracks := m.favorites
	if len(tracks) == 0 {
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No favorites yet — press f on any track",
		)
	}

	// Rows are two columns narrower than the panel, the same as the
	// stream list and the queue, so a page switch does not shift the
	// right-hand column.
	rowW := max(1, width-2)
	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.favOffset
	end := start + maxItems
	if end > len(tracks) {
		end = len(tracks)
	}

	for i := start; i < end; i++ {
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.favCursor
		t := tracks[i]
		prefix := fmt.Sprintf("%d. ", i+1)
		title := t.Title
		maxTitle := rowW - lipgloss.Width(prefix) - 2
		if maxTitle > 3 {
			title = truncate(title, maxTitle)
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
		rightInfo := "♥  " + dur
		maxLeft := rowW - lipgloss.Width(rightInfo) - 2
		if maxLeft > 3 {
			leftInfo = truncate(leftInfo, maxLeft)
		}
		spacing := rowW - lipgloss.Width(leftInfo) - lipgloss.Width(rightInfo)
		if spacing < 1 {
			spacing = 1
		}
		info := leftInfo + strings.Repeat(" ", spacing) + rightInfo

		lines = append(lines, renderListItemBlock(line, info, isSelected, false, rowW))
	}

	if ind := scrollIndicator(start, len(tracks)-end, m.favCursor+1, len(tracks)); ind != "" {
		lines = append(lines, ind)
	}

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

func (m Model) renderHistory(width, height int) string {
	entries := m.history
	if len(entries) == 0 {
		if !m.historyLoaded {
			return styleEmpty.Width(width - 2).Height(height).Render(
				m.spinner() + "  Loading history…",
			)
		}
		return styleEmpty.Width(width - 2).Height(height).Render(
			"No history yet — play some tracks!",
		)
	}

	// Rows are two columns narrower than the panel, the same as the
	// stream list and the queue, so a page switch does not shift the
	// right-hand column.
	rowW := max(1, width-2)
	var lines []string
	maxItems := (height - 1) / 2
	if maxItems < 1 {
		maxItems = 1
	}
	start := m.historyOffset
	end := start + maxItems
	if end > len(entries) {
		end = len(entries)
	}

	for i := start; i < end; i++ {
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.historyCursor
		e := entries[i]

		prefix := fmt.Sprintf("%d. ", i+1)
		title := e.Title
		maxTitle := rowW - lipgloss.Width(prefix) - 2
		if maxTitle > 3 {
			title = truncate(title, maxTitle)
		}
		line := prefix + title

		artist := e.Artist
		if artist == "" {
			artist = "Unknown artist"
		}
		timeAgo := relativeTime(e.PlayedAt)
		leftInfo := "   " + artist
		maxLeft := rowW - lipgloss.Width(timeAgo) - 2
		if maxLeft > 3 {
			leftInfo = truncate(leftInfo, maxLeft)
		}
		spacing := rowW - lipgloss.Width(leftInfo) - lipgloss.Width(timeAgo)
		if spacing < 1 {
			spacing = 1
		}
		info := leftInfo + strings.Repeat(" ", spacing) + timeAgo

		lines = append(lines, renderListItemBlock(line, info, isSelected, false, rowW))
	}

	if ind := scrollIndicator(start, len(entries)-end, m.historyCursor+1, len(entries)); ind != "" {
		lines = append(lines, ind)
	}

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
	maxTitle := width - lipgloss.Width(prefix)
	if maxTitle > 3 {
		title = truncate(title, maxTitle)
	}
	line := prefix + title

	// Right-align track metadata details
	leftInfo := "   " + artist
	heart := ""
	if m.favoriteSet[r.ID] {
		heart = "♥  "
	}
	rightInfo := heart + dur
	maxLeft := width - lipgloss.Width(rightInfo) - 2
	if maxLeft > 3 {
		leftInfo = truncate(leftInfo, maxLeft)
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

	if ind := scrollIndicator(start, len(tracks)-end, m.queueCursor+1, len(tracks)); ind != "" {
		lines = append(lines, ind)
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
	maxTitle := width - lipgloss.Width(prefix)
	title := t.Title
	if maxTitle > 3 {
		title = truncate(title, maxTitle)
	}
	line := prefix + title

	// The downloaded marker sits in a fixed-width slot on the left. It
	// used to be appended after the duration, which pushed the duration
	// three columns left on exactly those tracks that were downloaded,
	// so the column never lined up.
	slot := "   "
	if t.Downloaded {
		slot = " ✓ "
	}
	leftInfo := slot + t.Artist
	heart := ""
	if m.favoriteSet[t.ID] {
		heart = "♥  "
	}
	rightInfo := heart + t.Duration
	maxLeft := width - lipgloss.Width(rightInfo) - 2
	if maxLeft > 3 {
		leftInfo = truncate(leftInfo, maxLeft)
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

	return bgStyle.Render(truncate(titleStyle.Render(line), width) + "\n" + truncate(infoStyle.Render(info), width))
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
		// up a job, but the download takes a beat before emitting its first
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
			line = fmt.Sprintf("⬇ %s  %s  Starting…",
				truncate(j.Title, max(1, width-25)),
				m.spinner(),
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

	// Rows come from settingDefs — the single source of truth shared
	// with the keyboard and mouse handlers.

	// Each item uses ~4 lines (label, value, desc, blank).
	// Reserve 2 lines for scroll indicator + help text at bottom.
	vis := (panelHeight - 2) / 4
	if vis < 1 {
		vis = 1
	}
	offset := m.settingsOffset
	end := offset + vis
	if end > len(settingDefs) {
		end = len(settingDefs)
	}

	innerW := max(1, panelWidth-2)

	for idx := offset; idx < end; idx++ {
		def := settingDefs[idx]
		cursor := "  "
		if idx == m.settingsCursor && !m.settingsEditField {
			cursor = "▶ "
		}

		// Truncate each element to innerW so it never spills out of the
		// bordered panel — descriptions like "… Offline (download first)"
		// are particularly long and would overflow on narrow terminals.
		label := styleSettingsLabel.Render(truncate(cursor+def.label, innerW))
		value := styleSettingsValue.Render(truncate(def.value(&m), innerW))
		desc := styleSettingsDesc.Render(truncate(def.desc(&m), innerW))

		// Show an inline [Open] button on rows that declare one (the
		// Download Dir row) — makes the 'o' shortcut discoverable.
		if def.openBtn && !m.settingsEditField {
			openBtn := "  " + styleSettingsOpenBtn.Render("[Open]")
			value = value + openBtn
		}

		// When editing a string field, show the input
		if m.settingsEditField && idx == m.settingsCursor {
			value = styleSettingsValue.Render(m.settingsEditInput.View())
		}

		// Clamp AFTER styling: the styles add left padding, so a string
		// truncated to innerW before styling can still wrap inside the
		// box and grow it a row (breaking mouse hit-testing below).
		lines = append(lines, truncate(label, innerW))
		lines = append(lines, truncate("  "+value, innerW))
		lines = append(lines, truncate(desc, innerW))
		lines = append(lines, "")
	}

	// Scroll indicator
	if end < len(settingDefs) {
		lines = append(lines, truncate(styleSettingsDesc.Render("  ↓ more items below"), innerW))
	} else if offset > 0 {
		lines = append(lines, truncate(styleSettingsDesc.Render("  ↑ more items above"), innerW))
	}

	// Help text at bottom
	lines = append(lines, truncate(styleSettingsDesc.Render("↑↓ navigate · Enter toggle/edit · Esc cancel edit · 1-5 switch page"), innerW))

	// Pad/truncate each line to full width and full height — overwrites
	// any stale content from prior taller frames.
	result := strings.Join(lines, "\n")
	result = padToWidth(result, innerW)
	contentLines := strings.Split(result, "\n")
	if len(contentLines) > panelHeight {
		contentLines = contentLines[:panelHeight]
	}
	if cnt := len(contentLines); cnt < panelHeight {
		contentLines = append(contentLines, make([]string, panelHeight-cnt)...)
		for i := cnt; i < panelHeight; i++ {
			contentLines[i] = strings.Repeat(" ", innerW)
		}
	}
	return strings.Join(contentLines, "\n")
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

// truncate shortens s to at most maxLen terminal cells, appending an
// ellipsis when cut. Width-aware: CJK characters occupy two cells and
// are never split mid-rune.
func truncate(s string, maxLen int) string {
	if maxLen < 1 {
		return ""
	}
	if lipgloss.Width(s) <= maxLen {
		return s
	}
	return ansi.Truncate(s, maxLen, "…")
}

var styleTextDim lipgloss.Style

// ─── Player Bar ────────────────────────────────────────────────────

func (m Model) renderPlayerBar() string {
	fullW := m.width - 6 // box width(m.width) - doubleBorder(2) - padding(4)
	innerW := fullW
	if m.playerCoverSlot() {
		innerW -= playerCoverCols + 2
	}

	nowPlayingIdx := m.queue.CurrentIndex()
	tracks := m.queue.Tracks()

	var nowPlaying, albumRow string
	if !m.playerCoverSlot() || nowPlayingIdx >= len(tracks) {
		msg := "Ready — search and add tracks"
		if m.queue.Len() > 0 {
			msg = "Playback finished"
		}
		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
			styleTime.Render("⏹"),
			"  ",
			styleTime.Render(msg),
		)
	} else {
		t := tracks[nowPlayingIdx]
		trackLabel := t.Title + " — " + t.Artist
		if innerW > 5 {
			trackLabel = truncate(trackLabel, innerW)
		}
		nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
			styleNowIndicator.Render("▶"),
			"  ",
			styleNowTitle.Render(trackLabel),
		)
		// Right-align a dim "up next" hint on the same row when it fits,
		// so the layout height (and mouse hit-testing) never changes.
		if next, ok := m.queue.PeekNext(); ok {
			upNext := "⏭ " + next.Title
			avail := innerW - lipgloss.Width(nowPlaying) - 4
			if avail >= 12 {
				upNext = truncate(upNext, avail)
				gapN := innerW - lipgloss.Width(nowPlaying) - lipgloss.Width(upNext)
				if gapN > 0 {
					nowPlaying = lipgloss.JoinHorizontal(lipgloss.Left,
						nowPlaying,
						strings.Repeat(" ", gapN),
						styleTextDim.Render(upNext),
					)
				}
			}
		}
		// The middle row carries the album, which the title row has no
		// room for. Blank when the source didn't say.
		if t.Album != "" {
			albumRow = styleTextDim.Render(truncate("○ "+t.Album, innerW))
		}
	}

	// Transport, seek bar and modes share one row — the bar takes
	// whatever width the clusters leave. Geometry lives in
	// playerRowLayout, which the mouse reads too.
	combined := m.playerRowLayout().row

	rows := []string{
		truncate(nowPlaying, max(1, innerW)),
		truncate(albumRow, max(1, innerW)),
		// innerW, not fullW: the combined row is placed after the cover
		// column, so letting it run to the full width overflowed the
		// box by exactly the cover slot.
		truncate(combined, max(1, innerW)),
	}

	var content string
	if m.playerCoverSlot() {
		// Cover column left of all three rows. renderCoverBlock emits
		// the kitty escapes (or half-block cells); a fixed-width slot
		// keeps the text from shifting while the art loads.
		cover := m.renderCoverBlock(m.playerCoverFit())
		lines := make([]string, 3)
		for i := 0; i < 3; i++ {
			c := ""
			if i < len(cover) {
				c = cover[i]
			}
			pad := playerCoverCols - lipgloss.Width(c)
			if pad > 0 {
				c += strings.Repeat(" ", pad)
			}
			// The combined row measures its own inset, so it must not
			// be prefixed twice — rows 0 and 1 are plain text and get
			// the cover plus gap; row 2 already accounted for it in its
			// zone arithmetic but still needs the visible cells.
			lines[i] = c + "  " + rows[i]
		}
		content = strings.Join(lines, "\n")
	} else {
		content = strings.Join(rows, "\n")
	}

	boxStyle := stylePlayerBox
	if m.playerState != player.StatePlaying {
		boxStyle = stylePlayerBoxStopped
	}

	// The delete escape rides on the player bar because the bar is on
	// every page — a kitty image outlives the frame that drew it, and
	// this is the one place guaranteed to still be rendering.
	return m.clearCoverImage() + boxStyle.Render(content)
}

// playerCoverFit sizes the art for the player bar's fixed slot.
func (m Model) playerCoverFit() (cols, rows, height int) {
	c, r := coverFitCells(m.coverImg, playerCoverCols, 3)
	return c, r, 3
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
		left = styleVersion.Render("⋯ " + ver.Version)
	case "latest":
		left = styleVersion.Render("✓ " + ver.Version + " — up to date")
	default:
		left = styleUpdateAvail.Render("⬆  Update " + ver.Version + " → " + m.updateAvailable + " — press U")
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
	// Hard-truncate so the help bar can never wrap (see renderStatus).
	return truncate(strings.Repeat(" ", margin)+left+strings.Repeat(" ", gap)+right+strings.Repeat(" ", margin), m.width)
}

// ─── Status ────────────────────────────────────────────────────────

func (m Model) renderStatus() string {
	// Hard-truncate every variant: a status line wider than the terminal
	// (long Japanese titles, long errors) wraps to a second physical row
	// and shifts the player bar's mouse hit zones.
	if m.err != nil {
		return truncate(styleStatusErr.Render("✗ Error: "+m.err.Error()), m.width)
	}
	if m.isConfirming() && m.confirmAction == confirmDeleteTrack {
		// Delete-track confirmation is fully styled inline in the message
		// itself — return it raw so the ANSI codes aren't re-wrapped.
		return truncate(m.statusMessage, m.width)
	}
	if m.statusMessage != "" {
		return truncate(styleStatus.Render("● "+m.statusMessage), m.width)
	}
	// Nothing actionable — show a quote (when enabled) or classic tip
	if m.settings.ShowQuotes {
		return truncate(styleStatusIdle.Render("▸ "+m.currentQuote), m.width)
	}
	return truncate(styleStatusIdle.Render("▸ "+m.currentTip()), m.width)
}

// ─── Help Overlay ──────────────────────────────────────────────────

// renderHelpPanel renders keyboard shortcuts inside a bordered panel.
// When the single-column list would overflow the panel (it usually
// would: the keymap is longer than a terminal is tall), the entries
// flow into two columns so no shortcut is silently cut off.
func (m Model) renderHelpPanel(panelWidth, panelHeight int) string {
	innerW := max(1, panelWidth-2)

	keyCol := styleHelpKey.Width(12)
	var entries []string
	for i, group := range Keys.FullHelp() {
		if i > 0 {
			entries = append(entries, "")
		}
		for _, kb := range group {
			// Help().Key is the human-readable label; the raw key list
			// renders a space key as literally nothing.
			entries = append(entries, " "+keyCol.Render(kb.Help().Key)+" "+styleHelp.Render(kb.Help().Desc))
		}
	}

	var lines []string
	if len(entries) > panelHeight && innerW >= 56 {
		colW := innerW / 2
		rows := (len(entries) + 1) / 2
		left, right := entries[:rows], entries[rows:]
		for i := 0; i < rows; i++ {
			l := truncate(left[i], colW)
			l += strings.Repeat(" ", max(0, colW-lipgloss.Width(l)))
			r := ""
			if i < len(right) {
				r = truncate(right[i], innerW-colW)
			}
			lines = append(lines, l+r)
		}
	} else {
		for _, e := range entries {
			lines = append(lines, truncate(e, innerW))
		}
	}

	result := padToWidth(strings.Join(lines, "\n"), innerW)
	out := strings.Split(result, "\n")
	if len(out) > panelHeight {
		out = out[:panelHeight]
	}
	for len(out) < panelHeight {
		out = append(out, strings.Repeat(" ", innerW))
	}
	return strings.Join(out, "\n")
}

// ─── Helpers ───────────────────────────────────────────────────────

// queueTotalSecs sums the known durations of all queued tracks.
func (m Model) queueTotalSecs() int {
	total := 0
	for _, t := range m.queue.Tracks() {
		total += t.DurationSec
	}
	return total
}

// formatTotalDuration renders a total running time, with hours when needed.
func formatTotalDuration(secs int) string {
	if secs <= 0 {
		return "0:00"
	}
	h := secs / 3600
	mnt := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mnt, s)
	}
	return fmt.Sprintf("%d:%02d", mnt, s)
}

// spinnerFrames drives the loading spinners; advances on the 500ms tick.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴"}

// spinner returns the current loading-spinner frame.
func (m Model) spinner() string {
	return spinnerFrames[m.tickCount%len(spinnerFrames)]
}

// scrollIndicator renders the "more items" footer under a scrolled list.
// Returns "" when everything is visible.
func scrollIndicator(above, below, cursor, total int) string {
	if above <= 0 && below <= 0 {
		return ""
	}
	var parts []string
	if above > 0 {
		parts = append(parts, fmt.Sprintf("↑ %d above", above))
	}
	if below > 0 {
		parts = append(parts, fmt.Sprintf("↓ %d below", below))
	}
	txt := fmt.Sprintf("  %s  [cursor %d/%d]", strings.Join(parts, " · "), cursor, total)
	return lipgloss.NewStyle().Foreground(colorTextDim).Italic(true).PaddingLeft(1).Render(txt)
}

// relativeTime converts an ISO 8601 timestamp to a human-readable "ago" string.
func relativeTime(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		// Try alternate format without timezone
		t, err = time.Parse("2006-01-02T15:04:05Z", iso)
		if err != nil {
			return iso
		}
	}
	now := time.Now().UTC()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 7*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", d)
	default:
		return t.Format("Jan 2")
	}
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
