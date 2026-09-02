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
// coverSendFrames is how many consecutive frames must carry a kitty
// transmit or delete. Bubble Tea discards frames rendered between
// flushes, so emitting once can be lost; three is comfortably more than
// the renderer ever drops.
const coverSendFrames = 3

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Count down the escapes owed to the terminal. Doing this here — not
	// in View — is what makes it reliable: Update runs exactly once per
	// message, while View may run and be thrown away.
	if m.coverSendN > 0 {
		m.coverSendN--
	}
	if m.coverClearN > 0 {
		m.coverClearN--
	}
	if m.albumArtSendN > 0 {
		m.albumArtSendN--
	}
	if m.albumArtClearN > 0 {
		m.albumArtClearN--
	}

	// Whether the now-playing panel is on screen is derived from two
	// things — the user's toggle and the current page — so rather than
	// have every path that changes either remember to start or stop the
	// spectrum, the visibility is compared before and after and
	// reconciled here, once.
	wasVisible := m.npVisible()
	wasCover := m.coverOnScreen()
	wasAlbumArt := m.albumArtOnScreen()
	updated, cmd := m.dispatch(msg)
	next, ok := updated.(Model)
	if !ok {
		return updated, cmd
	}
	if sync := next.syncNowPlaying(wasVisible); sync != nil {
		cmd = tea.Batch(cmd, sync)
	}
	// The cover in the player bar follows the same pattern: appearing
	// owes the terminal a transmit, disappearing owes it a delete.
	if now := next.coverOnScreen(); now != wasCover {
		if now {
			next.coverSendN = coverSendFrames
		} else {
			next.coverClearN = coverSendFrames
			next.coverSendN = 0
		}
	}
	// And the open album's art in the browse strip: closing the album
	// or leaving the page owes the terminal its delete.
	if now := next.albumArtOnScreen(); now != wasAlbumArt {
		if now {
			next.albumArtSendN = coverSendFrames
		} else {
			next.albumArtClearN = coverSendFrames
			next.albumArtSendN = 0
		}
	}
	return next, cmd
}

func (m Model) dispatch(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Window resize ────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if !m.ready {
			m.ready = true
		}

		// Keep the input's width in step with the box the header draws
		// around it. Both come from searchBoxWidth, which is the whole
		// point: this was once resized to the full header while the
		// wrapper stayed fixed, and an input rendering wider than its
		// box loses the text past the edge — a lipgloss width wraps
		// rather than truncates, and wraps on word boundaries, so a
		// query with no spaces moved wholesale onto a second line that
		// Height(1) discarded. The field showed its prompt and nothing
		// else.
		m.searchInput.Width = searchInputWidth(m.width)
		m.settingsEditInput.Width = searchInputWidth(m.width)
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
		if !m.npOn {
			return m, nil // toggled off while this frame was in flight
		}
		m.vizFrame = msg.Frame
		return m, vizFrameCmd(m.viz)

	case VizStoppedMsg:
		if m.viz == nil {
			return m, nil
		}
		m.viz.Close()
		m.viz = nil
		m.vizFrame = nil
		if msg.Err != nil {
			m.setStatus("Spectrum stopped: " + msg.Err.Error())
		}
		// The spectrum had been clocking the redraws; hand that back to
		// the progress ticker so the bar keeps gliding.
		return m, m.resumePlayerTick()

	case AlbumArtLoadedMsg:
		if msg.Seq != m.albumSeq || msg.Err != nil {
			return m, nil // superseded, or best-effort art that didn't load
		}
		m.albumArtImg = msg.Img
		m.albumArtURL = msg.URL
		// Transmit only — no delete. Re-transmitting under the same id
		// replaces any resident image, and the delete escape rides the
		// player bar, which renders *after* the strip in the frame: a
		// delete scheduled here lands right behind the placement and
		// removes the image the same frame draws, leaving the strip
		// blank until something re-sent it.
		m.albumArtSendN = coverSendFrames
		return m, nil

	case CoverLoadedMsg:
		// Loads race: a track change can have a second fetch in flight
		// before the first lands. Only the art the bar wants now may
		// settle, or a slow older load would overwrite a newer one.
		if msg.URL != m.desiredCoverURL() {
			return m, nil
		}
		m.coverLoading = false
		if msg.Err != nil {
			m.coverErr = msg.Err.Error()
			return m, nil
		}
		m.coverImg = msg.Img
		m.coverURL = msg.URL
		m.coverErr = ""
		// New artwork: replace whatever the terminal is holding.
		m.coverClearN = coverSendFrames
		m.coverSendN = coverSendFrames
		return m, nil

	// ── Lyrics loaded ────────────────────────────────────────────
	case LyricsLoadedMsg:
		return m.handleLyricsLoaded(msg)

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
