package tui

import (
	"fmt"
	"ytmgo/internal/db"
	"ytmgo/internal/mpris"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	ver "ytmgo/internal/version"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

// historyEntryTrack converts a play-history row back into a playable
// track. New-style entries store a YouTube videoId, so the watch URL is
// reconstructed directly; legacy entries (TIDAL ids) fall back to
// yt-dlp resolution when played.
func historyEntryTrack(e db.PlayHistoryEntry) queue.Track {
	t := queue.Track{ID: e.TrackID, Title: e.Title, Artist: e.Artist, CoverURL: e.CoverURL}
	if ytmusic.IsVideoID(e.TrackID) {
		t.URL = ytmusic.WatchURL(e.TrackID)
	}
	return t
}

// Init satisfies tea.Model. It starts the tick for progress animation,
// opens the database, and fetches recommendations.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), initQueueFavoritesCmd(m.db), fetchQuoteCmd(m.quoteSeq), fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.db), scanLibraryCmd(m.downloadDir(), m.db), checkUpdateCmd(ver.Version), discordRPCInitCmd(m.settings.DiscordRPCEnabled), mprisInitCmd())
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

	// ── Album search / album opened ──────────────────────────────
	case AlbumResultsMsg:
		return m.handleAlbumResults(msg)

	case AlbumTracksMsg:
		return m.handleAlbumTracks(msg)

	case AlbumDownloadMsg:
		return m.handleAlbumDownload(msg)

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

	// ── Autoplay results (queue was empty, suggestions arrived) ──
	case AutoplayResultsMsg:
		return m.handleAutoplayResults(msg)

	// ── MPRIS service connected ──────────────────────────────────
	case MprisReadyMsg:
		m.mpris = msg.Svc
		m.updateMPRIS()
		return m, listenMprisCmd(m.mpris)

	// ── External control via MPRIS (media keys, playerctl) ──────
	case MprisCmdMsg:
		var cmd tea.Cmd
		switch msg.Cmd {
		case mpris.CmdPlayPause:
			cmd = m.togglePlayPause()
		case mpris.CmdPlay:
			if m.playerState == player.StatePaused {
				cmd = m.togglePlayPause()
			}
		case mpris.CmdPause:
			if m.playerState == player.StatePlaying {
				cmd = m.togglePlayPause()
			}
		case mpris.CmdStop:
			if m.player != nil {
				m.player.Stop()
			}
			m.playerState = player.StateStopped
			m.position = 0
			m.updatePresence()
		case mpris.CmdNext:
			cmd = m.nextTrack()
		case mpris.CmdPrev:
			cmd = m.prevTrack()
		}
		return m, tea.Batch(cmd, listenMprisCmd(m.mpris))

	// ── Visualizer frame ────────────────────────────────────────
	case VizFrameMsg:
		if !m.vizOn {
			return m, nil // toggled off while this frame was in flight
		}
		m.vizFrame = msg.Frame
		return m, vizFrameCmd(m.viz)

	case VizStoppedMsg:
		if !m.vizOn {
			return m, nil
		}
		m.vizOn = false
		m.viz.Close()
		m.viz = nil
		m.vizFrame = nil
		if msg.Err != nil {
			m.setStatus("Visualizer stopped: " + msg.Err.Error())
		} else {
			m.setStatus("Visualizer stopped")
		}
		return m, nil

	// ── URL prefetched (background cache populate) ──────────────
	case URLPrefetchedMsg:
		return m.handleURLPrefetched(msg)

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

	return m.resolveAndPlayCmd(t)
}
