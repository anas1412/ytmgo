package tui

import (
	"fmt"
	"strings"
	"time"

	"ytmgo/internal/player"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ─── Mouse click handling ──────────────────────────────────────────
//
// Layout reference for Stream & Library pages (must stay in sync with view.go):
//
//   y=0            header
//   y=1..N         panels (panelHeight)
//   y=N+1..N+5     player bar (4-5 lines: now-playing + progress + controls + borders)
//   y=N+6          nav bar (1 line)
//   y=N+7          status line (optional)
//   y=N+8          help bar
//
// All section heights are approximate because borders add variable padding.
// Click positions are best-effort, not pixel-perfect.

const (
	clickHeaderLines  = 1
	clickPanelStartY  = 1
	clickPlayerHeight = 5 // player bar is taller now (no download bar above it)
)

// handleMouse processes mouse wheel events and delegates clicks.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Wheel up/down (action is always press, identified by button)
	if msg.Button == tea.MouseButtonWheelUp {
		// Over the lyrics pane, the wheel scrolls the lyrics and
		// releases the auto-follow until the next track.
		if m.overLyricsPane(msg.X, msg.Y) {
			m.scrollLyrics(-3)
			return m, nil
		}
		switch m.activePage {
		case PageSettings:
			if !m.settingsEditField && m.settingsCursor > 0 {
				m.settingsCursor--
				m.clampSettingsOffset()
			}
		default:
			switch m.activePanel {
			case PanelSearch:
				switch m.activePage {
				case PageHistory:
					if m.historyCursor > 0 {
						m.historyCursor--
						m.clampHistoryOffset()
					}
				case PageFavorites:
					if m.favCursor > 0 {
						m.favCursor--
						m.clampFavoritesOffset()
					}
				case PageLibrary:
					if m.libraryCursor > 0 {
						m.libraryCursor--
						m.clampLibraryOffset()
					}
				default:
					if m.searchCursor > 0 {
						m.searchCursor--
						m.clampSearchOffset()
					}
				}
			case PanelQueue:
				if m.queueCursor > 0 {
					m.queueCursor--
					m.clampQueueOffset()
				}
			}
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		// Over the lyrics pane, the wheel scrolls the lyrics and
		// releases the auto-follow until the next track.
		if m.overLyricsPane(msg.X, msg.Y) {
			m.scrollLyrics(3)
			return m, nil
		}
		switch m.activePage {
		case PageSettings:
			if !m.settingsEditField && m.settingsCursor < len(settingDefs)-1 {
				m.settingsCursor++
				m.clampSettingsOffset()
			}
		default:
			switch m.activePanel {
			case PanelSearch:
				switch m.activePage {
				case PageHistory:
					maxIdx := len(m.history) - 1
					if m.historyCursor < maxIdx {
						m.historyCursor++
						m.clampHistoryOffset()
					}
				case PageFavorites:
					maxIdx := len(m.favorites) - 1
					if m.favCursor < maxIdx {
						m.favCursor++
						m.clampFavoritesOffset()
					}
				case PageLibrary:
					maxIdx := len(m.filteredLibrary()) - 1
					if m.libraryCursor < maxIdx {
						m.libraryCursor++
						m.clampLibraryOffset()
					}
				default:
					if m.searchCursor < m.streamListLen()-1 {
						m.searchCursor++
						m.clampSearchOffset()
					}
				}
			case PanelQueue:
				if m.queueCursor < m.queue.Len()-1 {
					m.queueCursor++
					m.clampQueueOffset()
				}
			}
		}
		return m, nil
	}

	// Left-button press → click handling
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.handleClick(msg.X, msg.Y)
	}

	return m, nil
}

// overLyricsPane reports whether (x, y) sits over the lyrics pane at
// the bottom of the right column. Geometry mirrors renderPanels: the
// column starts at y=1, the queue box renders queueContentH content
// rows inside 3 lines of chrome, and the lyrics box follows.
func (m Model) overLyricsPane(x, y int) bool {
	queueContentH, lyricsContentH := m.rightPanelSplit()
	if lyricsContentH <= 0 {
		return false
	}
	top := 1 + queueContentH + 3
	return x >= m.width/2 && y >= top && y < top+lyricsContentH+2
}

// scrollLyrics scrolls the lyrics pane by delta rows, releasing the
// auto-follow until the next track starts.
func (m *Model) scrollLyrics(delta int) {
	_, lyricsContentH := m.rightPanelSplit()
	maxOffset := max(0, len(m.lyricLines)-lyricsContentH)
	m.lyricsFollow = false
	m.lyricsOffset = max(0, min(m.lyricsOffset+delta, maxOffset))
}

