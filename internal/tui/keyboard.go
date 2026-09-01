package tui

import (
	"fmt"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	ver "ytmgo/internal/version"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// handleKey processes all tea.KeyMsg events. Extracted from Update so each
// message-handler family lives in its own focused file.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// ── Search-input focus: route letters/numbers to textinput ──
	// Checked *before* global keys so typing "o", "R", "1" etc. in
	// the search box works instead of triggering page/action shortcuts.
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
			if m.activePage == PageLibrary {
				// On Library page, Enter just exits the search field (filtering already happened live)
				return m, nil
			}
			if query != "" {
				m.recsSeq++ // invalidate any pending recommendations
				m.showingRecommendations = false
				m.isSearching = true
				m.err = nil
				m.resetStreamCursor()
				if m.albumMode {
					coverCmd := m.leaveAlbumView()
					m.albums = nil
					m.albumQuery = query
					return m, tea.Batch(coverCmd, searchAlbumsCmd(query, m.settings.SearchLimit))
				}
				m.results = nil
				return m, searchCmd(query, m.settings.SearchLimit)
			}
			// Empty query submitted — bring the recommendations back.
			if !m.showingRecommendations {
				return m, m.showRecommendations()
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
		if m.activePage == PageLibrary {
			m.clampLibraryOffset()
		}
		// Clearing the search box on the Stream page brings the
		// recommendations back without waiting for Enter.
		if m.activePage == PageStream && !m.showingRecommendations && m.searchInput.Value() == "" {
			return m, tea.Batch(cmd, m.showRecommendations())
		}
		return m, cmd
	}

	// ── Settings string editing: route letters/numbers to textinput ──
	// Checked before global keys so typing "o" in a path field works.
	if m.settingsEditField {
		switch msg.String() {
		case "esc":
			m.settingsEditField = false
			m.settingsEditInput.Blur()
			return m, nil
		case "enter":
			// Handled in the Enter case below — let it fall through
		default:
			var cmd tea.Cmd
			m.settingsEditInput, cmd = m.settingsEditInput.Update(msg)
			return m, cmd
		}
	}

	// ── Global keys (only fire when no text input is focused) ──
	if handled, cmd := m.handleGlobalKey(msg); handled {
		return m, cmd
	}

	// When confirming a destructive action, route the key press.
	// All confirmations follow the same pattern: Enter to confirm,
	// Esc to cancel. Page-nav keys (1/2/3) cancel the confirmation
	// and fall through to their usual page-switch handler. Every
	// other key is ignored so the prompt stays visible.
	if m.isConfirming() {
		key := msg.String()
		confirmed := key == "enter"
		switch {
		case confirmed:
			return m.executeConfirmedAction()
		case key == "esc":
			m.clearConfirm()
			m.setStatus("Cancelled")
			return m, nil
		case key == "1" || key == "2" || key == "3":
			m.clearConfirm()
			// fall through — let the global page-nav handler run
		default:
			return m, nil // keep prompt visible
		}
	}

	// ── Global keybindings ───────────────────────────────
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		m.Shutdown()
		return m, tea.Quit

	case "?":
		m.switchPage(PageSettings)
		return m, nil

	case "tab":
		switch m.activePage {
		case PageSettings:
			// Tab does nothing on settings — arrows navigate the list.
			return m, nil
		case PageFavorites, PageLibrary, PageHistory:
			// Favorites/Library/History page: search input ↔ list
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
				m.activePanel = PanelSearch
			} else if m.activePanel == PanelSearch {
				m.activePanel = PanelQueue
			} else {
				m.activePanel = PanelSearch
				m.searchFocused = true
				m.searchInput.Focus()
			}
		default: // PageStream
			// 3-state cycle: search input → search results → queue → search input
			if m.searchFocused {
				m.searchFocused = false
				m.searchInput.Blur()
				m.activePanel = PanelSearch
			} else if m.activePanel == PanelSearch {
				m.activePanel = PanelQueue
			} else {
				m.activePanel = PanelSearch
				m.searchFocused = true
				m.searchInput.Focus()
			}
		}
		return m, nil

	case "esc":
		// Inside an album: step back to the album list. The preview's
		// cover override leaves with it, restoring the playing art.
		if m.activePage == PageStream && (m.openAlbum != nil || m.isLoadingAlbum || m.albumCoverURL != "") {
			coverCmd := m.leaveAlbumView()
			m.resetStreamCursor()
			m.setStatus("Albums")
			return m, coverCmd
		}
		if m.activePage == PageSettings && m.settingsEditField {
			// Cancel inline editing on Settings page
			m.settingsEditField = false
			m.settingsEditInput.Blur()
			return m, nil
		}
		// Otherwise Esc does nothing outside edit mode.
		return m, nil

	// ── Panel navigation ─────────────────────────────────
	case "up", "k":
		// Settings page: navigate settings list
		if m.activePage == PageSettings && !m.settingsEditField {
			if m.settingsCursor > 0 {
				m.settingsCursor--
				m.clampSettingsOffset()
			}
			return m, nil
		}
		// Panel navigation
		switch m.activePanel {
		case PanelSearch:
			switch m.activePage {
			case PageHistory:
				if m.historyCursor > 0 {
					m.historyCursor--
					m.clampHistoryOffset()
				}
			case PageFavorites:
				if m.favCursor > 0 {
					m.favCursor--
					m.clampFavoritesOffset()
				}
			case PageLibrary:
				if m.libraryCursor > 0 {
					m.libraryCursor--
					m.clampLibraryOffset()
				}
			default:
				if m.searchCursor > 0 {
					m.searchCursor--
					m.clampSearchOffset()
				}
			}
		case PanelQueue:
			if m.queueCursor > 0 {
				m.queueCursor--
				m.clampQueueOffset()
			}
		}
		return m, nil

	case "down", "j":
		// Settings page: navigate settings list
		if m.activePage == PageSettings && !m.settingsEditField {
			if m.settingsCursor < len(settingDefs)-1 {
				m.settingsCursor++
				m.clampSettingsOffset()
			}
			return m, nil
		}
		// Panel navigation
		switch m.activePanel {
		case PanelSearch:
			switch m.activePage {
			case PageHistory:
				maxIdx := len(m.history) - 1
				if m.historyCursor < maxIdx {
					m.historyCursor++
					m.clampHistoryOffset()
				}
			case PageFavorites:
				maxIdx := len(m.favorites) - 1
				if m.favCursor < maxIdx {
					m.favCursor++
					m.clampFavoritesOffset()
				}
			case PageLibrary:
				maxIdx := len(m.filteredLibrary()) - 1
				if m.libraryCursor < maxIdx {
					m.libraryCursor++
					m.clampLibraryOffset()
				}
			default:
				if m.searchCursor < m.streamListLen()-1 {
					m.searchCursor++
					m.clampSearchOffset()
				}
			}
		case PanelQueue:
			if m.queueCursor < m.queue.Len()-1 {
				m.queueCursor++
				m.clampQueueOffset()
			}
		}
		return m, nil

	case "enter":
		// ── Settings page: toggle/edit setting (see settingDefs) ──
		if m.activePage == PageSettings {
			return m.activateSettingsItem()
		}

		// ── Other pages: activate the focused item (see actions.go) ──
		return m, m.activateSelection()

	case " ":
		return m, m.togglePlayPause()

	case "n", "right":
		return m, m.nextTrack()

	case "p", "left":
		return m, m.prevTrack()

	case "A":
		// Toggle the Stream search between songs and albums.
		if m.activePage != PageStream {
			return m, nil
		}
		m.albumMode = !m.albumMode
		coverCmd := m.leaveAlbumView()
		m.resetStreamCursor()
		if !m.albumMode {
			// Back to songs: restore whatever the results panel had.
			m.setStatus("Searching songs")
			if len(m.results) == 0 {
				return m, tea.Batch(coverCmd, m.showRecommendations())
			}
			return m, coverCmd
		}
		// Album results are kept across toggles: only refetch when the
		// query actually changed, so flipping A back and forth is free.
		q := m.searchInput.Value()
		switch {
		case q != "" && (len(m.albums) == 0 || m.albumQuery != q):
			m.albumQuery = q
			m.isSearching = true
			m.setStatus("Searching albums…")
			return m, tea.Batch(coverCmd, searchAlbumsCmd(q, m.settings.SearchLimit))
		case len(m.albums) > 0:
			m.setStatus(fmt.Sprintf("Albums — %d results", len(m.albums)))
		default:
			m.setStatus("Albums — type a query and press Enter")
		}
		return m, coverCmd

	case "i":
		// Open the album page of the highlighted track, from any list
		// that knows it: search results, queue, favorites, history.
		return m, m.openAlbumOfSelected()

	case "a":
		// Queue every track of the open album.
		if m.activePage == PageStream && m.openAlbum != nil && len(m.albumTracks) > 0 {
			var cmd tea.Cmd
			for i, r := range m.albumTracks {
				t := m.resolveTrack(r)
				if i == 0 {
					cmd = m.enqueueAndMaybePlay(t)
					continue
				}
				m.queue.Add(t)
			}
			m.setStatus(fmt.Sprintf("Queued %d tracks from %s", len(m.albumTracks), m.openAlbum.Title))
			return m, tea.Batch(cmd, saveQueueCmd(m.db, m.queue))
		}
		return m, nil

	case "v":
		// The visualizer: the spectrum beneath the results, on every
		// page that has a results list.
		if m.activePage == PageSettings {
			m.setStatus("The visualizer is not shown on the settings page")
			return m, nil
		}
		if !m.npOn && !m.npFits() {
			m.setStatus("Terminal too short for the visualizer — make the window taller")
			return m, nil
		}
		// Only the flag is flipped here. Starting and stopping the
		// spectrum, and the cover's kitty escapes, are reconciled from
		// the visibility change in Update, which also covers the panel
		// going off screen because the page changed.
		m.npOn = !m.npOn
		if m.npOn {
			m.setStatus("Visualizer on  ([v] hide)")
			m.clampSearchOffset()
			return m, m.refreshCoverCmd()
		}
		m.setStatus("Visualizer off")
		// The spectrum was clocking redraws; restart the ticker.
		return m, m.resumePlayerTick()

	case "y":
		// Lyrics live under the queue in the right column, independent
		// of the now-playing panel on the left.
		if m.activePage == PageSettings {
			m.setStatus("The lyrics pane is not shown on the settings page")
			return m, nil
		}
		if !m.lyricsOn && !m.lyricsFits() {
			m.setStatus("Terminal too short for the lyrics pane — make the window taller")
			return m, nil
		}
		m.lyricsOn = !m.lyricsOn
		m.clampQueueOffset()
		if !m.lyricsOn {
			m.setStatus("Lyrics off")
			return m, nil
		}
		m.setStatus("Lyrics on  ([y] hide)")
		if t, ok := m.queue.Current(); ok {
			if cmd := m.loadLyricsCmd(t); cmd != nil {
				return m, cmd
			}
		}
		return m, nil

	case "X":
		// Downloads live on their own page; X jumps there and back.
		if m.activePage == PageDownloads {
			m.switchPage(PageStream)
		} else {
			m.switchPage(PageDownloads)
		}
		return m, nil

	case "g":
		m.moveCursorToEdge(false)
		return m, nil

	case "G":
		m.moveCursorToEdge(true)
		return m, nil

	case "l", "ctrl+f":
		m.position = min(m.position+5, m.duration)
		if m.player != nil {
			m.player.Seek(5)
		}
		m.updatePresence()
		return m, nil

	case "L":
		// L now behaves the same as "3" (always go to Library, consistent behavior)
		if m.activePage != PageLibrary {
			m.switchPage(PageLibrary)
			msg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
			if len(m.library) == 0 {
				msg = "No downloaded tracks"
			}
			m.setStatus(msg)
		}
		return m, nil

	case "f":
		// Toggle favorite on the highlighted track.
		switch {
		case m.activePage == PageFavorites && m.activePanel == PanelSearch:
			if len(m.favorites) > 0 && m.favCursor >= 0 && m.favCursor < len(m.favorites) {
				return m, m.toggleFavorite(m.favorites[m.favCursor])
			}
		case m.activePage == PageLibrary && m.activePanel == PanelSearch:
			tracks := m.filteredLibrary()
			if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
				return m, m.toggleFavorite(tracks[m.libraryCursor])
			}
		case m.activePanel == PanelQueue && m.queue.Len() > 0 && m.queueCursor >= 0 && m.queueCursor < m.queue.Len():
			t := m.queue.Tracks()[m.queueCursor]
			return m, m.toggleFavorite(t)
		default:
			// Stream page: search results, recommendations, or an open
			// album's tracks — whichever list owns the cursor. (The bare
			// album list shows albums, which are not favoritable.)
			if m.activePage != PageSettings && m.activePanel == PanelSearch {
				list := m.results
				if m.openAlbum != nil {
					list = m.albumTracks
				}
				if len(list) > 0 && m.searchCursor >= 0 && m.searchCursor < len(list) {
					r := list[m.searchCursor]
					t := m.resolveTrack(r)
					return m, m.toggleFavorite(t)
				}
			}
		}
		return m, nil

	case "h", "ctrl+b":
		m.position = max(m.position-5, 0)
		if m.player != nil {
			m.player.Seek(-5)
		}
		return m, nil

	case "+", "=":
		// Settings page: adjust number settings (see settingDefs)
		if m.activePage == PageSettings && !m.settingsEditField {
			return m, m.adjustSetting(+1)
		}
		// Global: volume up
		return m, m.changeVolume(+5)

	case "-", "_":
		// Settings page: adjust number settings (see settingDefs)
		if m.activePage == PageSettings && !m.settingsEditField {
			return m, m.adjustSetting(-1)
		}
		// Global: volume down
		return m, m.changeVolume(-5)

	case "x":
		// Download a track for offline use.
		// Works from either the search results panel (download the highlighted result)
		// or the queue panel (download the highlighted queue track).
		switch {
		case m.activePage == PageStream && m.activePanel == PanelSearch:
			// Album list: x grabs the whole album into its own folder.
			if m.openAlbum == nil && m.albumMode {
				if len(m.albums) == 0 || m.searchCursor < 0 || m.searchCursor >= len(m.albums) {
					return m, nil
				}
				a := m.albums[m.searchCursor]
				m.setStatus("Fetching " + a.Title + "…")
				return m, downloadAlbumCmd(a, m.downloadDir())
			}
			// Inside an album, x downloads the highlighted track only.
			list := m.results
			if m.openAlbum != nil {
				list = m.albumTracks
			}
			if len(list) == 0 || m.searchCursor < 0 || m.searchCursor >= len(list) {
				return m, nil
			}
			r := list[m.searchCursor]
			t := m.resolveTrack(r)
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			// IsDownloaded checks the actual filesystem using the
			// current download format (m4a/mp3), so switching format
			// in Settings correctly allows re-downloading in the new
			// format. The old t.Downloaded guard from resolveTrack
			// was format-agnostic and blocked re-downloads — removed.
			if m.downloader.IsDownloaded(t.ID, t.Title, r.Uploader, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: r.Uploader,
					CoverURL: r.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, r.Uploader, t.URL, m.downloadDir(), r.CoverURL)
			m.switchPage(PageDownloads) // show the job it just started
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageStream && m.activePanel == PanelQueue && m.queue.Len() > 0:
			if m.queueCursor < 0 || m.queueCursor >= m.queue.Len() {
				return m, nil
			}
			t := m.queue.Tracks()[m.queueCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(t.ID, t.Title, t.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: t.Artist,
					CoverURL: t.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir(), t.CoverURL)
			m.switchPage(PageDownloads) // show the job it just started
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageFavorites && m.activePanel == PanelSearch:
			if len(m.favorites) == 0 || m.favCursor < 0 || m.favCursor >= len(m.favorites) {
				return m, nil
			}
			t := m.favorites[m.favCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(t.ID) {
				m.setStatus("Already in download queue: " + t.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(t.ID, t.Title, t.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + t.Title)
				return m, nil
			}
			// Resolve YouTube URL only when we know we need to download.
			if t.URL == "" {
				m.pendingResolve = &pendingDownloadResolve{
					TrackID:  t.ID,
					Title:    t.Title,
					Uploader: t.Artist,
					CoverURL: t.CoverURL,
					Action:   "download",
				}
				m.setStatus("Fetching URL…")
				return m, resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
			}
			m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir(), t.CoverURL)
			m.switchPage(PageDownloads) // show the job it just started
			m.setStatus("Download queued: " + t.Title)
			return m, downloadCmd(m.downloader)

		case m.activePage == PageHistory && m.activePanel == PanelSearch:
			if len(m.history) == 0 || m.historyCursor < 0 || m.historyCursor >= len(m.history) {
				return m, nil
			}
			e := m.history[m.historyCursor]
			m.ensureDownloader()
			if m.downloader.HasPendingJob(e.TrackID) {
				m.setStatus("Already in download queue: " + e.Title)
				return m, nil
			}
			if m.downloader.IsDownloaded(e.TrackID, e.Title, e.Artist, m.downloadDir()) {
				m.setStatus("Already downloaded: " + e.Title)
				return m, nil
			}
			// New-style entries store a videoId, so the watch URL is
			// known without a resolve; legacy entries still resolve.
			if t := historyEntryTrack(e); t.URL != "" {
				m.downloader.Enqueue(t.ID, t.Title, t.Artist, t.URL, m.downloadDir(), t.CoverURL)
				m.switchPage(PageDownloads) // show the job it just started
				m.setStatus("Download queued: " + t.Title)
				return m, downloadCmd(m.downloader)
			}
			m.pendingResolve = &pendingDownloadResolve{
				TrackID:  e.TrackID,
				Title:    e.Title,
				Uploader: e.Artist,
				CoverURL: e.CoverURL,
				Action:   "download",
			}
			m.setStatus("Fetching URL…")
			return m, resolveURLCmd(e.Artist, e.Title, m.pendingResolve)
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
				m.updatePresence()
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
				m.updatePresence()
			}
			return m, saveQueueCmd(m.db, m.queue)
		}
		// Library page: delete a downloaded track from disk (requires confirmation)
		if m.activePage == PageLibrary && m.activePanel == PanelSearch && !m.searchFocused {
			tracks := m.filteredLibrary()
			if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
				t := tracks[m.libraryCursor]
				m.startConfirm(confirmDeleteTrack, t.Title)
				// Styled confirmation: orange bullet, white action, mint Enter, pink Esc
				bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
				action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Delete")
				title := lipgloss.NewStyle().Foreground(colorText).Render("\"" + t.Title + "\"?")
				enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
				enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
				escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
				escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
				m.setStatus(bullet + " " + action + " " + title + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			}
			return m, nil
		}
		return m, nil

	case "D":
		if m.queue.Len() == 0 {
			return m, nil
		}
		if !m.isConfirming() {
			m.startConfirm(confirmClearQueue, "")
			bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
			action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Clear queue?")
			enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
			enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
			escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
			escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
			m.setStatus(bullet + " " + action + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			return m, nil
		}
		return m, nil

	case "U":
		if m.updateAvailable != "" && m.updateAvailable != "latest" {
			// Update already known — start confirmation immediately
			m.startConfirm(confirmUpdate, m.updateAvailable)
			// Styled confirmation: orange bullet, white action, mint Enter, pink Esc
			bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
			action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Update to")
			verStr := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render(m.updateAvailable + "?")
			enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
			enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
			escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
			escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
			m.setStatus(bullet + " " + action + " " + verStr + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			return m, nil
		}
		m.updateCheckManual = true
		m.setStatus("Checking for updates…")
		return m, checkUpdateCmd(ver.Version)

	case "C":
		if m.activePage != PageHistory || len(m.history) == 0 {
			return m, nil
		}
		if !m.isConfirming() {
			m.startConfirm(confirmClearHistory, "")
			bullet := lipgloss.NewStyle().Foreground(colorWarning).Render("●")
			action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Clear history?")
			enterKey := lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("[Enter]")
			enterDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("yes")
			escKey := lipgloss.NewStyle().Foreground(colorAccent3).Bold(true).Render("[Esc]")
			escDesc := lipgloss.NewStyle().Foreground(colorTextDim).Render("no")
			m.setStatus(bullet + " " + action + "  " + enterKey + " " + enterDesc + "  " + escKey + " " + escDesc)
			return m, nil
		}
		return m, nil

	case "s":
		return m, m.toggleShuffleAction()

	case "r":
		return m, m.cycleRepeatAction()

	case "ctrl+up":
		if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor > 0 {
			m.queue.MoveUp(m.queueCursor)
			m.queueCursor--
		}
		return m, saveQueueCmd(m.db, m.queue)

	case "ctrl+down":
		if m.activePage == PageStream && m.activePanel == PanelQueue && m.queueCursor < m.queue.Len()-1 {
			m.queue.MoveDown(m.queueCursor)
			m.queueCursor++
		}
		return m, saveQueueCmd(m.db, m.queue)
	}

	return m, nil
}

// handleGlobalKey dispatches the key against the keymap's global
// bindings (page switch, refresh recs, open download dir). If a
// binding matches, its action runs and handled=true is returned.
// Called first by Update so a focused text input cannot swallow
// these keys.
func (m *Model) handleGlobalKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	for _, b := range Keys.Globals() {
		if !key.Matches(msg, b) {
			continue
		}
		// Matched a global — run the action. The case label and the
		// binding must agree; if a key is renamed in keys.go, update
		// both places.
		switch msg.String() {
		case "1": // Keys.PageStream
			if m.activePage != PageStream {
				m.switchPage(PageStream)
				m.setStatus("")
			}
			return true, nil
		case "2": // Keys.PageFavorites
			if m.activePage != PageFavorites {
				m.switchPage(PageFavorites)
				count := len(m.favorites)
				statusMsg := fmt.Sprintf("Favorites: %d tracks  (F to toggle)", count)
				if count == 0 {
					statusMsg = "No favorites yet — press F on any track"
				}
				m.setStatus(statusMsg)
			}
			return true, nil
		case "3": // Keys.PageLibrary
			if m.activePage != PageLibrary {
				m.switchPage(PageLibrary)
				statusMsg := fmt.Sprintf("Library: %d tracks  (type to filter)", len(m.library))
				if len(m.library) == 0 {
					statusMsg = "No downloaded tracks"
				}
				m.setStatus(statusMsg)
			}
			return true, nil
		case "4": // Keys.PageHistory
			if m.activePage != PageHistory {
				m.switchPage(PageHistory)
				m.setStatus("")
			}
			m.loadPlayHistory()
			return true, nil
		case "5": // Keys.PageDownloads
			if m.activePage != PageDownloads {
				m.switchPage(PageDownloads)
				n := 0
				if m.downloader != nil {
					n = len(m.downloader.Jobs())
				}
				if n == 0 {
					m.setStatus("No downloads yet — press x on any track")
				} else {
					m.setStatus(fmt.Sprintf("Downloads: %d", n))
				}
			}
			return true, nil
		case "6": // Keys.PageSettings
			if m.activePage != PageSettings {
				m.switchPage(PageSettings)
				m.setStatus("")
			}
			return true, nil
		case "R": // Keys.Recs
			if m.activePage != PageStream {
				m.switchPage(PageStream)
			}
			m.recsSeq++
			m.showingRecommendations = true
			m.results = nil
			m.searchCursor = 0
			m.searchOffset = 0
			m.setStatus("Loading recommendations…")
			return true, fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.db)
		case "o": // Keys.Open
			path := m.downloadDir()
			if err := openInOS(path); err != nil {
				m.setStatus("Failed to open: " + err.Error())
			} else {
				m.setStatus("Opened: " + path)
			}
			return true, nil
		}
		// Matched a global we don't have an action for here.
		return true, nil
	}
	return false, nil
}

