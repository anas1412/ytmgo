package tui

import (
	"fmt"
	"ytmgo/internal/player"
	ver "ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies tea.Model. It starts the tick for progress animation,
// opens the database, and fetches TIDAL recommendations.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), initQueueFavoritesCmd(m.db), fetchQuoteCmd(m.quoteSeq), fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.tidalClient, m.db), scanLibraryCmd(m.downloadDir()), checkUpdateCmd(ver.Version), discordRPCInitCmd(m.settings.DiscordRPCEnabled))
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
		return m.handleMouse(msg)

	// ── Async search results ─────────────────────────────────────
	case SearchResultsMsg:
		return m.handleSearchResults(msg)

	// ── Recommendations ─────────────────────────────────────────
	case RecommendationsMsg:
		return m.handleRecommendations(msg)

	// ── Library scan complete ────────────────────────────────────
	case LibraryScanMsg:
		return m.handleLibraryScan(msg)

	// ── Update check complete ──────────────────────────────────
	case UpdateCheckMsg:
		return m.handleUpdateCheck(msg)

	// ── Update install complete ────────────────────────────────
	case UpdateResultMsg:
		return m.handleUpdateResult(msg)

	// ── Random quote received ─────────────────────────────────
	case QuoteMsg:
		return m.handleQuote(msg)

	// ── Settings saved ────────────────────────────────────────────
	case SettingsSavedMsg:
		return m.handleSettingsSaved(msg)

	// ── Database ready (SQLite opened, queue + favorites loaded) ──
	case DbReadyMsg:
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.queue.LoadData(msg.QueueTracks, msg.Shuffle, msg.Repeat, msg.RepeatAll)
		m.favorites = msg.Favorites
		m.favoriteSet = make(map[string]bool, len(msg.Favorites))
		for _, ft := range msg.Favorites {
			m.favoriteSet[ft.ID] = true
		}
		m.setStatus(fmt.Sprintf("Loaded %d tracks and %d favorites",
			len(msg.QueueTracks), len(msg.Favorites)))
		return m, nil

	// ── Play history recorded ───────────────────────────────────
	case PlayRecordedMsg:
		if msg.Error != nil {
			m.err = msg.Error
		}
		return m, nil

	// ── Async YouTube URL resolution ─────────────────────────────
	case URLResolvedMsg:
		return m.handleURLResolved(msg)

	// ── Download progress ────────────────────────────────────────
	case DownloadProgressMsg:
		return m.handleDownloadProgress(msg)

	// ── Player position update (from mpv IPC) ─────────────────────
	case PositionMsg:
		return m.handlePosition(msg)

	// ── Song ended naturally (mpv exited / track finished) ───────
	case SongEndedMsg:
		return m.handleSongEnded(msg)

	// ── Periodic tick (progress bar animation) ───────────────────
	case tickMsg:
		return m.handleTick(msg)

	// ── Fast player tick (smooth progress interpolation) ────────
	case playerTickMsg:
		return m.handlePlayerTick(msg)

	// ── Key presses ──────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

// playSelectedQueueItem plays the currently selected queue item,
// supporting both downloaded (local file) and streamed (URL) playback.
// Returns the tea.Cmd (which may be a resolveURLCmd for async URL
// resolution) or nil if nothing was played.
func (m *Model) playSelectedQueueItem() tea.Cmd {
	if m.queue.Len() == 0 {
		return nil
	}
	// Clamp cursor
	if m.queueCursor < 0 {
		m.queueCursor = 0
	} else if m.queueCursor >= m.queue.Len() {
		m.queueCursor = m.queue.Len() - 1
	}

	t := m.queue.Tracks()[m.queueCursor]
	m.queue.SetCurrentIndex(m.queueCursor)

	// suppressAutoAdvance prevents the stale endedCmd from the PREVIOUS
	// playback (which is still blocked on the old endCh) from calling
	// Next() in the SongEnded handler when the old mpv is killed by
	// the Play() call below.
	if m.playerState == player.StatePlaying {
		m.suppressAutoAdvance = true
	}

	return m.resolveAndPlayCmd(t)
}




