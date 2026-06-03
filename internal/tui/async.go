package tui

import (
	"fmt"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"

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

// ── Library scan complete ────────────────────────────────────────────

func (m Model) handleLibraryScan(msg LibraryScanMsg) (tea.Model, tea.Cmd) {
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
	// Keep listening
	if m.player != nil {
		return m, positionCmd(m.player)
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

	// Auto-advance: play the next track. PlayURL() returns the
	// local file when downloaded, otherwise the original YouTube
	// URL. Tracks with neither source are skipped.
	for {
		t, ok := m.queue.Next()
		if !ok {
			m.playerState = player.StateStopped
			m.player.Stop()
			m.position = 0
			m.duration = 0
			m.lastPosition = 0
			m.lastPositionAt = time.Time{}
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
}