// handleClick maps a mouse click at (x, y) to the relevant UI action.
// Returns (updated Model, optional Cmd).
func (m Model) handleClick(x, y int) (Model, tea.Cmd) {
	// ── Header (y=0) → page tabs or search input ──
	if y == 0 {
		// Replicate the tab rendering from renderHeader to find tab positions.
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
		var tabWidths []int
		for i, t := range tabs {
			hint := styleKeyHint.Render("[" + t.key + "]")
			label := styleNavTab.Render(t.label)
			if int(m.activePage) == i {
				label = styleNavTabActive.Render(t.label)
			}
			rendered := hint + " " + label
			renderedTabs = append(renderedTabs, rendered)
			tabWidths = append(tabWidths, lipgloss.Width(rendered))
		}
		tabsStr := strings.Join(renderedTabs, " ")
		tabsWidth := lipgloss.Width(tabsStr)

		// From renderHeader: gap = m.width - leftWidth - tabsWidth - 2,
		// then styleHeader adds PaddingLeft(1). Tabs start at
		// 1 + leftWidth + gap = m.width - tabsWidth - 1.
		tabsStartX := m.width - tabsWidth - 1
		if x >= tabsStartX && x < tabsStartX+tabsWidth {
			// Determine which tab was clicked by cumulative width.
			cumX := tabsStartX
			for i, tw := range tabWidths {
				if x >= cumX && x < cumX+tw {
					if m.activePage != Page(i) {
						m.switchPage(Page(i))
					}
					// Load history when clicking the History tab
					// (switchPage sets historyLoaded=false, so we need to
					// load synchronously here, matching keyboard "4" behavior)
					if Page(i) == PageHistory {
						m.loadPlayHistory()
					}
					return m, nil
				}
				cumX += tw + 1 // +1 for the joining space
			}
		}

		// Click in the left/search area of the header — focus search input.
		m.searchFocused = true
		m.searchInput.Focus()
		m.activePanel = PanelSearch
		return m, nil
	}

	// ── Settings page ──
	if m.activePage == PageSettings {
		panelHeight := m.panelHeight()
		panelsEnd := clickPanelStartY + panelHeight

		if y >= clickPanelStartY && y < panelsEnd {
			// Clicking in the panel area — unfocus search
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
			}

			// Items start after: border-top(1) + title-line(1) + implicit pad(1) = 3
			const clickItemOffsetY = 3
			// Each item is 4 lines from renderSettingsList: label, value, desc, blank
			const settingsLinesPerItem = 4

			midX := m.width / 2
			if x < midX {
				// Left panel: settings list
				idx := (y - clickItemOffsetY) / settingsLinesPerItem
				idx += m.settingsOffset
				if idx < 0 {
					idx = 0
				}
				if idx > len(settingDefs)-1 {
					idx = len(settingDefs) - 1
				}
				m.settingsCursor = idx
				m.clampSettingsOffset()
			}
			// Right panel is keyboard shortcuts (view-only) — nothing to click

			// ── Double-click detection ──
			if m.lastClickY == y && !m.lastClickAt.IsZero() && time.Since(m.lastClickAt) < 300*time.Millisecond {
				m.lastClickAt = time.Time{} // reset to prevent triple-click cascade
				return m.activateSettingsItem()
			}
			m.lastClickAt = time.Now()
			m.lastClickY = y
			return m, nil
		}

		// ── Player row (transport + seek + modes + volume) ──
		if y == panelsEnd+4 && m.width > 0 {
			return m.handlePlayerRowClick(x)
		}

		return m, nil
	}

	// ── Panels area (Stream & Library pages) ──
	panelHeight := m.panelHeight()
	panelsEnd := clickPanelStartY + panelHeight

	if y >= clickPanelStartY && y < panelsEnd {
		// Clicking in the panel area — unfocus search
		if m.searchFocused {
			m.searchFocused = false
			m.searchInput.Blur()
		}

		// Items start after: border-top(1) + title-line(1) + implicit pad(1) = 3
		const clickItemOffsetY = 3
		// Each row is 2 lines: title + artist
		const clickLinesPerItem = 2

		midX := m.width / 2
		if x < midX {
			// Left panel: search results / library / favorites
			m.activePanel = PanelSearch
			idx := (y - clickItemOffsetY) / clickLinesPerItem
			switch m.activePage {
			case PageHistory:
				idx += m.historyOffset
				switch {
				case idx < 0:
					idx = 0
				case idx >= len(m.history):
					idx = len(m.history) - 1
				}
				m.historyCursor = idx
			case PageFavorites:
				idx += m.favOffset
				switch {
				case idx < 0:
					idx = 0
				case idx >= len(m.favorites):
					idx = len(m.favorites) - 1
				}
				m.favCursor = idx
			case PageLibrary:
				tracks := m.filteredLibrary()
				idx += m.libraryOffset
				switch {
				case idx < 0:
					idx = 0
				case idx >= len(tracks):
					idx = len(tracks) - 1
				}
				m.libraryCursor = idx
			default:
				idx += m.searchOffset
				// Clamp against the active list (results, albums, or an
				// open album's tracks), not len(m.results).
				switch {
				case idx < 0:
					idx = 0
				case idx >= m.streamListLen():
					idx = m.streamListLen() - 1
				}
				m.searchCursor = idx
			}
		} else {
			// Right column: split into queue (top) and downloads (bottom).
			// rightPanelSplit is shared with renderPanels() so the click
			// boundary always matches what was drawn.
			queueContentH, _ := m.rightPanelSplit()
			// Queue sub-panel ends at: start (1) + queueHeight (queueContentH + 3)
			queueBorderY := clickPanelStartY + queueContentH + 3
			if y < queueBorderY {
				// Click landed in the queue sub-panel
				m.activePanel = PanelQueue
				idx := (y - clickItemOffsetY) / clickLinesPerItem
				idx += m.queueOffset
				switch {
				case idx < 0:
					idx = 0
				case m.queue.Len() == 0:
					idx = 0
				case idx >= m.queue.Len():
					idx = m.queue.Len() - 1
				}
				m.queueCursor = idx
			}
			// Click in the downloads sub-panel: not navigable, leave activePanel as-is
		}

		// ── Double-click detection ──
		// If the same panel row was clicked twice within 300ms, treat it as
		// an Enter (activate the focused item).
		if m.lastClickPanel == m.activePanel && m.lastClickY == y && !m.lastClickAt.IsZero() && time.Since(m.lastClickAt) < 300*time.Millisecond {
			m.lastClickAt = time.Time{} // reset to prevent triple-click cascade
			return m.activateFocusedItem()
		}
		m.lastClickAt = time.Now()
		m.lastClickY = y
		m.lastClickPanel = m.activePanel
		return m, nil
	}

	// ── Player row (transport + seek bar + modes + volume) ──
	// y layout: header(1) + panels(panelHeight) + status(1)
	//   + playerBar: border(1) + title(1) + album(1) + combined row(1) + border(1)
	//   + help(1)
	// Status is always rendered, so player starts at panelsEnd+1.
	playerRowY := panelsEnd + 4
	if y == playerRowY && m.width > 0 {
		return m.handlePlayerRowClick(x)
	}

	return m, nil
}

