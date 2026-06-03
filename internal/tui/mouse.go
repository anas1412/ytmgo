package tui

import (
	"fmt"
	"strings"
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/settings"

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
		switch m.activePage {
		case PageSettings:
			if !m.settingsEditField && m.settingsCursor > 0 {
				m.settingsCursor--
				m.clampSettingsOffset()
			}
		default:
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
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelDown {
		switch m.activePage {
		case PageSettings:
			if !m.settingsEditField && m.settingsCursor < 6 {
				m.settingsCursor++
				m.clampSettingsOffset()
			}
		default:
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
		}
		return m, nil
	}

	// Left-button press → click handling
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
		return m.handleClick(msg.X, msg.Y)
	}

	return m, nil
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
			{"2", "Library"},
			{"3", "Settings"},
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
				if idx > 6 { // 7 items indexed 0-6
					idx = 6
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

		// ── Progress bar row (click to seek) ──
		progressY := panelsEnd + 3
		if y == progressY && m.width > 0 {
			return m.handleProgressClick(x)
		}

		// ── Player controls row ──
		controlsY := panelsEnd + 4
		if y == controlsY && m.width > 0 {
			return m.handleControlsClick(x)
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

	// ── Progress bar row (click to seek) ──
	progressY := panelsEnd + 3
	if y == progressY && m.width > 0 {
		return m.handleProgressClick(x)
	}

	// ── Player controls row ──
	// y layout: header(1) + panels(panelHeight) + status(1)
	//   + playerBar: border(1) + nowPlaying(1) + progress(1) + controls(1) + border(1)
	//   + help(1)
	// Status is always rendered, so player starts at panelsEnd+1.
	controlsY := panelsEnd + 4
	if y == controlsY && m.width > 0 {
		return m.handleControlsClick(x)
	}

	return m, nil
}

// handleControlsClick maps a click on the controls row (transport + modes + volume).
// Hit zones are based on the exact rendered character positions from renderControls,
// not approximated tolerances — 1:1 accurate like the tab click handler.
func (m Model) handleControlsClick(x int) (Model, tea.Cmd) {
	// ── Transport cluster (left) ──
	// Layout rendered by renderControls:
	//   transport = pHint + " " + prevLabel + "  " + spaceHint + " " + playLabel + "  " + nHint + " " + nextLabel
	//     text:  [p] ⏮ Prev  [space] ▶ Play  [n] ⏭ Next
	//                ←prev→     ←─play/pause─→     ←next→
	//
	// Player box has DoubleBorder (1 char left border) + PaddingLeft(2).
	// Content starts at terminal column 3.
	contentStartX := 3

	// Render transport components to measure zone boundaries dynamically.
	pHint := styleKeyHint.Render("[p]")
	prevLabel := styleCtrlBtn.Render("⏮ Prev")
	spaceHint := styleKeyHint.Render("[space]")
	// Use "▶ Play" (7 chars) for zone calculation; "⏸ Pause" (8 chars) is
	// close enough — the 1-char difference doesn't cross group boundaries.
	playLabel := styleCtrlBtn.Render("▶ Play")
	nHint := styleKeyHint.Render("[n]")
	nextLabel := styleCtrlBtn.Render("⏭ Next")

	prevGroupW := lipgloss.Width(pHint + " " + prevLabel)
	playGroupW := lipgloss.Width(spaceHint + " " + playLabel)
	nextGroupW := lipgloss.Width(nHint + " " + nextLabel)

	prevEnd := contentStartX + prevGroupW            // exclusive
	playEnd := prevEnd + 2 + playGroupW              // +2 for "  " between groups

	// ── Right cluster (modes + volume) ──
	sHint := styleKeyHint.Render("[s]")
	rHint := styleKeyHint.Render("[r]")
	volDownHint := styleKeyHint.Render("[-]")
	volUpHint := styleKeyHint.Render("[+]")
	volBar := renderVolumeBar(m.volume, 8)

	var repeatText string
	switch {
	case m.queue.IsRepeat():
		repeatText = "🔁 ONE"
	case m.queue.IsRepeatAll():
		repeatText = "🔁 ALL"
	default:
		repeatText = "🔁 OFF"
	}

	shuffleLabel := sHint + " " + "🔀 SHFL"                           // "[s] 🔀 SHFL"
	repeatLabel := rHint + " " + repeatText                            // "[r] 🔁 ..."
	volLabel := volDownHint + " " + volBar + " " + fmt.Sprintf("%d%%", m.volume) + " " + volUpHint
	right := lipgloss.JoinHorizontal(lipgloss.Left, shuffleLabel, "  ", repeatLabel, "  ", volLabel)
	rightW := lipgloss.Width(right)

	transportW := prevGroupW + 2 + playGroupW + 2 + nextGroupW
	contentW := m.width - 6 // from renderControls
	sepW := 1                // "│"
	gap := contentW - transportW - rightW - sepW
	if gap < 2 {
		gap = 2
	}
	rightStartX := contentStartX + transportW + gap + sepW

	// ── Transport clicks (queue required) ──
	if m.queue.Len() > 0 && x < rightStartX {
		switch {
		case x >= contentStartX && x < prevEnd: // prev zone
			if m.position > 3 {
				// Restart current track
				oldPos := m.position
				m.position = 0
				if m.player != nil {
					m.player.Seek(-oldPos)
				}
				m.setStatus("Restarting")
				return m, nil
			}
			if _, ok := m.queue.Prev(); !ok {
				return m, nil
			}
			m.queueCursor = m.queue.CurrentIndex()
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
			}
			return m, nil

		case x >= prevEnd && x < playEnd: // play/pause zone
			if m.player != nil {
				m.player.Pause()
				m.playerState = m.player.State()
			} else {
				if m.playerState == player.StatePlaying {
					m.playerState = player.StatePaused
				} else {
					m.playerState = player.StatePlaying
				}
			}
			m.lastPositionAt = time.Now()
			if m.playerState == player.StatePlaying {
				return m, playerTickCmd()
			}
			return m, nil

		default: // next zone: x >= playEnd && x < rightStartX
			if _, ok := m.queue.Next(); !ok {
				return m, nil
			}
			m.queueCursor = m.queue.CurrentIndex()
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
			}
			return m, nil
		}
	}

	// Empty queue: only play/pause still works (via the player directly).
	if m.queue.Len() == 0 && x >= prevEnd && x < playEnd {
		if m.player != nil {
			m.player.Pause()
			m.playerState = m.player.State()
		}
		m.lastPositionAt = time.Now()
		if m.playerState == player.StatePlaying {
			return m, playerTickCmd()
		}
		return m, nil
	}

	// ── Right cluster clicks ──
	if x >= rightStartX {
		shuffleW := lipgloss.Width(shuffleLabel)
		repeatW := lipgloss.Width(repeatLabel)
		volLabelW := lipgloss.Width(volLabel)

		shuffleEnd := rightStartX + shuffleW         // exclusive
		repeatStart := shuffleEnd + 2                 // after "  "
		repeatEnd := repeatStart + repeatW            // exclusive
		volStart := repeatEnd + 2                     // after "  "
		volEnd := volStart + volLabelW                // exclusive

		switch {
		case x < shuffleEnd:
			// ── Shuffle toggle ──
			m.queue.ToggleShuffle()
			m.modeFlashTarget = "shuffle"
			m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
			if m.queue.IsShuffle() {
				m.setStatus("Shuffle: ON")
			} else {
				m.setStatus("Shuffle: OFF")
			}
			return m, nil

		case x >= repeatStart && x < repeatEnd:
			// ── Repeat cycle ──
			if !m.queue.IsRepeat() && !m.queue.IsRepeatAll() {
				m.queue.ToggleRepeat() // → repeat: true
				m.setStatus("Repeat: ONE")
			} else if m.queue.IsRepeat() {
				m.queue.ToggleRepeat()    // repeat: false
				m.queue.ToggleRepeatAll() // repeatAll: true
				m.setStatus("Repeat: ALL")
			} else {
				m.queue.ToggleRepeatAll() // repeatAll: false
				m.setStatus("Repeat: OFF")
			}
			m.modeFlashTarget = "repeat"
			m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
			return m, nil

		case x >= volStart && x < volEnd:
			// ── Volume sub-regions ──
			volDownW := lipgloss.Width(volDownHint) // "[-]" = 3
			volUpW := lipgloss.Width(volUpHint)      // "[+]" = 3
			volDownEnd := volStart + volDownW         // exclusive end of "[-]"
			volUpStart := volEnd - volUpW            // start of "[+]"

			switch {
			case x < volDownEnd:
				m.volume = max(m.volume-5, 0)
				if m.player != nil {
					m.player.SetVolume(m.volume)
				}
				m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
				return m, nil

			case x >= volUpStart:
				m.volume = min(m.volume+5, 100)
				if m.player != nil {
					m.player.SetVolume(m.volume)
				}
				m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
				return m, nil

			default:
				// Click on the volume bar or percentage — set proportionally.
				barStart := volStart + volDownW + 1  // after "[-] "
				barEnd := volEnd - volUpW - 1         // before " [+]"
				barWidth := barEnd - barStart
				if barWidth > 0 {
					pct := float64(x-barStart) / float64(barWidth) * 100.0
					m.volume = int(pct)
					if m.volume < 0 {
						m.volume = 0
					}
					if m.volume > 100 {
						m.volume = 100
					}
					if m.player != nil {
						m.player.SetVolume(m.volume)
					}
					m.setStatus(fmt.Sprintf("Volume: %d%%", m.volume))
				}
				return m, nil
			}
		}
	}

	return m, nil
}

