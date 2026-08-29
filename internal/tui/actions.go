package tui

import (
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"

	tea "github.com/charmbracelet/bubbletea"
)

// Shared user actions. Keyboard, mouse, and MPRIS all funnel through
// these so the three input paths can never drift apart (duplicated
// switch statements are what made +/- edit the wrong settings row).

// enqueueAndMaybePlay adds t to the end of the queue; when nothing is
// playing it starts playback of the new track. Always persists the queue.
func (m *Model) enqueueAndMaybePlay(t queue.Track) tea.Cmd {
	m.autoplayFired = false
	m.queue.Add(t)
	cmds := []tea.Cmd{saveQueueCmd(m.db, m.queue)}
	if m.playerState == player.StateStopped {
		m.queue.SetCurrentIndex(m.queue.Len() - 1)
		m.queueCursor = m.queue.CurrentIndex()
		m.clampQueueOffset()
		if playCmd := m.resolveAndPlayCmd(t); playCmd != nil {
			cmds = append(cmds, playCmd)
			return tea.Batch(cmds...)
		}
	}
	m.setStatus("Added to queue: " + t.Title)
	return tea.Batch(cmds...)
}

// activateSelection implements Enter / double-click for the focused
// panel of the current page: the queue panel plays the highlighted
// queue item; list panels add the highlighted track to the queue
// (auto-playing when idle) and, in Offline/Hybrid mode on the Stream
// page, also start a background download.
func (m *Model) activateSelection() tea.Cmd {
	// Queue panel: play the highlighted queue item on any page.
	if m.activePanel == PanelQueue {
		if playCmd := m.playSelectedQueueItem(); playCmd != nil {
			return tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
		}
		return nil
	}

	switch m.activePage {
	case PageFavorites:
		if len(m.favorites) > 0 && m.favCursor >= 0 && m.favCursor < len(m.favorites) {
			return m.enqueueAndMaybePlay(m.favorites[m.favCursor])
		}
	case PageLibrary:
		tracks := m.filteredLibrary()
		if len(tracks) > 0 && m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
			return m.enqueueAndMaybePlay(tracks[m.libraryCursor])
		}
	case PageHistory:
		if len(m.history) > 0 && m.historyCursor >= 0 && m.historyCursor < len(m.history) {
			return m.enqueueAndMaybePlay(historyEntryTrack(m.history[m.historyCursor]))
		}
	case PageStream:
		if len(m.results) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.results) {
			r := m.results[m.searchCursor]
			// resolveTrack consults the local library so a track that
			// already exists on disk plays the local file instead of
			// re-streaming.
			t := m.resolveTrack(r)
			cmds := []tea.Cmd{m.enqueueAndMaybePlay(t)}
			if m.settings.PlaybackMode == settings.PlaybackOffline ||
				(m.settings.PlaybackMode == settings.PlaybackHybrid && !t.Downloaded) {
				m.ensureDownloader()
				m.downloader.Enqueue(t.ID, t.Title, r.Uploader, t.URL, m.downloadDir(), r.CoverURL)
				cmds = append(cmds, downloadCmd(m.downloader))
			}
			return tea.Batch(cmds...)
		}
	}
	return nil
}

// nextTrack advances the queue and plays the next track.
func (m *Model) nextTrack() tea.Cmd {
	if m.queue.Len() == 0 {
		return nil
	}
	if _, ok := m.queue.Next(); !ok {
		return nil
	}
	m.queueCursor = m.queue.CurrentIndex()
	if playCmd := m.playSelectedQueueItem(); playCmd != nil {
		return tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
	}
	return saveQueueCmd(m.db, m.queue)
}

// prevTrack restarts the current track when more than 3s in, otherwise
// goes back to the previous track.
func (m *Model) prevTrack() tea.Cmd {
	if m.queue.Len() == 0 {
		return nil
	}
	if m.position > 3 {
		oldPos := m.position
		m.position = 0
		if m.player != nil {
			m.player.Seek(-oldPos)
		}
		m.setStatus("Restarting")
		return nil
	}
	if _, ok := m.queue.Prev(); !ok {
		return nil
	}
	m.queueCursor = m.queue.CurrentIndex()
	if playCmd := m.playSelectedQueueItem(); playCmd != nil {
		return tea.Batch(playCmd, saveQueueCmd(m.db, m.queue))
	}
	return saveQueueCmd(m.db, m.queue)
}

// togglePlayPause pauses or resumes playback.
func (m *Model) togglePlayPause() tea.Cmd {
	if m.player != nil {
		m.player.Pause()
		m.playerState = m.player.State()
	} else {
		// Dev mode (no player): toggle cached state.
		if m.playerState == player.StatePlaying {
			m.playerState = player.StatePaused
		} else {
			m.playerState = player.StatePlaying
		}
	}
	// Re-anchor the smooth-progress timer and restart the fast redraw
	// tick on resume.
	m.lastPositionAt = time.Now()
	m.updatePresence()
	if m.playerState == player.StatePlaying && !m.playerTicking {
		m.playerTicking = true
		return playerTickCmd()
	}
	return nil
}

// toggleShuffleAction toggles shuffle with the SHFL label flash.
func (m *Model) toggleShuffleAction() tea.Cmd {
	m.queue.ToggleShuffle()
	m.modeFlashTarget = "shuffle"
	m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
	if m.queue.IsShuffle() {
		m.setStatus("Shuffle: ON")
	} else {
		m.setStatus("Shuffle: OFF")
	}
	m.updateMPRIS()
	return saveQueueCmd(m.db, m.queue)
}

// cycleRepeatAction cycles repeat OFF → ONE → ALL → OFF.
func (m *Model) cycleRepeatAction() tea.Cmd {
	if !m.queue.IsRepeat() && !m.queue.IsRepeatAll() {
		m.queue.ToggleRepeat()
		m.setStatus("Repeat: ONE")
	} else if m.queue.IsRepeat() {
		m.queue.ToggleRepeat()
		m.queue.ToggleRepeatAll()
		m.setStatus("Repeat: ALL")
	} else {
		m.queue.ToggleRepeatAll()
		m.setStatus("Repeat: OFF")
	}
	m.modeFlashTarget = "repeat"
	m.modeFlashUntil = time.Now().Add(250 * time.Millisecond)
	m.updateMPRIS()
	return saveQueueCmd(m.db, m.queue)
}