// handlePlayerRowClick maps a click on the combined player row. The
// zones come from playerRowLayout — the same builder the view renders
// from — so a click lands exactly where the pixel says it should.
func (m Model) handlePlayerRowClick(x int) (Model, tea.Cmd) {
	l := m.playerRowLayout()

	// ── Transport ──
	if x >= l.transportStart && x < l.transportEnd {
		switch {
		case x < l.prevEnd:
			if m.queue.Len() > 0 {
				return m, m.prevTrack()
			}
		case x < l.playEnd:
			return m, m.togglePlayPause()
		default:
			if m.queue.Len() > 0 {
				return m, m.nextTrack()
			}
		}
		return m, nil
	}

	// ── Seek bar ──
	if x >= l.barStart && x < l.barStart+l.barWidth {
		if m.playerState == player.StateStopped || m.duration <= 0 {
			return m, nil
		}
		pct := float64(x-l.barStart) / float64(l.barWidth)
		targetPos := pct * m.duration
		delta := targetPos - m.position
		if m.player != nil {
			m.player.Seek(delta)
		}
		// Optimistically update so the bar jumps immediately — the next
		// PositionMsg from the player will correct any discrepancy.
		m.position = targetPos
		m.lastPosition = targetPos
		m.lastPositionAt = time.Now()
		return m, nil
	}

	// ── Modes and volume ──
	if x >= l.rightStart && x < l.volEnd {
		switch {
		case x < l.shuffleEnd:
			return m, m.toggleShuffleAction()
		case x >= l.repeatStart && x < l.repeatEnd:
			return m, m.cycleRepeatAction()
		case x >= l.volStart && x < l.volDownEnd:
			cmd := m.changeVolume(-5)
			m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
			return m, cmd
		case x >= l.volUpStart && x < l.volEnd:
			cmd := m.changeVolume(+5)
			m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
			return m, cmd
		case !l.compact && x >= l.volDownEnd+1 && x < l.volUpStart-1:
			// Click on the volume bar — set proportionally.
			barStart := l.volDownEnd + 1
			barW := (l.volUpStart - 1) - barStart
			if barW > 0 {
				pct := float64(x-barStart) / float64(barW) * 100.0
				cmd := m.setVolumeTo(int(pct))
				m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
				return m, cmd
			}
		}
	}

	return m, nil
}

// activateFocusedItem replicates the Enter key behaviour for the current
// cursor position (model must already have cursor and activePanel set).
// Called by double-click detection in handleClick.
func (m Model) activateFocusedItem() (Model, tea.Cmd) {
	cmd := m.activateSelection()
	return m, cmd
}
