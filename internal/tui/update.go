package tui

import (
	"fmt"
"ytmgo/internal/player"

	"ytmgo/internal/queue"

	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies tea.Model. It starts the tick for progress animation
// and fetches personalized YouTube recommendations.
func (m Model) Init() tea.Cmd {
	m.results = nil // will be filled incrementally via streaming
	return tea.Batch(tickCmd(), startRecStreamCmd(), scanLibraryCmd())
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
				if m.showingLibrary {
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
				if m.showingLibrary {
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
			m.statusMessage = "Search failed: " + msg.Error.Error()
		} else {
			m.results = msg.Results
			m.searchCursor = 0
			m.searchOffset = 0
			if len(msg.Results) > 0 {
				m.statusMessage = fmt.Sprintf("Found %d results", len(msg.Results))
			} else {
				m.statusMessage = "No results found"
			}
		}
		return m, nil

	// ── Streaming recommendations ───────────────────────────────
	case RecStreamMsg:
		if msg.Cancel != nil {
			// First message from a stream — store channel & cancel.
			m.recStreamCh = msg.Ch
			m.recStreamCancel = msg.Cancel
		}
		if msg.Result != nil {
			m.results = append(m.results, *msg.Result)
			m.showingRecommendations = true
			// Read the next result.
			return m, readNextRecCmd(msg.Ch)
		}
		// Stream ended.
		if len(m.results) == 0 {
			m.statusMessage = "No recommendations available"
		} else {
			m.statusMessage = fmt.Sprintf("%d recommendations", len(m.results))
		}
		return m, nil

	// ── Recommendations (YouTube home feed) ──────────────────────
		case RecommendationsMsg:
			m.showingRecommendations = msg.Error == nil
			if msg.Error != nil {
				m.err = msg.Error
				m.statusMessage = "Recommendations unavailable: " + msg.Error.Error()
			} else {
				m.results = msg.Results
				m.searchCursor = 0
				m.searchOffset = 0
				if len(msg.Results) > 0 {
				m.statusMessage = fmt.Sprintf("%d recommendations", len(msg.Results))
			} else {
				m.statusMessage = "No recommendations available"
			}
		}
		return m, nil

	// ── Library scan complete ────────────────────────────────────
	case LibraryScanMsg:
		m.library = msg.Tracks
		if len(msg.Tracks) > 0 {
			m.statusMessage = fmt.Sprintf("Library: %d downloaded tracks", len(msg.Tracks))
		}
		return m, nil

	// ── Download progress ────────────────────────────────────────
	case DownloadProgressMsg:
		m.downloadPct = msg.Progress
		if msg.Error != nil {
			m.downloadErr = msg.Error
			m.downloadDone = false
			m.downloading = false
			return m, nil
		}
		if msg.Done {
			m.downloadPct = 100
			m.downloadDone = true
			m.downloading = false
			// Mark the track as downloaded and record file path
			m.queue.UpdateTrack(msg.TrackID, func(t *queue.Track) {
				t.Downloaded = true
				if msg.FilePath != "" {
					t.FilePath = msg.FilePath
				}
			})
			// Auto-play the downloaded track if nothing is currently playing
			if m.playerState == player.StateStopped {
				tracks := m.queue.Tracks()
				for i, t := range tracks {
					if t.ID == msg.TrackID && t.Downloaded && t.FilePath != "" {
						m.queue.SetCurrentIndex(i)
						m.queueCursor = i
						m.playerState = player.StatePlaying
						m.duration = float64(t.DurationSec)
						m.position = 0
						m.statusMessage = "Now playing: " + t.Title
						m.ensurePlayer()
						if err := m.player.Play(t.FilePath); err != nil {
							m.err = err
							m.playerState = player.StateStopped
							return m, downloadCmd(m.downloader)
						}
						return m, tea.Batch(
							downloadCmd(m.downloader),
							positionCmd(m.player),
							endedCmd(m.player),
						)
					}
				}
			}
			// Keep listening for next download
			return m, downloadCmd(m.downloader)
		}
		// In progress — keep listening
		return m, downloadCmd(m.downloader)

	// ── Player position update (from mpv IPC) ─────────────────────
	case PositionMsg:
		m.position = msg.Position
		if msg.Duration > 0 {
			m.duration = msg.Duration
		}
		// Keep listening
		if m.player != nil {
			return m, positionCmd(m.player)
		}
		return m, nil

	// ── Song ended naturally (mpv exited / track finished) ───────
	case SongEndedMsg:
		// Try auto-advancing through queued tracks until we find one that's downloaded
		for {
			t, ok := m.queue.Next()
			if !ok {
				m.playerState = player.StateStopped
				m.position = 0
				return m, nil
			}
			m.queueCursor = m.queue.CurrentIndex()
			if t.Downloaded && t.FilePath != "" {
				m.playerState = player.StatePlaying
				m.duration = float64(t.DurationSec)
				m.position = 0
				m.statusMessage = "Now playing: " + t.Title
				if err := m.player.Play(t.FilePath); err != nil {
					m.err = err
					m.playerState = player.StateStopped
					return m, nil
				}
				return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
			}
			// Skip undownloaded tracks, keep looking
		}

	// ── Periodic tick (progress bar animation) ───────────────────
	case tickMsg:
		// Dev mode (no player): fake position advance
		if m.player == nil && m.playerState == player.StatePlaying && m.duration > 0 {
			m.position += 0.5
			if m.position >= m.duration {
				m.position = 0
				if t, ok := m.queue.Next(); ok {
					m.queueCursor = m.queue.CurrentIndex()
					m.duration = float64(t.DurationSec)
					m.statusMessage = "Now playing: " + t.Title
				} else {
					m.playerState = player.StateStopped
				}
			}
		}
		// Real position comes from PositionMsg when player is active
		return m, tickCmd() // keep the tick going

	// ── Key presses ──────────────────────────────────────────────
	case tea.KeyMsg:
		// When help is open, only Esc and q close it
		if m.showHelp {
			switch msg.String() {
			case "?", "esc", "q":
				m.showHelp = false
			}
			return m, nil
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
				if m.showingLibrary {
					// In library view, Enter just exits the search field (filtering already happened live)
					return m, nil
				}
				if query != "" {
					m.showingRecommendations = false
					m.showingLibrary = false
					// Cancel any running recommendation stream.
					if m.recStreamCancel != nil {
						m.recStreamCancel()
						m.recStreamCancel = nil
						m.recStreamCh = nil
					}
					m.results = nil
					m.isSearching = true
					m.searchCursor = 0
					m.err = nil
					return m, searchCmd(query, 10)
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
			if m.showingLibrary {
				m.clampLibraryOffset()
			}
			return m, cmd
		}

		// ── Global keybindings ───────────────────────────────
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.Shutdown()
			return m, tea.Quit

		case "?":
			m.showHelp = true
			return m, nil

		case "tab":
			// 3-state cycle: search input → search results → queue → search input
			if m.activePanel == PanelSearch {
				// Search results → Queue
				m.activePanel = PanelQueue
			} else {
				// Queue → Search input
				m.activePanel = PanelSearch
				m.searchFocused = true
				m.searchInput.Focus()
			}
			return m, nil

		case "esc":
			m.showHelp = false
			return m, nil

		// ── Panel navigation ─────────────────────────────────
		case "up", "k":
			switch m.activePanel {
			case PanelSearch:
				if m.showingLibrary {
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
			switch m.activePanel {
			case PanelSearch:
				if m.showingLibrary {
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
			switch m.activePanel {
			case PanelSearch:
				if m.showingLibrary {
					// Library mode: play a downloaded track directly (use filtered list)
					tracks := m.filteredLibrary()
					if len(tracks) > 0 && m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
						t := tracks[m.libraryCursor]
						m.queue.Add(t)
						m.queue.SetCurrentIndex(m.queue.Len() - 1)
						m.queueCursor = m.queue.Len() - 1
						m.playerState = player.StatePlaying
						m.duration = float64(t.DurationSec)
						m.position = 0
						m.statusMessage = "Now playing: " + t.Title
						m.ensurePlayer()
						if err := m.player.Play(t.FilePath); err != nil {
							m.err = err
							m.playerState = player.StateStopped
						} else {
							return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
						}
					}
					return m, nil
				}
				if len(m.results) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.results) {
					r := m.results[m.searchCursor]
					t := searchResultToTrack(r)
					m.queue.Add(t)
					m.statusMessage = "Added to queue: " + t.Title
					// Enqueue download for the added track
					m.ensureDownloader()
					m.downloader.Enqueue(t.ID, t.Title, r.URL, downloadDir())
					m.downloading = true
					m.downloadTitle = t.Title
					m.downloadPct = 0
					m.downloadDone = false
					m.downloadErr = nil
					return m, downloadCmd(m.downloader)
				}
			case PanelQueue:
				if m.queue.Len() > 0 {
					// Clamp cursor in case it was corrupted (e.g. by a click on an empty queue)
					if m.queueCursor < 0 {
						m.queueCursor = 0
					} else if m.queueCursor >= m.queue.Len() {
						m.queueCursor = m.queue.Len() - 1
					}
					t := m.queue.Tracks()[m.queueCursor]
					m.queue.SetCurrentIndex(m.queueCursor)

					if t.Downloaded && t.FilePath != "" {
						// Track is ready — play it
						m.playerState = player.StatePlaying
						m.duration = float64(t.DurationSec)
						m.position = 0
						m.statusMessage = "Now playing: " + t.Title
						m.ensurePlayer()
						if err := m.player.Play(t.FilePath); err != nil {
							m.err = err
							m.playerState = player.StateStopped
							return m, nil
						}
						return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
					}

					// Track not yet downloaded
					m.playerState = player.StateStopped
					if m.downloading {
						m.statusMessage = "Still downloading: " + t.Title
					} else {
						m.statusMessage = "Not downloaded yet: " + t.Title
					}
				}
			}
			return m, nil

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
			return m, nil

		case "n", "right":
			if m.queue.Len() == 0 {
				return m, nil
			}
			if _, ok := m.queue.Next(); !ok {
				return m, nil
			}
			next := m.queue.CurrentIndex()
			m.queueCursor = next
			t := m.queue.Tracks()[next]
			// Play only if file is downloaded
			if t.Downloaded && t.FilePath != "" {
				m.playerState = player.StatePlaying
				m.duration = float64(t.DurationSec)
				m.position = 0
				m.statusMessage = "Now playing: " + t.Title
				m.ensurePlayer()
				if err := m.player.Play(t.FilePath); err != nil {
					m.err = err
					m.playerState = player.StateStopped
				} else {
					return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
				}
			}
			m.playerState = player.StateStopped
			m.statusMessage = "Not downloaded: " + t.Title
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
				m.statusMessage = "Restarting"
				return m, nil
			}
			// Less than 3s in — go to the previous track
			if _, ok := m.queue.Prev(); !ok {
				return m, nil
			}
			prev := m.queue.CurrentIndex()
			m.queueCursor = prev
			t := m.queue.Tracks()[prev]
			if t.Downloaded && t.FilePath != "" {
				m.playerState = player.StatePlaying
				m.duration = float64(t.DurationSec)
				m.position = 0
				m.statusMessage = "Now playing: " + t.Title
				m.ensurePlayer()
				if err := m.player.Play(t.FilePath); err != nil {
					m.err = err
					m.playerState = player.StateStopped
				} else {
					return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
				}
			}
			m.playerState = player.StateStopped
			m.statusMessage = "Not downloaded: " + t.Title
			return m, nil

		case "l", "ctrl+f":
			m.position = min(m.position+5, m.duration)
			if m.player != nil {
				m.player.Seek(5)
			}
			return m, nil

		case "L":
			// Toggle library view
			m.showingLibrary = !m.showingLibrary
			if m.showingLibrary {
				m.showingRecommendations = false
				m.libraryCursor = 0
				m.libraryOffset = 0
				n := len(m.library)
				if n > 0 {
					m.statusMessage = fmt.Sprintf("Library: %d tracks  (type to filter)", n)
				} else {
					m.statusMessage = "No downloaded tracks"
				}
			} else {
				m.statusMessage = ""
			}
			return m, nil

		case "R":
			// Cancel any existing recommendation stream and start fresh.
			if m.recStreamCancel != nil {
				m.recStreamCancel()
				m.recStreamCancel = nil
				m.recStreamCh = nil
			}
			m.showingRecommendations = true
			m.showingLibrary = false
			m.results = nil
			m.searchCursor = 0
			m.searchOffset = 0
			m.statusMessage = "Loading recommendations…"
			return m, startRecStreamCmd()

		case "h", "ctrl+b":
			m.position = max(m.position-5, 0)
			if m.player != nil {
				m.player.Seek(-5)
			}
			return m, nil

		case "+", "=":
			m.volume = min(m.volume+5, 100)
			if m.player != nil {
				m.player.SetVolume(m.volume)
			}
			return m, nil

		case "-", "_":
			m.volume = max(m.volume-5, 0)
			if m.player != nil {
				m.player.SetVolume(m.volume)
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
			}
			return m, nil

		case "D":
			m.queue.Clear()
			m.queueCursor = 0
			m.playerState = player.StateStopped
			m.position = 0
			m.duration = 0
			if m.player != nil {
				m.player.Stop()
			}
			m.statusMessage = "Queue cleared"
			return m, nil

		case "s":
			m.queue.ToggleShuffle()
			if m.queue.IsShuffle() {
				m.statusMessage = "Shuffle: ON"
			} else {
				m.statusMessage = "Shuffle: OFF"
			}
			return m, nil

		case "r":
			if !m.queue.IsRepeat() && !m.queue.IsRepeatAll() {
				m.queue.ToggleRepeat() // → repeat: true
				m.statusMessage = "Repeat: ONE"
			} else if m.queue.IsRepeat() {
				m.queue.ToggleRepeat()   // repeat: false
				m.queue.ToggleRepeatAll() // repeatAll: true
				m.statusMessage = "Repeat: ALL"
			} else {
				m.queue.ToggleRepeatAll() // repeatAll: false
				m.statusMessage = "Repeat: OFF"
			}
			return m, nil

		case "ctrl+up":
			if m.activePanel == PanelQueue && m.queueCursor > 0 {
				m.queue.MoveUp(m.queueCursor)
				m.queueCursor--
			}
			return m, nil

		case "ctrl+down":
			if m.activePanel == PanelQueue && m.queueCursor < m.queue.Len()-1 {
				m.queue.MoveDown(m.queueCursor)
				m.queueCursor++
			}
			return m, nil
		}
	}

	return m, nil
}

// ─── Mouse click handling ──────────────────────────────────────────
//
// Layout reference (must stay in sync with view.go / renderPanels):
//
//   y=0            header
//   y=1..N         panels (height = screenH - header - download - player - help - statuses)
//   y=N+1..N+4     download bar (border + content + border + gap)
//   y=N+5..N+9     player bar (4 lines: now-playing + progress + controls + pad)
//   y=N+10         status line (optional)
//   y=N+11         help bar
//
// All section heights are approximate because borders add variable padding.
// Click positions are best-effort, not pixel-perfect.

const (
	clickHeaderLines    = 1
	clickPanelStartY    = 1
	clickDownloadHeight = 4
	clickPlayerHeight   = 4
)

// handleClick maps a mouse click at (x, y) to the relevant UI action.
func (m Model) handleClick(x, y int) Model {
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
			if m.showingLibrary {
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
			// Right panel: queue
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
		return m
	}

	// ── Player controls (bottom region) — rough play/pause toggle ──
	playerStartY := panelsEnd + clickDownloadHeight
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


