package tui

import (
	"errors"
	"time"

	"ytmgo/internal/clipboard"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
	"ytmgo/internal/visualizer"
	"ytmgo/internal/ytmusic"

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
			if albums := m.streamAlbums(); len(albums) > 0 && m.searchCursor >= 0 && m.searchCursor < len(albums) {
				a := albums[m.searchCursor]
				m.isLoadingAlbum = true
				m.albumSeq++
				m.albumCoverURL = a.CoverURL
				m.setStatus("Opening " + a.Title + "…")
				return openAlbumCmd(a, m.albumSeq)
			}
			return nil
		}
		// Inside an album: its tracks behave like ordinary results.
		list := m.results
		if m.streamShowsTracks() {
			list = m.streamTracks()
		}
		if len(list) > 0 && m.searchCursor >= 0 && m.searchCursor < len(list) {
			r := list[m.searchCursor]
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

// coverOnScreen reports whether the player bar is showing album art.
// The kitty image outlives the frame that drew it, so transitions of
// this — reconciled once per message in Update — schedule the delete
// and the re-send.
func (m Model) coverOnScreen() bool {
	return m.playerCoverSlot() && m.coverImg != nil
}

// albumArtOnScreen reports whether the browse strip is showing the open
// album's cover — the stream page, an open album, and art in hand.
//
// A fetch in flight counts as not showing it. The wait replaces the
// whole strip, art included, but the kitty image is an overlay the
// terminal keeps until something deletes it: with the album still set
// this answered yes, no delete was ever owed, and the old cover hung
// over the loading message until the new page arrived.
func (m Model) albumArtOnScreen() bool {
	if m.isLoadingAlbum || m.isLoadingArtist {
		return false
	}
	return m.activePage == PageStream && (m.openAlbum != nil || m.openArtist != nil) && m.albumArtImg != nil
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
		return nil
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

// leaveAlbumView drops any open album preview — tracklist and cover.
// Returns a cover-refresh command that retries the playing track's art
// if an earlier load failed (nil when nothing was open or needed).
func (m *Model) leaveAlbumView() tea.Cmd {
	if m.openAlbum == nil && !m.isLoadingAlbum && m.albumCoverURL == "" {
		return nil
	}
	m.openAlbum = nil
	m.albumTracks = nil
	if m.isLoadingAlbum {
		// Same as leaveArtistView: cancel the fetch rather than let it
		// open an album the user has already stepped out of.
		m.albumSeq++
		m.isLoadingAlbum = false
	}
	m.albumCoverURL = ""
	return m.refreshCoverCmd()
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

// toggleAutoplayAction flips the same setting the Settings page owns,
// so the two stay one value rather than two.
func (m *Model) toggleAutoplayAction() tea.Cmd {
	m.settings.AutoplayEnabled = !m.settings.AutoplayEnabled
	if m.settings.AutoplayEnabled {
		m.setStatus("Autoplay: ON")
	} else {
		m.setStatus("Autoplay: OFF")
	}
	return saveSettingsCmd(m.db, m.settings)
}

func (m *Model) toggleShuffleAction() tea.Cmd {
	m.queue.ToggleShuffle()
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
	m.updateMPRIS()
	return saveQueueCmd(m.db, m.queue)
}

// selectedTrack returns the track the cursor is on, wherever the cursor
// happens to be. The per-page branching matches what `i` and `x` each
// do inline; this is the one copy new actions should use.
func (m *Model) selectedTrack() (queue.Track, bool) {
	switch {
	case m.activePage == PageSettings:
		return queue.Track{}, false

	case m.activePanel == PanelQueue && m.queue.Len() > 0:
		idx := min(m.queueCursor, m.queue.Len()-1)
		return m.queue.Tracks()[idx], true

	case m.activePage == PageFavorites:
		if m.favCursor >= 0 && m.favCursor < len(m.favorites) {
			return m.favorites[m.favCursor], true
		}

	case m.activePage == PageHistory:
		if m.historyCursor >= 0 && m.historyCursor < len(m.history) {
			return historyEntryTrack(m.history[m.historyCursor]), true
		}

	case m.activePage == PageLibrary:
		tracks := m.filteredLibrary()
		if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
			return tracks[m.libraryCursor], true
		}

	default: // Stream page: search results, or an open album's tracks
		list := m.results
		if m.streamShowsTracks() {
			list = m.streamTracks()
		}
		if m.searchCursor >= 0 && m.searchCursor < len(list) {
			return m.resolveTrack(list[m.searchCursor]), true
		}
	}
	return queue.Track{}, false
}

// copyLinkAction puts the highlighted track's YouTube Music URL on the
// clipboard. Built from the video id rather than the track's own URL:
// that field is empty until playback resolves it, and for a downloaded
// track it is a local path.
func (m *Model) copyLinkAction() tea.Cmd {
	t, ok := m.selectedTrack()
	if !ok {
		m.setStatus("Nothing selected to copy")
		return nil
	}
	if !ytmusic.IsVideoID(t.ID) {
		// Local library files and pre-YouTube history rows have no link.
		m.setStatus("No link for " + t.Title)
		return nil
	}
	url := ytmusic.ShareURL(t.ID, m.settings.CopyMusicLinks)
	if err := clipboard.Copy(url); err != nil {
		if errors.Is(err, clipboard.ErrNoTool) {
			m.setStatus("No clipboard tool — " + clipboard.InstallHint())
		} else {
			m.setStatus("Copy failed: " + err.Error())
		}
		return nil
	}
	m.setStatus("Copied link: " + url)
	return nil
}

// openAlbumOfSelected opens the release the highlighted song belongs
// to. Works wherever a track is listed, an artist's top songs included:
// their rows carry the album id, so a song on an artist's page opens
// into its release and esc steps back to the artist.
func (m *Model) openAlbumOfSelected() tea.Cmd {
	t, ok := m.selectedTrack()
	if !ok {
		m.setStatus("Nothing selected")
		return nil
	}
	if t.AlbumBrowseID == "" {
		// Local files, legacy rows, and singles that belong to no
		// release page.
		name := t.Album
		if name == "" {
			name = t.Title
		}
		m.setStatus("No album page for " + name)
		return nil
	}
	m.isLoadingAlbum = true
	m.albumSeq++
	// The song's own thumbnail is the album's art; the fetched page
	// refines it in handleAlbumTracks.
	m.albumCoverURL = t.CoverURL
	name := t.Album
	if name == "" {
		name = "album"
	}
	m.browseLoadingName = name
	m.setStatus("Opening " + name + "…")
	return openAlbumCmd(ytmusic.Album{BrowseID: t.AlbumBrowseID}, m.albumSeq)
}

// browsingHere reports whether the artist or album on screen is what
// the cursor is actually in. With the focus in the queue, the
// highlighted queue track is the subject — a and A act on it — rather
// than on the page behind it, which is not where the user is pointing.
func (m Model) browsingHere() bool {
	return m.activePage == PageStream && m.activePanel == PanelSearch
}

// openArtistOfSelected opens the artist page of the highlighted track.
// The album a song came from is reachable from there, so this is the
// only "go to" key.
func (m *Model) openArtistOfSelected() tea.Cmd {
	t, ok := m.selectedTrack()
	if !ok {
		m.setStatus("Nothing selected")
		return nil
	}
	who := t.Artist
	if who == "" {
		who = t.Title
	}
	if t.ArtistBrowseID == "" && t.AlbumBrowseID == "" {
		// Local files, and the occasional track whose byline names an
		// artist YouTube Music has no page for.
		m.setStatus("No artist page for " + who)
		return nil
	}
	m.isLoadingArtist = true
	m.artistSeq++
	m.browseLoadingName = who
	m.setStatus("Opening " + who + "…")
	if t.ArtistBrowseID == "" {
		// Queue and history rows saved before tracks carried an artist
		// id; the album they name knows who made it.
		return artistViaAlbumCmd(t.AlbumBrowseID, m.artistSeq)
	}
	return openArtistCmd(t.ArtistBrowseID, m.artistSeq)
}

// cancelBrowseLoad drops an artist or album fetch still in flight and
// leaves whatever was on screen before it started. Bumping the sequence
// is the cancel: the response still arrives, and its handler discards
// it for being a generation behind.
func (m *Model) cancelBrowseLoad() tea.Cmd {
	var cmd tea.Cmd
	if m.isLoadingAlbum {
		cmd = m.leaveAlbumView()
	}
	if m.isLoadingArtist {
		// Only the incoming artist is dropped — an artist page already
		// open stays, since that is the screen being returned to.
		m.artistSeq++
		m.isLoadingArtist = false
	}
	m.browseLoadingName = ""
	return cmd
}

// leaveArtistView closes the artist page, if one is open.
func (m *Model) leaveArtistView() {
	m.openArtist = nil
	m.artistShowsAlbums = false
	// Bumping the sequence drops a fetch still in flight. Without it,
	// escaping during a load did not cancel anything: the response
	// landed a second later and opened the page the user had left.
	if m.isLoadingArtist {
		m.artistSeq++
		m.isLoadingArtist = false
	}
}
