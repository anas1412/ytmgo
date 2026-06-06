package tui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	ver "ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Async search results ─────────────────────────────────────────────

func (m Model) handleSearchResults(msg SearchResultsMsg) (tea.Model, tea.Cmd) {
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
}

// ── Recommendations ──────────────────────────────────────────────────

func (m Model) handleRecommendations(msg RecommendationsMsg) (tea.Model, tea.Cmd) {
	// Stale response — a newer request or search invalidated this one.
	if msg.Seq != m.recsSeq {
		return m, nil
	}
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
}

// ── Update check complete ─────────────────────────────────────────────

func (m Model) handleUpdateCheck(msg UpdateCheckMsg) (tea.Model, tea.Cmd) {
	if msg.LatestVersion == "" {
		if m.updateCheckManual {
			m.updateCheckManual = false
			m.setStatus("Update check failed")
		}
		return m, nil // check failed/skipped, stay unknown
	}
	if msg.LatestVersion != ver.Version {
		// ignore "dev" vs. something (local build) — don't notify
		if ver.Version == "dev" {
			return m, nil
		}
		m.updateAvailable = msg.LatestVersion
	} else {
		m.updateAvailable = "latest"
	}

	if m.updateCheckManual {
		m.updateCheckManual = false
		if m.updateAvailable == "latest" {
			m.setStatus("Already up to date (" + ver.Version + ")")
		} else {
			m.setStatus("Update " + m.updateAvailable + " available — press U")
		}
	}
	return m, nil
}

// ── Update install complete ──────────────────────────────────────────

func (m Model) handleUpdateResult(msg UpdateResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.setStatus("✗ " + msg.Error.Error())
		return m, nil
	}
	m.setStatus("Update complete, restarting…")
	// Launch the updated binary, then quit
	exe, err := os.Executable()
	if err != nil {
		m.setStatus("✗ Cannot restart: " + err.Error())
		return m, nil
	}
	exec.Command(exe).Start()
	return m, tea.Quit
}

// ── Random quote received ──────────────────────────────────────────

func (m Model) handleQuote(msg QuoteMsg) (tea.Model, tea.Cmd) {
	if msg.Seq != m.quoteSeq {
		return m, nil // stale response
	}
	m.currentQuote = fmt.Sprintf(`"%s" — %s`, msg.Quote, msg.Author)
	return m, nil
}

// ── Library scan complete ────────────────────────────────────────────

func (m Model) handleLibraryScan(msg LibraryScanMsg) (tea.Model, tea.Cmd) {
	m.library = msg.Tracks
	m.libraryLoaded = true
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
}

// ── Settings saved ───────────────────────────────────────────────────

func (m Model) handleSettingsSaved(msg SettingsSavedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Failed to save settings: " + msg.Error.Error())
	} else {
		m.setStatus("Settings saved")
	}
	return m, nil
}

// ── Download progress ────────────────────────────────────────────────

func (m Model) handleDownloadProgress(msg DownloadProgressMsg) (tea.Model, tea.Cmd) {
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
					playCmd := m.resolveAndPlayCmd(t)
					if playCmd == nil {
						// resolveAndPlayCmd already set m.err / m.playerState.
						return m, tea.Batch(downloadCmd(m.downloader), saveQueueCmd(m.db, m.queue))
					}
					return m, tea.Batch(downloadCmd(m.downloader), playCmd, saveQueueCmd(m.db, m.queue))
				}
			}
		}
	}
	// Always keep listening for the next progress event so the
	// DOWNLOADS sub-panel (active/pending/completed/failed sections)
	// stays in sync. Failed downloads (msg.Error != nil) are surfaced
	// in the Failed section of the sub-panel.
	return m, downloadCmd(m.downloader)
}

// ── Async YouTube URL resolution ─────────────────────────────────────

func (m Model) handleURLResolved(msg URLResolvedMsg) (tea.Model, tea.Cmd) {
	// Check if this resolve result is still relevant — a newer resolve
	// may have been triggered since this one was dispatched.
	if m.pendingResolve == nil {
		return m, nil
	}
	m.pendingResolve = nil

	if msg.Error != nil {
		switch msg.Action {
		case "play":
			m.err = fmt.Errorf("failed to resolve stream URL: %w", msg.Error)
			m.playerState = player.StateStopped
			m.setStatus("Cannot play '" + msg.Title + "': " + msg.Error.Error())
		case "download":
			m.setStatus("No URL available for: " + msg.Title)
		}
		return m, nil
	}

	// Cache the resolved URL so future plays skip the yt-dlp call.
	if msg.URL != "" && msg.TrackID != "" {
		m.resolvedURLs[msg.TrackID] = msg.URL
		if m.db != nil {
			_ = m.db.SaveCachedURL(msg.TrackID, msg.URL) // non-fatal
		}
	}

	switch msg.Action {
	case "download":
		// Proceed with the enqueue now that we have the URL.
		m.ensureDownloader()
		m.downloader.Enqueue(msg.TrackID, msg.Title, msg.Uploader, msg.URL, m.downloadDir(), msg.CoverURL)
		m.setStatus("Download queued: " + msg.Title)
		return m, downloadCmd(m.downloader)

	case "play":
		t := msg.Track
		if playCmd := m.startTrackPlayback(msg.URL, t); playCmd != nil {
			return m, playCmd
		}
		return m, nil
	}

	return m, nil
}

// ── URL prefetched (background cache populated) ─────────────────────

func (m Model) handleURLPrefetched(msg URLPrefetchedMsg) (tea.Model, tea.Cmd) {
	if msg.URL == "" || msg.TrackID == "" {
		return m, nil // sanity check
	}
	m.resolvedURLs[msg.TrackID] = msg.URL
	if m.db != nil {
		_ = m.db.SaveCachedURL(msg.TrackID, msg.URL) // non-fatal
	}
	return m, nil
}