// activateFocusedItem replicates the Enter key behaviour for the current
// cursor position (model must already have cursor and activePanel set).
// Called by double-click detection in handleClick.
func (m Model) activateFocusedItem() (Model, tea.Cmd) {
	switch m.activePage {
	case PageLibrary:
		if m.activePanel == PanelSearch {
			tracks := m.filteredLibrary()
			if len(tracks) > 0 && m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
				t := tracks[m.libraryCursor]
				m.queue.Add(t)

				if m.playerState == player.StateStopped {
					m.queue.SetCurrentIndex(m.queue.Len() - 1)
					m.queueCursor = m.queue.CurrentIndex()
					m.clampQueueOffset()
					if playCmd := m.startTrackPlayback(t.FilePath, t.Title, t.DurationSec); playCmd != nil {
						return m, playCmd
					}
				}

				m.setStatus("Added to queue: " + t.Title)
			}
			return m, nil
		}
		if m.activePanel == PanelQueue {
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
			}
			return m, nil
		}
		return m, nil

	default: // Stream page
		switch m.activePanel {
		case PanelSearch:
			if len(m.results) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.results) {
				r := m.results[m.searchCursor]
				t := m.resolveTrack(r)
				m.queue.Add(t)

				var cmds []tea.Cmd

				if m.playerState == player.StateStopped {
					m.queue.SetCurrentIndex(m.queue.Len() - 1)
					m.queueCursor = m.queue.CurrentIndex()
					m.clampQueueOffset()
					if playCmd := m.startTrackPlayback(t.PlayURL(), t.Title, t.DurationSec); playCmd != nil {
						cmds = append(cmds, playCmd)
					}
				} else {
					m.setStatus("Added to queue: " + t.Title)
				}

				if m.settings.PlaybackMode == settings.PlaybackOffline {
					m.ensureDownloader()
					m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir())
					cmds = append(cmds, downloadCmd(m.downloader))
				} else if m.settings.PlaybackMode == settings.PlaybackHybrid && !t.Downloaded {
					m.ensureDownloader()
					m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir())
					cmds = append(cmds, downloadCmd(m.downloader))
				}

				if len(cmds) == 0 {
					return m, nil
				}
				return m, tea.Batch(cmds...)
			}
			return m, nil

		case PanelQueue:
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
			}
			return m, nil

		default:
			return m, nil
		}
	}
}

