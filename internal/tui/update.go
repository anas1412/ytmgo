package tui

import (
	"fmt"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Init satisfies tea.Model. It starts the tick for progress animation
// and fetches YouTube recommendations.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchRecommendationsCmd(m.settings.CookieBrowser, m.settings.UserAgent), scanLibraryCmd(m.downloadDir()))
}

// Update satisfies tea.Model. It handles all messages without making
// any actual backend calls — purely UI state transitions.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Window resize ────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
		}

		// Search input fills available header space dynamically
		// Reserve ~16 chars for "♫ ytmgo" logo and padding
		m.searchInput.Width = max(20, msg.Width-18)
		return m, nil

	// ── Mouse events ─────────────────────────────────────────────
	case tea.MouseMsg:
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

	// ── Async search results ─────────────────────────────────────
	case SearchResultsMsg:
		m.isSearching = false
		if msg.Error != nil {
			m.err = msg.Error
			m.setStatus("Search failed: " + msg.Error.Error())
		} else {
			m.results = msg.Results
			m.searchCursor = 0
			m.searchOffset = 0
			if len(msg.Results) > 0 {
				m.setStatus(fmt.Sprintf("Found %d results", len(msg.Results)))
			} else {
				m.setStatus("No results found")
			}
		}
		return m, nil

	// ── Recommendations ─────────────────────────────────────────
	case RecommendationsMsg:
		m.showingRecommendations = msg.Error == nil
		if msg.Error != nil {
			m.err = msg.Error
			m.setStatus("Recommendations unavailable: " + msg.Error.Error())
		} else {
			m.results = msg.Results
			m.searchCursor = 0
			m.searchOffset = 0
			if len(msg.Results) > 0 {
				m.setStatus(fmt.Sprintf("%d recommendations", len(msg.Results)))
			} else {
				m.setStatus("No recommendations available")
			}
		}
		return m, nil

	// ── Library scan complete ────────────────────────────────────
	case LibraryScanMsg:
		m.library = msg.Tracks
		// Back-fill FilePath/Downloaded on any track already in the
		// queue that was added from search before the library finished
		// scanning. Without this, tracks queued in the first few
		// hundred milliseconds of app startup would still stream from
		// YouTube even though a local copy is now known to exist.
		m.backfillQueueFromLibrary()
		if len(msg.Tracks) > 0 {
			m.setStatus(fmt.Sprintf("Library: %d downloaded tracks", len(msg.Tracks)))
		}
		return m, nil

	// ── Settings saved ────────────────────────────────────────────
	case SettingsSavedMsg:
		if msg.Error != nil {
			m.err = msg.Error
			m.setStatus("Failed to save settings: " + msg.Error.Error())
		} else {
			m.setStatus("Settings saved")
		}
		return m, nil

	// ── Download progress ────────────────────────────────────────
	// Downloader state (active/done/failed/queued) lives in the DOWNLOADS
	// sub-panel; we just keep the event subscription alive here.
	case DownloadProgressMsg:
		if msg.Done {
			// Fix the status message when the file already existed on
			// disk (StatusSkipped) — the x-key handler optimistically
			// shows "Download queued" before the downloader checks.
			if msg.Status == downloader.StatusSkipped {
				m.setStatus("Already downloaded: " + msg.Title)
			}
			// Mark the track as downloaded and record file path
			m.queue.UpdateTrack(msg.TrackID, func(t *queue.Track) {
				t.Downloaded = true
				if msg.FilePath != "" {
					t.FilePath = msg.FilePath
				}
			})
			// Append to m.library so subsequent plays of the same
			// song from search/recommendations resolve to the local
			// file via resolveTrack's library lookup. Without this,
			// m.library would stay frozen at the startup scan and
			// freshly-downloaded tracks would always re-stream from
			// YouTube. Dedup by FilePath so re-runs / duplicate events
			// don't add the same entry twice.
			if msg.FilePath != "" && msg.Title != "" {
				alreadyInLibrary := false
				for _, lt := range m.library {
					if lt.FilePath == msg.FilePath {
						alreadyInLibrary = true
						break
					}
				}
				if !alreadyInLibrary {
					m.library = append(m.library, queue.Track{
						ID:         msg.TrackID,
						Title:      msg.Title,
						Artist:     msg.Uploader,
						FilePath:   msg.FilePath,
						Downloaded: true,
						// Duration/DurationSec left as zero — the next
						// library scan (or ffprobe on demand) will
						// populate them. The signature match in
						// findLibraryMatch only needs Title+Artist.
					})
				}
			}
			// Auto-play the downloaded track if nothing is currently playing
			if m.playerState == player.StateStopped {
				tracks := m.queue.Tracks()
				for i, t := range tracks {
					if t.ID == msg.TrackID && t.Downloaded && t.FilePath != "" {
						m.queue.SetCurrentIndex(i)
						m.queueCursor = i
						playCmd := m.startTrackPlayback(t.FilePath, t.Title, t.DurationSec)
						if playCmd == nil {
							// startTrackPlayback already set m.err / m.playerState.
							return m, downloadCmd(m.downloader)
						}
						return m, tea.Batch(downloadCmd(m.downloader), playCmd)
					}
				}
			}
		}
		// Always keep listening for the next progress event so the
		// DOWNLOADS sub-panel (active/pending/completed/failed sections)
		// stays in sync. Failed downloads (msg.Error != nil) are surfaced
		// in the Failed section of the sub-panel.
		return m, downloadCmd(m.downloader)

	// ── Player position update (from mpv IPC) ─────────────────────
	case PositionMsg:
		m.position = msg.Position
		if msg.Duration > 0 {
			m.duration = msg.Duration
		}
		// Record for smooth interpolation in View. The bar will glide
		// from this point forward based on time.Now() in renderPlayerBar.
		m.lastPosition = msg.Position
		m.lastPositionAt = time.Now()
		// Keep listening
		if m.player != nil {
			return m, positionCmd(m.player)
		}
		return m, nil

	// ── Song ended naturally (mpv exited / track finished) ───────
	case SongEndedMsg:
		// Suppress auto-advance if the old mpv was just killed by a
		// user-initiated playback (Enter on queue item, n/p keys).
		// Without this guard, the stale endedCmd from the previous
		// playback fires a SongEndedMsg milliseconds after Play()
		// kills the old process, and the Next() below advances past
		// the track the user just selected — the "press Enter on
		// first song → skips to the 2nd" bug.
		if m.suppressAutoAdvance {
			m.suppressAutoAdvance = false
			return m, nil
		}

		// Auto-advance: play the next track. PlayURL() returns the
		// local file when downloaded, otherwise the original YouTube
		// URL. Tracks with neither source are skipped.
		for {
			t, ok := m.queue.Next()
			if !ok {
				m.playerState = player.StateStopped
				m.setStatus("Queue empty")
				return m, nil
			}
			playURL := t.PlayURL()
			if playURL == "" {
				continue // skip — nothing to play
			}

			// Single source of truth: cursor follows the playing track.
			m.queueCursor = m.queue.CurrentIndex()
			m.clampQueueOffset()
			return m, m.startTrackPlayback(playURL, t.Title, t.DurationSec)
		}

	// ── Periodic tick (progress bar animation) ───────────────────
	case tickMsg:
		// Dev mode (no player): fake position advance
		if m.player == nil && m.playerState == player.StatePlaying && m.duration > 0 {
			m.position += 0.5
			// Keep the interpolation anchor fresh in dev mode so the
			// smooth bar reads the dev-simulated position accurately.
			m.lastPosition = m.position
			m.lastPositionAt = time.Now()
			if m.position >= m.duration {
				m.position = 0
				if t, ok := m.queue.Next(); ok {
					m.queueCursor = m.queue.CurrentIndex()
					m.duration = float64(t.DurationSec)
					m.setStatus("Now playing: " + t.Title)
				} else {
					m.playerState = player.StateStopped
				}
			}
		}
		// Auto-clear status messages after 3 seconds so the rotating idle
		// tips cycle back into view. Never auto-clear during confirmation
		// — the prompt must stay visible until Enter or Esc.
		if m.statusMessage != "" && m.err == nil && !m.isConfirming() && !m.statusMessageSetAt.IsZero() && time.Since(m.statusMessageSetAt) >= 3*time.Second {
			m.clearStatus()
		}
		// Rotate the idle status-bar tip every idleTipRotateEvery ticks.
		// Only advance the counter when no live status message or error is
		// being shown — keeps rotation cadence steady regardless of activity.
		if m.statusMessage == "" && m.err == nil {
			m.tickCount++
			if m.tickCount >= idleTipRotateEvery {
				m.advanceTip()
			}
		}
		// Real position comes from PositionMsg when player is active
		return m, tickCmd() // keep the tick going

	// ── Fast player tick (smooth progress interpolation) ────────
	// Fires every 50ms while a track is playing. The model itself
	// doesn't need to change — View reads time.Now() against
	// lastPositionAt and renders a gliding bar. We just keep the
	// ticker alive as long as we're in the playing state, and let
	// it die off naturally when paused/stopped.
	case playerTickMsg:
		if m.playerState == player.StatePlaying {
			return m, playerTickCmd()
		}
		return m, nil

	// ── Key presses ──────────────────────────────────────────────
	case tea.KeyMsg:
		// Global keys work in any focus mode. Checked first so a focused
		// text input (search box, settings field) cannot swallow them.
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
			case m.confirmAction == confirmDeleteTrack:
				// Keep the status-bar prompt visible until Enter or Esc
				return m, nil
			default:
				// For other confirmations, any key cancels
				m.clearConfirm()
				m.setStatus("Cancelled")
				return m, nil
			}
		}

		// When search is focused, route input to textinput (except tab/esc/enter)
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
					m.showingRecommendations = false
					m.results = nil
					m.isSearching = true
					m.searchCursor = 0
					m.err = nil
					return m, searchCmd(query, m.settings.SearchLimit, m.settings.CookieBrowser, m.settings.UserAgent)
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

		// When editing a string field on the Settings page, route to input
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
			case PageLibrary:
				// Library page: search input ↔ library list
				if m.searchFocused {
					// Search input → library list
					m.searchFocused = false
					m.searchInput.Blur()
					m.activePanel = PanelSearch
				} else if m.activePanel == PanelSearch {
					// Library list → download queue (using PanelQueue as right panel)
					m.activePanel = PanelQueue
				} else {
					// Download queue → search input
					m.activePanel = PanelSearch
					m.searchFocused = true
					m.searchInput.Focus()
				}
			default: // PageStream
				// 3-state cycle: search input → search results → queue → search input
				if m.searchFocused {
					// Search input → search results
					m.searchFocused = false
					m.searchInput.Blur()
					m.activePanel = PanelSearch
				} else if m.activePanel == PanelSearch {
					// Search results → Queue
					m.activePanel = PanelQueue
				} else {
					// Queue → Search input
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

		case "enter":
			// ── Settings page: toggle/edit setting ────────────────
			if m.activePage == PageSettings {
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
				case 0: // Stream Mode (boolean)
					m.settings.StreamMode = !m.settings.StreamMode
					return m, tea.Batch(saveSettingsCmd(m.settings))
				case 1: // Auto-Download (boolean)
					m.settings.AutoDownload = !m.settings.AutoDownload
					return m, tea.Batch(saveSettingsCmd(m.settings))
				case 2, 3: // Volume / Search Limit (numbers — Enter does nothing, use +/-)
					return m, nil
				case 4, 5, 6: // Download Dir / Cookie Browser / User-Agent (strings)
					m.startSettingsEdit()
					return m, nil
				}
				return m, nil
			}

			// ── Other pages (Stream / Library) ────────────────────
			switch m.activePage {
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

						if m.playerState == player.StateStopped && m.queue.Len() == 1 {
							// First track in an empty queue — auto-play
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
				// On Library page download queue panel: play selected completed download
				if m.activePanel == PanelQueue && !m.searchFocused {
					m.playSelectedQueueItem()
					if m.playerState == player.StatePlaying {
						return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
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
					// queueCursor intentionally NOT moved — the user's
					// selection stays where they left it.

					var cmds []tea.Cmd

					// Auto-play only if nothing was playing (smart start).
					// Only SetCurrentIndex when a track actually starts
					// playing — the queue's CurrentIndex is the single
					// source of truth for "what mpv is playing", so it
					// must never be set to a track that isn't playing.
					// queueCursor always follows currentIndex on playback.
					if m.playerState == player.StateStopped && m.queue.Len() == 1 {
						m.queue.SetCurrentIndex(m.queue.Len() - 1)
						m.queueCursor = m.queue.CurrentIndex()
						m.clampQueueOffset()
						if playCmd := m.startTrackPlayback(t.PlayURL(), t.Title, t.DurationSec); playCmd != nil {
							cmds = append(cmds, playCmd)
						}
					} else {
						m.setStatus("Added to queue: " + t.Title)
					}

					// Download handling (separate from auto-play).
					if !m.settings.StreamMode {
						m.ensureDownloader()
						m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir())
						cmds = append(cmds, downloadCmd(m.downloader))
					} else if m.settings.AutoDownload && !t.Downloaded {
						m.ensureDownloader()
						m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir())
						cmds = append(cmds, downloadCmd(m.downloader))
					}

					if len(cmds) == 0 {
						return m, nil
					}
					return m, tea.Batch(cmds...)
				}

				case PanelQueue:
					m.playSelectedQueueItem()
					if m.playerState == player.StatePlaying {
						return m, tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
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
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
			}
			return m, nil

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
			m.playSelectedQueueItem()
			if m.playerState == player.StatePlaying {
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
			}
			return m, nil

		case "l", "ctrl+f":
			m.position = min(m.position+5, m.duration)
			if m.player != nil {
				m.player.Seek(5)
			}
			return m, nil

		case "L":
			// L now behaves the same as "2" (always go to Library, consistent behavior)
			if m.activePage != PageLibrary {
				m.switchPage(PageLibrary)
				msg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
				if len(m.library) == 0 {
					msg = "No downloaded tracks"
				}
				m.setStatus(msg)
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
				case 2: // Default Volume
					if m.settings.DefaultVolume < 100 {
						m.settings.DefaultVolume = min(m.settings.DefaultVolume+5, 100)
						if m.player != nil {
							m.player.SetVolume(m.settings.DefaultVolume)
						}
						m.volume = m.settings.DefaultVolume
					}
				case 3: // Search Limit
					m.settings.SearchLimit = min(m.settings.SearchLimit+5, 100)
				}
				return m, tea.Batch(saveSettingsCmd(m.settings))
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
				case 2: // Default Volume
					if m.settings.DefaultVolume > 0 {
						m.settings.DefaultVolume = max(m.settings.DefaultVolume-5, 0)
						if m.player != nil {
							m.player.SetVolume(m.settings.DefaultVolume)
						}
						m.volume = m.settings.DefaultVolume
					}
				case 3: // Search Limit
					m.settings.SearchLimit = max(m.settings.SearchLimit-5, 5)
				}
				return m, tea.Batch(saveSettingsCmd(m.settings))
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
				// resolveTrack checks the library first so we can tell
				// the user "Already downloaded" instead of re-enqueueing
				// a download for a file that already exists locally.
				t := m.resolveTrack(r)
				if t.URL == "" {
					m.setStatus("No URL available for: " + t.Title)
					return m, nil
				}
				if t.Downloaded {
					m.setStatus("Already downloaded: " + t.Title)
					return m, nil
				}
				// Filesystem check — the library scan may not have caught
				// every file (manually placed, different session, etc.).
				m.ensureDownloader()
				if m.downloader.IsDownloaded(t.ID, t.Title, m.downloadDir()) {
					m.setStatus("Already downloaded: " + t.Title)
					return m, nil
				}
				m.downloader.Enqueue(t.ID, t.Title, r.Uploader, t.URL, m.downloadDir())
				m.setStatus("Download queued: " + t.Title)
				return m, downloadCmd(m.downloader)

			case m.activePage == PageStream && m.activePanel == PanelQueue && m.queue.Len() > 0:
				if m.queueCursor < 0 || m.queueCursor >= m.queue.Len() {
					return m, nil
				}
				t := m.queue.Tracks()[m.queueCursor]
				if t.Downloaded {
					m.setStatus("Already downloaded: " + t.Title)
					return m, nil
				}
				if t.URL == "" {
					m.setStatus("No URL available for: " + t.Title)
					return m, nil
				}
				m.ensureDownloader()
				if m.downloader.IsDownloaded(t.ID, t.Title, m.downloadDir()) {
					m.setStatus("Already downloaded: " + t.Title)
					return m, nil
				}
				m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir())
				m.setStatus("Download queued: " + t.Title)
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
				}
				return m, nil
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
			m.setStatus("Queue cleared")
			return m, nil

		case "s":
			m.queue.ToggleShuffle()
			// Brief flash on the SHFL label so the keypress feels
			// acknowledged in the bar itself, not only in the status row.
			m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
			if m.queue.IsShuffle() {
				m.setStatus("Shuffle: ON")
			} else {
				m.setStatus("Shuffle: OFF")
			}
			return m, nil

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
			m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
			return m, nil

		case "ctrl+up":
			if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor > 0 {
				m.queue.MoveUp(m.queueCursor)
				m.queueCursor--
			}
			return m, nil

		case "ctrl+down":
			if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor < m.queue.Len()-1 {
				m.queue.MoveDown(m.queueCursor)
				m.queueCursor++
			}
			return m, nil
		}
	}

	return m, nil
}

