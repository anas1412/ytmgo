package tui

import (
	"strings"

	"ytmgo/internal/player"

	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
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
		switch m.activePanel {
		case PanelSearch:
			if m.activePage == PageLibrary {
				if m.libraryCursor > 0 {
					m.libraryCursor--
					m.clampLibraryOffset()
				}
			} else if m.searchCursor > 0 {
				m.searchCursor--
				m.clampSearchOffset()
			}
		case PanelQueue:
			if m.queueCursor > 0 {
				m.queueCursor--
				m.clampQueueOffset()
			}
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		switch m.activePanel {
		case PanelSearch:
			if m.activePage == PageLibrary {
				maxIdx := len(m.filteredLibrary()) - 1
				if m.libraryCursor < maxIdx {
					m.libraryCursor++
					m.clampLibraryOffset()
				}
			} else if m.searchCursor < len(m.results)-1 {
				m.searchCursor++
				m.clampSearchOffset()
			}
		case PanelQueue:
			if m.queueCursor < m.queue.Len()-1 {
				m.queueCursor++
				m.clampQueueOffset()
			}
		}
		return m, nil
	}

	// Left-button press → click handling
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.handleClick(msg.X, msg.Y), nil
	}

	return m, nil
}

// handleClick maps a mouse click at (x, y) to the relevant UI action.
func (m Model) handleClick(x, y int) Model {
	// ── Header (y=0) → page tabs or search input ──
	if y == 0 {
		// Replicate the tab rendering from renderHeader to find tab positions.
		tabs := []string{"1 Stream", "2 Library", "3 Settings"}
		var renderedTabs []string
		var tabWidths []int
		for i, tab := range tabs {
			var rendered string
			if int(m.activePage) == i {
				rendered = styleNavTabActive.Render(tab)
			} else {
				rendered = styleNavTab.Render(tab)
			}
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
					return m
				}
				cumX += tw + 1 // +1 for the joining space
			}
		}

		// Click in the left/search area of the header — focus search input.
		m.searchFocused = true
		m.searchInput.Focus()
		m.activePanel = PanelSearch
		return m
	}

	// Settings page has no clickable panels below the header.
	if m.activePage == PageSettings {
		return m
	}

	// ── Panels area ──
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
			// Left panel: search results / library
			m.activePanel = PanelSearch
			idx := (y - clickItemOffsetY) / clickLinesPerItem
			if m.activePage == PageLibrary {
				tracks := m.filteredLibrary()
				idx += m.libraryOffset
				switch {
				case idx < 0:
					idx = 0
				case idx >= len(tracks):
					idx = len(tracks) - 1
				}
				m.libraryCursor = idx
			} else {
				idx += m.searchOffset
				switch {
				case idx < 0:
					idx = 0
				case idx >= len(m.results):
					idx = len(m.results) - 1
				}
				m.searchCursor = idx
			}
		} else {
			// Right column: split into queue (top) and downloads (bottom).
			// Must match renderPanels() exactly. Each sub-panel: title (1) +
			// content (N) + borders (2) = N + 3 total lines.
			totalSubContentH := panelHeight - 6
			if totalSubContentH < 0 {
				totalSubContentH = 0
			}
			queueContentH := totalSubContentH / 2
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
		return m
	}

	// ── Player controls (bottom region) — rough play/pause toggle ──
	playerStartY := panelsEnd // no download bar before player
	playerEndY := playerStartY + clickPlayerHeight
	if y >= playerStartY && y <= playerEndY && m.width > 0 {
		if x < m.width/3 {
			if m.player != nil {
				m.player.Pause()
				m.playerState = m.player.State()
			} else {
				// Dev mode: toggle cached state
				if m.playerState == player.StatePlaying {
					m.playerState = player.StatePaused
				} else {
					m.playerState = player.StatePlaying
				}
			}
		}
	}

	return m
}