// activateSettingsItem replicates the Enter key behaviour for the currently
// focused settings item. Called by double-click on the settings page.
func (m Model) activateSettingsItem() (Model, tea.Cmd) {
	if m.settingsEditField {
		// Finish editing string field
		newVal := m.settingsEditInput.Value()
		m.settingsEditField = false
		m.settingsEditInput.Blur()
		switch m.settingsCursor {
		case 4: // Download Dir
			m.settings.DownloadDir = newVal
		case 5: // Cookie Browser
			m.settings.CookieBrowser = newVal
		case 6: // User-Agent
			m.settings.UserAgent = newVal
		}
		return m, tea.Batch(saveSettingsCmd(m.settings))
	}
	switch m.settingsCursor {
	case 0: // Playback Mode (cycle)
		m.settings.PlaybackMode = (m.settings.PlaybackMode + 1) % 3
		return m, tea.Batch(saveSettingsCmd(m.settings))
	case 1: // Show Quotes (boolean)
		m.settings.ShowQuotes = !m.settings.ShowQuotes
		m.tickCount = 0
		if m.settings.ShowQuotes {
			m.fallbackIdx = 0
			m.currentQuote = fallbackQuotes[0]
		} else {
			m.advanceTip()
		}
		return m, tea.Batch(saveSettingsCmd(m.settings))
	case 2, 3: // Volume / Search Limit (numbers — Enter does nothing)
		return m, nil
	case 4, 5, 6: // Download Dir / Cookie Browser / User-Agent (strings)
		m.startSettingsEdit()
		return m, nil
	}
	return m, nil
}

// handleProgressClick maps a click on the progress bar row to a seek position.
// The bar layout matches renderPlayerBar: [h] ▓▓▓▓░░░░  MM:SS / M:SS [l]
func (m Model) handleProgressClick(x int) (Model, tea.Cmd) {
	if m.playerState == player.StateStopped || m.duration <= 0 {
		return m, nil
	}

	innerW := m.width - 6
	contentStartX := 3 // double border (1) + left padding (2)

	hHint := styleKeyHint.Render("[h]")
	lHint := styleKeyHint.Render("[l]")

	// Replicate timeInfo from renderPlayerBar to measure rightPart width.
	currentStr := formatTime(m.position)
	totalStr := formatDuration(int(m.duration))
	if len(currentStr) < len(totalStr) {
		currentStr = strings.Repeat(" ", len(totalStr)-len(currentStr)) + currentStr
	}
	timeInfo := currentStr + " / " + totalStr
	rightPart := styleTime.Render(timeInfo)

	barWidth := innerW - lipgloss.Width(rightPart) - lipgloss.Width(hHint) - lipgloss.Width(lHint) - 5
	if barWidth < 10 {
		barWidth = 10
	}

	barStartX := contentStartX + lipgloss.Width(hHint) + 1
	barEndX := barStartX + barWidth

	if x < barStartX || x >= barEndX {
		return m, nil
	}

	// Map click position to seek position.
	relX := x - barStartX
	pct := float64(relX) / float64(barWidth)
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
	m.setStatus(fmt.Sprintf("Seeked to %s", formatTime(targetPos)))

	return m, nil
}