// toggleFavorite adds or removes a track from the favorites list.
// loadPlayHistory loads play history from the database synchronously.
// Must be called after switchPage(PageHistory) since switchPage sets
// historyLoaded = false. This is used by both keyboard "4" and mouse
// click on the History tab.
func (m *Model) loadPlayHistory() {
	if m.db == nil {
		m.historyLoaded = true
		return
	}
	entries, loadErr := m.db.LoadPlayHistory(100, 0)
	if loadErr != nil {
		m.err = loadErr
	} else {
		m.history = entries
	}
	m.historyLoaded = true
}

// Returns a saveFavoritesCmd so the caller can batch it.
func (m *Model) toggleFavorite(t queue.Track) tea.Cmd {
	if m.favoriteSet[t.ID] {
		// Remove
		delete(m.favoriteSet, t.ID)
		for i, ft := range m.favorites {
			if ft.ID == t.ID {
				m.favorites = append(m.favorites[:i], m.favorites[i+1:]...)
				break
			}
		}
		m.setStatus("Removed from favorites: " + t.Title)
	} else {
		// Add — prepend so most recent shows first
		m.favoriteSet[t.ID] = true
		m.favorites = append([]queue.Track{t}, m.favorites...)
		m.setStatus("Added to favorites: " + t.Title)
	}
	return saveFavoritesCmd(m.db, m.favorites)
}