// playSelectedQueueItem plays the currently selected queue item,
// supporting both downloaded (local file) and streamed (URL) playback.
func (m *Model) playSelectedQueueItem() {
	if m.queue.Len() == 0 {
		return
	}
	// Clamp cursor
	if m.queueCursor < 0 {
		m.queueCursor = 0
	} else if m.queueCursor >= m.queue.Len() {
		m.queueCursor = m.queue.Len() - 1
	}

	t := m.queue.Tracks()[m.queueCursor]
	m.queue.SetCurrentIndex(m.queueCursor)

	// PlayURL() returns the local file path when downloaded, else
	// the streaming URL. A track with neither source is unplayable.
	playURL := t.PlayURL()
	if playURL == "" {
		m.playerState = player.StateStopped
		m.setStatus("Cannot play '" + t.Title + "': no file or URL")
		return
	}

	// startTrackPlayback mirrors the player's authoritative state back to
	// the model. We discard the returned cmd here because the caller of
	// playSelectedQueueItem is responsible for attaching position/ended
	// listeners (see the n/p/Enter handlers, which check
	// m.playerState == StatePlaying to decide whether to batch them in).
	//
	// suppressAutoAdvance prevents the stale endedCmd from the PREVIOUS
	// playback (which is still blocked on the old endCh) from calling
	// Next() in the SongEnded handler when the old mpv is killed by
	// the Play() call below. This is the key fix for the "press Enter
	// on first queue item → skips to the 2nd" bug: without it, a stale
	// SongEndedMsg arrives milliseconds after Play() and advances
	// currentIndex past the track the user just selected.
	if m.playerState == player.StatePlaying {
		m.suppressAutoAdvance = true
	}
	_ = m.startTrackPlayback(playURL, t.Title, t.DurationSec)
}

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

// handleClick maps a mouse click at (x, y) to the relevant UI action.
func (m Model) handleClick(x, y int) Model {
	// Settings page has no clickable panels
	if m.activePage == PageSettings {
		return m
	}

	// ── Header (y=0) → focus search input ──
	if y == 0 {
		m.searchFocused = true
		m.searchInput.Focus()
		m.activePanel = PanelSearch
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
		case "2": // Keys.PageLibrary
			if m.activePage != PageLibrary {
				m.switchPage(PageLibrary)
				statusMsg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
				if len(m.library) == 0 {
					statusMsg = "No downloaded tracks"
				}
				m.setStatus(statusMsg)
			}
			return true, nil
		case "3": // Keys.PageSettings
			if m.activePage != PageSettings {
				m.switchPage(PageSettings)
				m.setStatus("")
			}
			return true, nil
		case "R": // Keys.Recs
			if m.activePage != PageStream {
				m.switchPage(PageStream)
			}
			m.showingRecommendations = true
			m.results = nil
			m.searchCursor = 0
			m.searchOffset = 0
			m.setStatus("Loading recommendations…")
			return true, fetchRecommendationsCmd(m.settings.CookieBrowser, m.settings.UserAgent)
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
