package tui

import (
	"fmt"
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
	ver "ytmgo/internal/version"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleKey processes all tea.KeyMsg events. Extracted from Update so each
// message-handler family lives in its own focused file.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Search-input focus: route letters/numbers to textinput ──
	// Checked *before* global keys so typing "o", "R", "1" etc. in
	// the search box works instead of triggering page/action shortcuts.
	if m.searchFocused {
		switch msg.String() {
		case "esc":
			m.searchFocused = false
			m.searchInput.Blur()
			m.activePanel = PanelSearch
			return m, nil
		case "enter":
			m.searchFocused = false
			m.searchInput.Blur()
			m.activePanel = PanelSearch
			query := m.searchInput.Value()
			if m.activePage == PageLibrary {
				// On Library page, Enter just exits the search field (filtering already happened live)
				return m, nil
			}
			if query != "" {
				m.recsSeq++ // invalidate any pending recommendations
				m.showingRecommendations = false
				m.results = nil
				m.isSearching = true
				m.searchCursor = 0
				m.err = nil
				return m, searchCmd(query, m.settings.SearchLimit, m.tidalClient)
			}
			return m, nil
		case "tab":
			// Tab → move to search results list
			m.searchFocused = false
			m.searchInput.Blur()
			m.activePanel = PanelSearch
			return m, nil
		}
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// When typing in library mode, clamp cursor to filtered results
		if m.activePage == PageLibrary {
			m.clampLibraryOffset()
		}
		return m, cmd
	}

	// ── Settings string editing: route letters/numbers to textinput ──
	// Checked before global keys so typing "o" in a path field works.
	if m.settingsEditField {
		switch msg.String() {
		case "esc":
			m.settingsEditField = false
			m.settingsEditInput.Blur()
			return m, nil
		case "enter":
			// Handled in the Enter case below — let it fall through
		default:
			var cmd tea.Cmd
			m.settingsEditInput, cmd = m.settingsEditInput.Update(msg)
			return m, cmd
		}
	}

	// ── Global keys (only fire when no text input is focused) ──
	if handled, cmd := m.handleGlobalKey(msg); handled {
		return m, cmd
	}

	// When confirming a destructive action, route the key press.
	// Navigation keys 1/2/3 cancel and pass through.
	if m.isConfirming() {
		key := msg.String()
		// Check if the pressed key confirms the pending action
		confirmed := false
		switch m.confirmAction {
		case confirmClearQueue:
			confirmed = key == "D"
		case confirmDeleteTrack:
			// Delete-track uses status-bar confirmation: Enter to confirm,
			// Esc to cancel, all other keys ignored so the prompt persists.
			confirmed = key == "enter"
		case confirmUpdate:
			// Update uses status-bar confirmation: Enter to confirm,
			// Esc to cancel, all other keys ignored so the prompt persists.
			confirmed = key == "enter"
		}

		switch {
		case confirmed:
			return m.executeConfirmedAction()
		case key == "esc":
			m.clearConfirm()
			m.setStatus("Cancelled")
			return m, nil
		case key == "1" || key == "2" || key == "3":
			// Cancel confirmation and let navigation key fall through
			m.clearConfirm()
		case m.confirmAction == confirmDeleteTrack || m.confirmAction == confirmUpdate:
			// Keep the status-bar prompt visible until Enter or Esc
			return m, nil
		default:
			// For other confirmations, any key cancels
			m.clearConfirm()
			m.setStatus("Cancelled")
			return m, nil
		}
	}

	// ── Global keybindings ───────────────────────────────
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		m.Shutdown()
		return m, tea.Quit

	case "?":
		m.switchPage(PageSettings)
		return m, nil

	case "tab":
		switch m.activePage {
		case PageSettings:
			// Tab does nothing on settings — arrows navigate the list.
			return m, nil
		case PageFavorites, PageLibrary, PageHistory:
			// Favorites/Library/History page: search input ↔ list
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
				m.activePanel = PanelSearch
			} else if m.activePanel == PanelSearch {
				m.activePanel = PanelQueue
			} else {
				m.activePanel = PanelSearch
				m.searchFocused = true
				m.searchInput.Focus()
			}
		default: // PageStream
			// 3-state cycle: search input → search results → queue → search input
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
				m.activePanel = PanelSearch
			} else if m.activePanel == PanelSearch {
				m.activePanel = PanelQueue
			} else {
				m.activePanel = PanelSearch
				m.searchFocused = true
				m.searchInput.Focus()
			}
		}
		return m, nil

	case "esc":
		if m.activePage == PageSettings && m.settingsEditField {
			// Cancel inline editing on Settings page
			m.settingsEditField = false
			m.settingsEditInput.Blur()
			return m, nil
		}
		// Otherwise Esc does nothing outside edit mode.
		return m, nil

	// ── Panel navigation ─────────────────────────────────
	case "up", "k":
		// Settings page: navigate settings list
		if m.activePage == PageSettings && !m.settingsEditField {
			if m.settingsCursor > 0 {
				m.settingsCursor--
				m.clampSettingsOffset()
			}
			return m, nil
		}
		// Panel navigation
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
		return m, nil

	case "down", "j":
		// Settings page: navigate settings list
		if m.activePage == PageSettings && !m.settingsEditField {
			if m.settingsCursor < 7 { // 8 items indexed 0-7
				m.settingsCursor++
				m.clampSettingsOffset()
			}
			return m, nil
		}
		// Panel navigation
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
				if m.searchCursor < len(m.results)-1 {
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
		return m, nil

	case "enter":
		// ── Settings page: toggle/edit setting ────────────────
		if m.activePage == PageSettings {
			if m.settingsEditField {
				// Finish editing string field
				newVal := m.settingsEditInput.Value()
				m.settingsEditField = false
				m.settingsEditInput.Blur()
				switch m.settingsCursor {
				case 5: // Download Dir
					m.settings.DownloadDir = newVal
				case 6: // TIDAL Proxy URL
					m.settings.TidalProxyURL = newVal
					m.reinitTidalClient()
				case 7: // Download Format (should not reach here, cycles on Enter)
					// no-op; format is cycled, not typed
				}
				return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
			}
			switch m.settingsCursor {
			case 0: // Playback Mode (cycle)
				m.settings.PlaybackMode = (m.settings.PlaybackMode + 1) % 3
				return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
			case 1: // Show Quotes (boolean)
				m.settings.ShowQuotes = !m.settings.ShowQuotes
				m.tickCount = 0
				if m.settings.ShowQuotes {
					// Start from first fallback quote
					m.fallbackIdx = 0
					m.currentQuote = fallbackQuotes[0]
				} else {
					// Advance to next tip
					m.advanceTip()
				}
				return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
			case 2: // Discord RPC (boolean)
				m.settings.DiscordRPCEnabled = !m.settings.DiscordRPCEnabled
				m.reinitDiscordRPC()
				return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
			case 3, 4: // Volume / Search Limit (numbers — Enter does nothing, use +/-)
				return m, nil
			case 5, 6: // Download Dir / TIDAL Proxy URL (strings)
				m.startSettingsEdit()
				return m, nil
			case 7: // Download Format (cycle)
				switch m.settings.DownloadFormat {
				case settings.FormatM4A:
					m.settings.DownloadFormat = settings.FormatMP3
				default:
					m.settings.DownloadFormat = settings.FormatM4A
				}
				if m.downloader != nil {
					m.downloader.SetFormat(m.settings.DownloadFormat)
				}
				return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
			}
			return m, nil
		}

		// ── Other pages (Stream / Library / Favorites) ────────
		switch m.activePage {
		case PageFavorites:
			if m.activePanel == PanelSearch && !m.searchFocused {
			if len(m.favorites) > 0 && m.favCursor >= 0 && m.favCursor < len(m.favorites) {
				t := m.favorites[m.favCursor]
				m.queue.Add(t)

				if m.playerState == player.StateStopped {
					m.queue.SetCurrentIndex(m.queue.Len() - 1)
					m.queueCursor = m.queue.CurrentIndex()
					m.clampQueueOffset()
					if playCmd := m.resolveAndPlayCmd(t); playCmd != nil {
						return m, playCmd
					}
				}

				m.setStatus("Added to queue: " + t.Title)
			}
				return m, saveQueueCmd(m.db, m.queue)
			}
			// On Favorites page queue panel: play selected queue item
			if m.activePanel == PanelQueue && !m.searchFocused {
				if playCmd := m.playSelectedQueueItem(); playCmd != nil {
					return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
				}
				return m, nil
			}
			return m, nil

		case PageLibrary:
			if m.activePanel == PanelSearch && !m.searchFocused {
				// Add library track to queue, auto-play only if stopped + first track.
				// Only SetCurrentIndex when a track actually starts playing —
				// CurrentIndex is the single source of truth for what mpv is
				// doing, so it must never point at a track that isn't playing.
				// queueCursor is NOT moved — the user's selection stays where
				// they left it. The new track is visible at the bottom.
				tracks := m.filteredLibrary()
				if len(tracks) > 0 && m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
					t := tracks[m.libraryCursor]
					m.queue.Add(t)

					if m.playerState == player.StateStopped {
						// First track in an empty queue — auto-play
						m.queue.SetCurrentIndex(m.queue.Len() - 1)
						m.queueCursor = m.queue.CurrentIndex()
						m.clampQueueOffset()
						if playCmd := m.resolveAndPlayCmd(t); playCmd != nil {
							return m, playCmd
						}
					}

					m.setStatus("Added to queue: " + t.Title)
				}
				return m, saveQueueCmd(m.db, m.queue)
			}
			// On Library page download queue panel: play selected completed download
			if m.activePanel == PanelQueue && !m.searchFocused {
				if playCmd := m.playSelectedQueueItem(); playCmd != nil {
					return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
				}
				return m, nil
			}
			return m, nil

		default:
			// ── Stream page (default) ──────────────────────────
			switch m.activePanel {
			case PanelSearch:
				if len(m.results) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.results) {
					r := m.results[m.searchCursor]
					// resolveTrack consults the local library so a track
					// that already exists on disk plays the local file
					// instead of re-streaming from YouTube.
				t := m.resolveTrack(r)
					m.queue.Add(t)

					cmds := []tea.Cmd{saveQueueCmd(m.db, m.queue)}

					// Auto-play only if nothing was playing (smart start).
					if m.playerState == player.StateStopped {
						m.queue.SetCurrentIndex(m.queue.Len() - 1)
						m.queueCursor = m.queue.CurrentIndex()
						m.clampQueueOffset()
						if playCmd := m.resolveAndPlayCmd(t); playCmd != nil {
							cmds = append(cmds, playCmd)
						}
					} else {
						m.setStatus("Added to queue: " + t.Title)
					}

					// Download handling (separate from auto-play).
					// Enqueue the track; if the URL isn't known yet,
					// the downloader will resolve it in the background
					// (see downloader.runJob).
					if m.settings.PlaybackMode == settings.PlaybackOffline ||
						(m.settings.PlaybackMode == settings.PlaybackHybrid && !t.Downloaded) {
						m.ensureDownloader()
						m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir(), r.CoverURL)
						cmds = append(cmds, downloadCmd(m.downloader))
					}

					if len(cmds) == 0 {
						return m, nil
					}
					return m, tea.Batch(cmds...)
				}

			case PanelQueue:
				if playCmd := m.playSelectedQueueItem(); playCmd != nil {
					return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
				}
			}
			return m, nil
		}

	case " ":
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
		// Re-anchor the smooth-progress timer and start the fast
		// redraws again on resume (or keep it running through the
		// pause tap if we never actually paused).
		m.lastPositionAt = time.Now()
		m.updateDiscordRPC()
		if m.playerState == player.StatePlaying {
			return m, playerTickCmd()
		}
		return m, nil

	case "n", "right":
		if m.queue.Len() == 0 {
			return m, nil
		}
		if _, ok := m.queue.Next(); !ok {
			return m, nil
		}
		m.queueCursor = m.queue.CurrentIndex()
		if playCmd := m.playSelectedQueueItem(); playCmd != nil {
			return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
		}
		return m, saveQueueCmd(m.db, m.queue)

	case "p", "left":
		if m.queue.Len() == 0 {
			return m, nil
		}
		// If more than 3s into a track, restart it instead of going back
		if m.position > 3 {
			oldPos := m.position
			m.position = 0
			if m.player != nil {
				m.player.Seek(-oldPos)
			}
			m.setStatus("Restarting")
			return m, nil
		}
		// Less than 3s in — go to the previous track
		if _, ok := m.queue.Prev(); !ok {
			return m, nil
		}
		m.queueCursor = m.queue.CurrentIndex()
		if playCmd := m.playSelectedQueueItem(); playCmd != nil {
			return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
		}
		return m, saveQueueCmd(m.db, m.queue)

	case "l", "ctrl+f":
		m.position = min(m.position+5, m.duration)
		if m.player != nil {
			m.player.Seek(5)
		}
		m.updateDiscordRPC()
		return m, nil

	case "L":
		// L now behaves the same as "3" (always go to Library, consistent behavior)
		if m.activePage != PageLibrary {
			m.switchPage(PageLibrary)
			msg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
			if len(m.library) == 0 {
				msg = "No downloaded tracks"
			}
			m.setStatus(msg)
		}
		return m, nil

	case "f":
		// Toggle favorite on the highlighted track.
		switch {
		case m.activePage == PageFavorites && m.activePanel == PanelSearch:
			if len(m.favorites) > 0 && m.favCursor >= 0 && m.favCursor < len(m.favorites) {
				return m, m.toggleFavorite(m.favorites[m.favCursor])
			}
		case m.activePage == PageLibrary && m.activePanel == PanelSearch:
			tracks := m.filteredLibrary()
			if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
				return m, m.toggleFavorite(tracks[m.libraryCursor])
			}
		case m.activePanel == PanelQueue && m.queue.Len() > 0 && m.queueCursor >= 0 && m.queueCursor < m.queue.Len():
			t := m.queue.Tracks()[m.queueCursor]
			return m, m.toggleFavorite(t)
		default:
			// Stream page search results or recommendations
			if m.activePage != PageSettings && m.activePanel == PanelSearch && len(m.results) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.results) {
				r := m.results[m.searchCursor]
				t := m.resolveTrack(r)
				return m, m.toggleFavorite(t)
			}
		}
		return m, nil

	case "h", "ctrl+b":
		m.position = max(m.position-5, 0)
		if m.player != nil {
			m.player.Seek(-5)
		}
		return m, nil

	case "+", "=":
		// Settings page: adjust number settings
		if m.activePage == PageSettings && !m.settingsEditField {
			switch m.settingsCursor {
			case 3: // Default Volume
				if m.settings.DefaultVolume < 100 {
					m.settings.DefaultVolume = min(m.settings.DefaultVolume+5, 100)
					if m.player != nil {
						m.player.SetVolume(m.settings.DefaultVolume)
					}
					m.volume = m.settings.DefaultVolume
				}
			case 4: // Search Limit
				m.settings.SearchLimit = min(m.settings.SearchLimit+5, 100)
			}
			return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
		}
		// Global: volume up
		m.volume = min(m.volume+5, 100)
		if m.player != nil {
			m.player.SetVolume(m.volume)
		}
		return m, nil

	case "-", "_":
		// Settings page: adjust number settings
		if m.activePage == PageSettings && !m.settingsEditField {
			switch m.settingsCursor {
			case 3: // Default Volume
				if m.settings.DefaultVolume > 0 {
					m.settings.DefaultVolume = max(m.settings.DefaultVolume-5, 0)
					if m.player != nil {
						m.player.SetVolume(m.settings.DefaultVolume)
					}
					m.volume = m.settings.DefaultVolume
				}
			case 4: // Search Limit
				m.settings.SearchLimit = max(m.settings.SearchLimit-5, 5)
			}
			return m, tea.Batch(saveSettingsCmd(m.db, m.settings))
		}
		// Global: volume down
		m.volume = max(m.volume-5, 0)
		if m.player != nil {
			m.player.SetVolume(m.volume)
		}
		return m, nil

	case "x":
		// Download a track for offline use.
		// Works from either the search results panel (download the highlighted result)
		// or the queue panel (download the highlighted queue track).
		switch {
		case m.activePage == PageStream && m.activePanel == PanelSearch:
			if len(m.results) == 0 || m.searchCursor < 0 || m.searchCursor >= len(m.results) {
				return m, nil
			}
			r := m.results[m.searchCursor]
			t := m.resolveTrack(r)
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			// IsDownloaded checks the actual filesystem using the
			// current download format (m4a/mp3), so switching format
			// in Settings correctly allows re-downloading in the new
			// format. The old t.Downloaded guard from resolveTrack
			// was format-agnostic and blocked re-downloads — removed.
			if m.downloader.IsDownloaded(t.ID, t.Title, r.Uploader, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: r.Uploader,
					CoverURL: r.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, r.Uploader, t.URL, m.downloadDir(), r.CoverURL)
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageStream && m.activePanel == PanelQueue && m.queue.Len() > 0:
			if m.queueCursor < 0 || m.queueCursor >= m.queue.Len() {
				return m, nil
			}
			t := m.queue.Tracks()[m.queueCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(t.ID, t.Title, t.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: t.Artist,
					CoverURL: t.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir(), t.CoverURL)
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageFavorites && m.activePanel == PanelSearch:
			if len(m.favorites) == 0 || m.favCursor < 0 || m.favCursor >= len(m.favorites) {
				return m, nil
			}
			t := m.favorites[m.favCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(t.ID, t.Title, t.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: t.Artist,
					CoverURL: t.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir(), t.CoverURL)
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageHistory && m.activePanel == PanelSearch:
			if len(m.history) == 0 || m.historyCursor < 0 || m.historyCursor >= len(m.history) {
				return m, nil
			}
			e := m.history[m.historyCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(e.TrackID) {
				m.setStatus("Already in download queue: " + e.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(e.TrackID, e.Title, e.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + e.Title)
				return m, nil
			}
			// History entries don't carry a YouTube URL — always resolve.
			m.pendingResolve = &pendingDownloadResolve{
				TrackID:  e.TrackID,
				Title:    e.Title,
				Uploader: e.Artist,
				CoverURL: e.CoverURL,
				Action:   "download",
			}
			m.setStatus("Fetching URL…")
			return m, resolveURLCmd(e.Artist, e.Title, m.pendingResolve)
		}
		return m, nil

	case "d", "delete":
		if m.activePanel == PanelQueue && m.queue.Len() > 0 {
			idx := m.queueCursor
			removed := m.queue.Remove(idx)
			if removed && m.queue.Len() == 0 {
				m.queueCursor = 0
				m.playerState = player.StateStopped
				m.position = 0
				m.duration = 0
				m.updateDiscordRPC()
			} else {
				if m.queueCursor >= m.queue.Len() {
					m.queueCursor = max(0, m.queue.Len()-1)
				}
			}
			if m.queue.CurrentIndex() < 0 {
				m.playerState = player.StateStopped
				if m.player != nil {
					m.player.Stop()
				}
				m.updateDiscordRPC()
			}
			return m, saveQueueCmd(m.db, m.queue)
		}
		// Library page: delete a downloaded track from disk (requires confirmation)
		if m.activePage == PageLibrary && m.activePanel == PanelSearch && !m.searchFocused {
			tracks := m.filteredLibrary()
			if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
				t := tracks[m.libraryCursor]
				m.startConfirm(confirmDeleteTrack, t.Title)
				// Styled confirmation: orange bullet, white action, mint Enter, pink Esc
				bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
				action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Delete")
				title := lipgloss.NewStyle().Foreground(colorText).Render("\"" + t.Title + "\"?")
				enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
				enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
				escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
				escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
				m.setStatus(bullet + " " + action + " " + title + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			}
			return m, nil
		}
		return m, nil

	case "D":
		if m.queue.Len() == 0 {
			return m, nil
		}
		if !m.isConfirming() {
			m.startConfirm(confirmClearQueue, "")
			m.setStatus("Press D again to clear the entire queue")
			return m, nil
		}
		// Already confirmed — execute
		m.clearConfirm()
		m.queue.Clear()
		m.queueCursor = 0
		m.playerState = player.StateStopped
		m.position = 0
		m.duration = 0
		if m.player != nil {
			m.player.Stop()
		}
		m.updateDiscordRPC()
		m.setStatus("Queue cleared")
		return m, nil

	case "U":
		if m.updateAvailable != "" && m.updateAvailable != "latest" {
			// Update already known — start confirmation immediately
			m.startConfirm(confirmUpdate, m.updateAvailable)
			// Styled confirmation: orange bullet, white action, mint Enter, pink Esc
			bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
			action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Update to")
			verStr := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render(m.updateAvailable + "?")
			enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
			enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
			escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
			escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
			m.setStatus(bullet + " " + action + " " + verStr + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			return m, nil
		}
		m.updateCheckManual = true
		m.setStatus("Checking for updates…")
		return m, checkUpdateCmd(ver.Version)

	case "s":
		m.queue.ToggleShuffle()
		// Brief flash on the SHFL label so the keypress feels
		// acknowledged in the bar itself, not only in the status row.
		m.modeFlashTarget = "shuffle"
		m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
		if m.queue.IsShuffle() {
			m.setStatus("Shuffle: ON")
		} else {
			m.setStatus("Shuffle: OFF")
		}
		return m, saveQueueCmd(m.db, m.queue)

	case "r":
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
		return m, saveQueueCmd(m.db, m.queue)

	case "ctrl+up":
		if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor > 0 {
			m.queue.MoveUp(m.queueCursor)
			m.queueCursor--
		}
		return m, saveQueueCmd(m.db, m.queue)

	case "ctrl+down":
		if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor < m.queue.Len()-1 {
			m.queue.MoveDown(m.queueCursor)
			m.queueCursor++
		}
		return m, saveQueueCmd(m.db, m.queue)
	}

	return m, nil
}

// handleGlobalKey dispatches the key against the keymap's global
// bindings (page switch, refresh recs, open download dir). If a
// binding matches, its action runs and handled=true is returned.
// Called first by Update so a focused text input cannot swallow
// these keys.
func (m *Model) handleGlobalKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	for _, b := range Keys.Globals() {
		if !key.Matches(msg, b) {
			continue
		}
		// Matched a global — run the action. The case label and the
		// binding must agree; if a key is renamed in keys.go, update
		// both places.
		switch msg.String() {
		case "1": // Keys.PageStream
			if m.activePage != PageStream {
				m.switchPage(PageStream)
				m.setStatus("")
			}
			return true, nil
		case "2": // Keys.PageFavorites
			if m.activePage != PageFavorites {
				m.switchPage(PageFavorites)
				count := len(m.favorites)
				statusMsg := fmt.Sprintf("Favorites: %d tracks  (F to toggle)", count)
				if count == 0 {
					statusMsg = "No favorites yet — press F on any track"
				}
				m.setStatus(statusMsg)
			}
			return true, nil
		case "3": // Keys.PageLibrary
			if m.activePage != PageLibrary {
				m.switchPage(PageLibrary)
				statusMsg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
				if len(m.library) == 0 {
					statusMsg = "No downloaded tracks"
				}
				m.setStatus(statusMsg)
			}
			return true, nil
		case "4": // Keys.PageHistory
			if m.activePage != PageHistory {
				m.switchPage(PageHistory)
				m.setStatus("")
			}
			m.loadPlayHistory()
			return true, nil
		case "5": // Keys.PageSettings
			if m.activePage != PageSettings {
				m.switchPage(PageSettings)
				m.setStatus("")
			}
			return true, nil
		case "R": // Keys.Recs
			if m.activePage != PageStream {
				m.switchPage(PageStream)
			}
			m.recsSeq++
			m.showingRecommendations = true
			m.results = nil
			m.searchCursor = 0
			m.searchOffset = 0
			m.setStatus("Loading recommendations…")
			return true, fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.tidalClient, m.db)
		case "o": // Keys.Open
			path := m.downloadDir()
			if err := openInOS(path); err != nil {
				m.setStatus("Failed to open: " + err.Error())
			} else {
				m.setStatus("Opened: " + path)
			}
			return true, nil
		}
		// Matched a global we don't have an action for here.
		return true, nil
	}
	return false, nil
}

// toggleFavorite adds or removes a track from the favorites list.
// loadPlayHistory loads play history from the database synchronously.
// Must be called after switchPage(PageHistory) since switchPage sets
// historyLoaded = false. This is used by both keyboard "4" and mouse
// click on the History tab.
func (m *Model) loadPlayHistory() {
	entries, loadErr := m.db.LoadPlayHistory(100, 0)
	if loadErr != nil {
		m.err = loadErr
	} else {
		m.history = entries
	}
	m.historyLoaded = true
}

// Returns a saveFavoritesCmd so the caller can batch it.
func (m *Model) toggleFavorite(t queue.Track) tea.Cmd {
	if m.favoriteSet[t.ID] {
		// Remove
		delete(m.favoriteSet, t.ID)
		for i, ft := range m.favorites {
			if ft.ID == t.ID {
				m.favorites = append(m.favorites[:i], m.favorites[i+1:]...)
				break
			}
		}
		m.setStatus("Removed from favorites: " + t.Title)
	} else {
		// Add — prepend so most recent shows first
		m.favoriteSet[t.ID] = true
		m.favorites = append([]queue.Track{t}, m.favorites...)
		m.setStatus("Added to favorites: " + t.Title)
	}
	return saveFavoritesCmd(m.db, m.favorites)
}