// ── Player position update (from mpv IPC) ────────────────────────────

func (m Model) handlePosition(msg PositionMsg) (tea.Model, tea.Cmd) {
	m.position = msg.Position
	if msg.Duration > 0 {
		m.duration = msg.Duration
	}
	// Record for smooth interpolation in View. The bar will glide
	// from this point forward based on time.Now() in renderPlayerBar.
	m.lastPosition = msg.Position
	m.lastPositionAt = time.Now()

	// Pre-fetch autoplay recommendations 30 s before the current track
	// ends so the suggestions are already in the queue by the time the
	// song finishes — seamless playback.
	var autoplayCmd tea.Cmd
	if m.settings.AutoplayEnabled &&
		!m.autoplayFired &&
		m.playerState == player.StatePlaying &&
		m.duration > 0 &&
		m.duration-m.position <= 30 &&
		m.queue.IsLastTrack() {
		m.autoplayFired = true
		m.setStatus("Autoplay fetching suggestions…")
		autoplayCmd = fetchAutoplayCmd(m.tidalClient, m.db)
	}

	// Keep listening
	if m.player != nil {
		cmds := []tea.Cmd{positionCmd(m.player)}
		if autoplayCmd != nil {
			cmds = append(cmds, autoplayCmd)
		}
		return m, tea.Batch(cmds...)
	}
	return m, nil
}

// ── Song ended naturally (mpv exited / track finished) ───────────────

func (m Model) handleSongEnded(msg SongEndedMsg) (tea.Model, tea.Cmd) {
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

	// Auto-advance: play the next track. Uses resolveAndPlayCmd
	// so already-downloaded tracks play immediately while streaming
	// tracks first show "Fetching URL…" while the
	// YouTube URL is resolved asynchronously.
	for {
		t, ok := m.queue.Next()
		if !ok {
			// Queue empty — try autoplay if enabled and not already triggered
			// (prevents infinite re-trigger loops when autoplay-added
			// tracks finish and the queue runs dry again).
			if m.settings.AutoplayEnabled && !m.autoplayFired {
				m.autoplayFired = true
				m.playerState = player.StateStopped
				m.player.Stop()
				m.position = 0
				m.duration = 0
				m.lastPosition = 0
				m.lastPositionAt = time.Time{}
				m.updateDiscordRPC()
				m.setStatus("Autoplay fetching suggestions…")
				return m, fetchAutoplayCmd(m.tidalClient, m.db)
			}

			// Pre-fetch already in progress (handlePosition fired
			// fetchAutoplayCmd 30s before the track ended but results
			// haven't arrived yet).  Wait for handleAutoplayResults.
			if m.settings.AutoplayEnabled && m.autoplayFired {
				m.playerState = player.StateStopped
				m.player.Stop()
				m.position = 0
				m.duration = 0
				m.lastPosition = 0
				m.lastPositionAt = time.Time{}
				m.updateDiscordRPC()
				m.setStatus("Autoplay loading suggestions…")
				return m, nil
			}

			m.playerState = player.StateStopped
			m.player.Stop()
			m.position = 0
			m.duration = 0
			m.lastPosition = 0
			m.lastPositionAt = time.Time{}
			m.updateDiscordRPC()
			m.setStatus("Queue empty")
			return m, nil
		}

		// Single source of truth: cursor follows the playing track.
		m.queueCursor = m.queue.CurrentIndex()
		m.clampQueueOffset()

		playCmd := m.resolveAndPlayCmd(t)
		if playCmd == nil {
			// resolveAndPlayCmd returns nil only when the track
			// can't be played locally — which we take as "skip".
			continue
		}
		return m, tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
	}
}

// ── Autoplay results received ────────────────────────────────────────────

func (m Model) handleAutoplayResults(msg AutoplayResultsMsg) (tea.Model, tea.Cmd) {
	// Reset the latch so the *next* batch can fire when the queue runs dry
	// again — infinite autoplay.  (This also unblocks if the user added a
	// manual track while the fetch was in-flight; the stale results simply
	// get enqueued and the next empty-queue detection works as expected.)
	m.autoplayFired = false

	if len(msg.Tracks) == 0 {
		if m.playerState != player.StatePlaying {
			m.updateDiscordRPC()
		}
		m.setStatus("Autoplay: no suggestions available")
		return m, nil
	}

	// Add all autoplay tracks to the end of the queue
	for _, t := range msg.Tracks {
		m.queue.Add(t)
	}

	// If player is already running (user queued something while autoplay
	// was loading), don't interrupt — just leave tracks in the queue.
	if m.playerState == player.StatePlaying {
		m.setStatus(fmt.Sprintf("Autoplay: %d suggestions added to queue", len(msg.Tracks)))
		return m, tea.Batch(saveQueueCmd(m.db, m.queue))
	}

	// Play the first autoplay track.  Set the queue's currentIndex so
	// that PeekNext() (for URL prefetch) and handleSongEnded's Next()
	// (for auto-advance) correctly point to autoplay tracks rather than
	// stale positions from the now-exhausted previous queue.
	firstIdx := m.queue.Len() - len(msg.Tracks)
	if firstIdx >= 0 {
		m.queue.SetCurrentIndex(firstIdx)
		m.queueCursor = firstIdx
	}
	cmds := []tea.Cmd{saveQueueCmd(m.db, m.queue)}
	playCmd := m.resolveAndPlayCmd(msg.Tracks[0])
	if playCmd != nil {
		cmds = append(cmds, playCmd)
	}

	m.setStatus("Autoplay: playing suggestions")
	return m, tea.Batch(cmds...)
}
