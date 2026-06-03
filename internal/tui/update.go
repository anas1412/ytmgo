package tui

import (
	"ytmgo/internal/player"
	ver "ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// Init satisfies tea.Model. It starts the tick for progress animation
// and fetches YouTube recommendations.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.settings.CookieBrowser, m.settings.UserAgent), scanLibraryCmd(m.downloadDir()), checkUpdateCmd(ver.Version))
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

	// ── Settings saved ────────────────────────────────────────────
	case SettingsSavedMsg:
		return m.handleSettingsSaved(msg)

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




