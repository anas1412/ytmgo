package tui

import (
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
	"ytmgo/internal/visualizer"

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
		// Album list: Enter opens the album rather than queueing it.
		if m.openAlbum == nil && m.albumMode {
			if len(m.albums) > 0 && m.searchCursor >= 0 && m.searchCursor < len(m.albums) {
				a := m.albums[m.searchCursor]
				m.isLoadingAlbum = true
				m.setStatus("Opening " + a.Title + "…")
				return openAlbumCmd(a)
			}
			return nil
		}
		// Inside an album: its tracks behave like ordinary results.
		list := m.results
		if m.openAlbum != nil {
			list = m.albumTracks
		}
		if len(list) > 0 && m.searchCursor >= 0 && m.searchCursor < len(list) {
			r := list[m.searchCursor]
			t := m.resolveTrack(r)
			cmds := []tea.Cmd{m.enqueueAndMaybePlay(t)}
			if m.settings.PlaybackMode == settings.PlaybackOffline ||
				(m.settings.PlaybackMode == settings.PlaybackHybrid && !t.Downloaded) {
				m.ensureDownloader()
				m.downloader.Enqueue(t.ID, t.Title, r.Uploader, t.URL, m.downloadDir(), r.CoverURL)
				m.revealDownloads()
				cmds = append(cmds, downloadCmd(m.downloader))
			}
			return tea.Batch(cmds...)
		}
		return nil

	}
	return nil
}

// npVisible reports whether the now-playing panel is actually on
// screen. npOn is the user's choice, which is on by default; the panel
// is additionally kept off the settings page, which draws its own
// layout with no room for it, and off terminals too short to give both
// it and the results list a usable number of rows. Since the spectrum
// is started and stopped from changes to this, it also means a window
// resized below the threshold shuts cava down rather than leaving it
// running for a panel that is not drawn — and before the first
// WindowSizeMsg there is no size, so nothing starts too early.
func (m Model) npVisible() bool {
	return m.npOn && m.activePage != PageSettings && m.npFits()
}

// syncNowPlaying starts or stops the spectrum so it runs exactly while
// the panel is on screen, and takes the artwork with it. Called once
// per message with the visibility from before the message was handled,
// so leaving the panel open and stepping onto the settings page shuts
// cava down, and stepping back off brings it and the cover back.
func (m *Model) syncNowPlaying(wasVisible bool) tea.Cmd {
	if m.npVisible() == wasVisible {
		return nil
	}
	if !m.npVisible() {
		if m.viz != nil {
			m.viz.Close()
			m.viz = nil
			m.vizFrame = nil
		}
		// Kitty images outlive the frame that drew them, so a panel
		// going off screen must take its artwork with it.
		m.coverClearN = coverSendFrames
		m.coverSendN = 0
		return nil
	}
	if m.coverImg != nil {
		m.coverSendN = coverSendFrames // re-send: it was deleted on the way out
	}
	if !visualizer.Available() {
		m.setStatus("Spectrum needs cava —  " + visualizer.InstallHint())
		return nil
	}
	v, err := visualizer.Start(m.vizBars())
	if err != nil {
		m.setStatus("Spectrum unavailable: " + err.Error())
		return nil
	}
	m.viz = v
	return vizFrameCmd(m.viz)
}

// revealDownloads opens the downloads panel. Called wherever a job is
// queued: the panel starts hidden, and a download the listener cannot
// see is worse than a panel they have to dismiss.
func (m *Model) revealDownloads() {
	m.downloadsHidden = false
}

// nextTrack advances the queue and plays the next track. It uses Skip,
// not Next, so an explicit press moves on even under repeat-one.
func (m *Model) nextTrack() tea.Cmd {
	if m.queue.Len() == 0 {
		return nil
	}
	if _, ok := m.queue.Skip(); !ok {
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

// resumePlayerTick restarts the progress-bar ticker when nothing else
// is driving redraws. Returns nil when it is already running or there
// is nothing to animate.
func (m *Model) resumePlayerTick() tea.Cmd {
	if m.playerState != player.StatePlaying || m.playerTicking || m.vizDrivesRedraw() {
		return nil
	}
	m.playerTicking = true
	return playerTickCmd()
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
