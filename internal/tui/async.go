package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"ytmgo/internal/clipboard"
	"ytmgo/internal/downloader"
	"ytmgo/internal/lastfm"
	"ytmgo/internal/lyrics"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	ver "ytmgo/internal/version"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Async search results ─────────────────────────────────────────────

func (m Model) handleSearchResults(msg SearchResultsMsg) (tea.Model, tea.Cmd) {
	m.isSearching = false
	// The user cleared the search while this request was in flight and
	// recommendations are showing again — drop the stale results.
	if m.showingRecommendations {
		return m, nil
	}
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Search failed: " + msg.Error.Error())
	} else {
		m.results = msg.Results
		m.searchCursor = 0
		m.searchOffset = 0
		switch {
		case msg.Notice != "":
			m.setStatus(msg.Notice)
		case len(msg.Results) > 0:
			m.setStatus(fmt.Sprintf("Found %d results", len(msg.Results)))
		default:
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
	m.recsLoaded = true
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Recommendations unavailable: " + msg.Error.Error())
	} else {
		m.results = msg.Results
		m.recommendations = msg.Results // cached; restored when a search is cleared
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

// ── Album opened (tracklist fetched) ─────────────────────────────────

func (m Model) handleAlbumTracks(msg AlbumTracksMsg) (tea.Model, tea.Cmd) {
	m.isLoadingAlbum = false
	// A superseded open (the user pressed `i` on another song, or
	// re-entered a different album) must not overwrite the newer one.
	if msg.Seq != m.albumSeq {
		return m, nil
	}
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Cannot open album: " + msg.Error.Error())
		return m, nil
	}
	alb := msg.Album
	m.openAlbum = &alb
	m.albumTracks = msg.Tracks
	// Fetch the album's cover for the header strip. Albums opened from
	// a track (i) sometimes lack their own CoverURL; the first track's
	// art is the same square.
	artURL := alb.CoverURL
	if artURL == "" && len(msg.Tracks) > 0 {
		artURL = msg.Tracks[0].CoverURL
	}
	var cmds []tea.Cmd
	if artURL != "" && artURL != m.albumArtURL {
		if cmd := loadAlbumArtCmd(artURL, m.albumSeq); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	// An album opened with i from a search or the queue has nothing
	// behind it, so esc would leave the artist entirely and A would
	// have no songs to flip to. Fetch the artist the page names and
	// stack it underneath, so every album sits in the same place.
	if m.openArtist == nil && alb.ArtistBrowseID != "" {
		cmds = append(cmds, artistBehindAlbumCmd(alb.ArtistBrowseID, alb.BrowseID))
	}
	m.resetStreamCursor()
	m.setStatus(fmt.Sprintf("%s — %d tracks  ([a] queue all songs · [esc] back)", alb.Title, len(msg.Tracks)))
	// The album page's own art replaces the provisional one from the
	// song result (usually identical, but the page is authoritative).
	if alb.CoverURL != "" {
		m.albumCoverURL = alb.CoverURL
	}
	return m, tea.Batch(cmds...)
}

// ── Album download requested ─────────────────────────────────────────

func (m Model) handleAlbumDownload(msg AlbumDownloadMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Album download failed: " + msg.Error.Error())
		return m, nil
	}
	alb := msg.Album
	m.ensureDownloader()
	// Number the stems so the folder sorts in album order, exactly as
	// the CLI does.
	width := len(fmt.Sprintf("%d", len(alb.Tracks)))
	for i, t := range alb.Tracks {
		stem := fmt.Sprintf("%0*d - %s", width, i+1, t.Title)
		m.downloader.EnqueueAs(t.VideoID, t.Title, alb.Artist,
			ytmusic.WatchURL(t.VideoID), msg.Dir, t.CoverURL, stem)
	}
	m.switchPage(PageDownloads) // an album download is an explicit x too
	m.setStatus(fmt.Sprintf("Downloading %s — %d tracks", alb.Title, len(alb.Tracks)))
	return m, downloadCmd(m.downloader)
}

// ── Lyrics loaded ────────────────────────────────────────────────────

func (m Model) handleLyricsLoaded(msg LyricsLoadedMsg) (tea.Model, tea.Cmd) {
	m.lyricsLoading = false
	// Stale — another track is playing now; a newer fetch is in flight.
	if msg.Seq != m.lyricsSeq || msg.TrackID != m.lyricsTrackID {
		return m, nil
	}
	switch {
	case msg.Error != nil && !errors.Is(msg.Error, lyrics.ErrNotFound):
		m.lyricLines = nil
		m.lyricsErr = "Lyrics unavailable: " + msg.Error.Error()
	case msg.Lyrics == nil || len(msg.Lyrics.Lines) == 0:
		m.lyricLines = nil
		m.lyricsErr = "No lyrics found"
	default:
		m.lyricsErr = ""
		m.lyricLines = msg.Lyrics.Lines
		m.lyricsSynced = msg.Lyrics.Synced
		m.lyricsOffset = 0
		m.lyricsFollow = true
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
		m.setStatus(fmt.Sprintf("Library: %d tracks", len(msg.Tracks)))
	}
	return m, nil
}

// maxFailedInARow is how many unplayable tracks pass before ytmgo stops
// and says so. Three is enough that a couple of genuinely dead streams
// still get skipped without comment, and few enough that a broken
// install is caught before the queue is gone.
const maxFailedInARow = 3

// ── Settings saved ───────────────────────────────────────────────────

func (m Model) handleSettingsSaved(msg SettingsSavedMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Failed to save settings: " + msg.Error.Error())
	} else if m.activePage == PageSettings {
		// Only the settings editor needs this: there, the saved row is
		// the whole feedback. Everywhere else the action that triggered
		// the save has already said what it did ("Autoplay: ON",
		// "Volume: 80%", "Key hints hidden") and this generic line, put
		// on screen a moment later, erased it.
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
	// may have been triggered since this one was dispatched. Matching on
	// TrackID+Action (not just non-nil) drops the reply of a superseded
	// request instead of letting the older pick win over the newer one.
	if m.pendingResolve == nil || m.pendingResolve.TrackID != msg.TrackID || m.pendingResolve.Action != msg.Action {
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
		m.switchPage(PageDownloads) // the deferred half of an explicit x
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

	// Keep the MPRIS Position property roughly current (no signal spam:
	// position changes are served on demand, not broadcast).
	m.updateMPRIS()

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
		autoplayCmd = fetchAutoplayCmd(m.db)
	}

	// Last.fm counts a track once it has been heard long enough, and
	// once only. Position is what the player reports, so a pause does
	// not count and a seek moves the needle the way Last.fm's own
	// clients let it.
	var scrobbleCmd tea.Cmd
	if m.settings.LastFMSessionKey != "" && !m.scrobbled &&
		m.playerState == player.StatePlaying &&
		lastfm.ShouldScrobble(m.position, m.duration) {
		if t, ok := m.queue.Current(); ok {
			m.scrobbled = true
			scrobbleCmd = lastfmScrobbleCmd(m.settings.LastFMSessionKey, t, m.scrobbleStart)
		}
	}

	// Keep listening
	if m.player != nil {
		cmds := []tea.Cmd{positionCmd(m.player)}
		if autoplayCmd != nil {
			cmds = append(cmds, autoplayCmd)
		}
		if scrobbleCmd != nil {
			cmds = append(cmds, scrobbleCmd)
		}
		return m, tea.Batch(cmds...)
	}
	return m, scrobbleCmd
}

// ── Last.fm ──────────────────────────────────────────────────────────

// handleLastFMToken has the token; the user now has to approve it. The
// link is opened in the browser and put on the clipboard, each as best
// effort, and the row's own description carries it for as long as the
// approval is pending — the status line fades and truncates, and the
// first version of this said a tab had opened when nothing had.
func (m Model) handleLastFMToken(msg LastFMTokenMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		m.setStatus("Last.fm: " + msg.Error.Error())
		return m, nil
	}
	m.lastfmToken = msg.Token
	m.lastfmTokenAt = time.Now()
	link := lastfm.AuthURL(msg.Token)
	opened := openInOS(link) == nil
	copied := clipboard.Copy(link) == nil
	switch {
	case opened && copied:
		m.setStatus("Last.fm: opened in your browser (link also copied) — click Allow and ytmgo will connect on its own")
	case opened:
		m.setStatus("Last.fm: opened in your browser — click Allow and ytmgo will connect on its own")
	case copied:
		m.setStatus("Last.fm: link copied — open it and click Allow; ytmgo will connect on its own")
	default:
		m.setStatus("Last.fm: open the link shown under the row and click Allow; ytmgo will connect on its own")
	}
	// From here the approval is watched for, not waited on.
	return m, lastfmPollCmd(msg.Token)
}

// handleLastFMPoll is the tick: check, unless the token it was set for
// is no longer the one pending.
func (m Model) handleLastFMPoll(msg LastFMPollMsg) (tea.Model, tea.Cmd) {
	if msg.Token == "" || msg.Token != m.lastfmToken {
		return m, nil
	}
	return m, lastfmSessionCmd(msg.Token, true)
}

// handleLastFMSession has the key, or the reason it does not.
func (m Model) handleLastFMSession(msg LastFMSessionMsg) (tea.Model, tea.Cmd) {
	if errors.Is(msg.Error, lastfm.ErrNotAuthorized) {
		if !msg.Polled {
			m.setStatus("Last.fm: not approved yet — click Allow on the Last.fm page; ytmgo is watching for it")
			return m, nil
		}
		if m.lastfmToken == "" {
			return m, nil // disconnected, or a new link was asked for
		}
		if time.Since(m.lastfmTokenAt) > lastfmPollTimeout {
			m.lastfmToken = ""
			m.setStatus("Last.fm: no approval after 10 minutes — press Enter for a fresh link")
			return m, nil
		}
		return m, lastfmPollCmd(m.lastfmToken)
	}
	if msg.Error != nil {
		m.lastfmToken = ""
		m.setStatus("Last.fm: " + msg.Error.Error())
		return m, nil
	}
	m.lastfmToken = ""
	m.settings.LastFMSessionKey = msg.Session.Key
	m.settings.LastFMUser = msg.Session.User
	m.setStatus("Last.fm: connected as " + msg.Session.User + " — scrobbling on")
	return m, saveSettingsCmd(m.db, m.settings)
}

func (m Model) handleLastFMErr(msg LastFMErrMsg) (tea.Model, tea.Cmd) {
	m.setStatus("Last.fm " + msg.Op + " failed: " + msg.Error.Error())
	return m, nil
}

// ── Song ended naturally (mpv exited / track finished) ───────────────

func (m Model) handleSongEnded(msg SongEndedMsg) (tea.Model, tea.Cmd) {
	// The endedCmd listener that delivered this message is gone; the
	// next startTrackPlayback re-arms it. Spurious ends are no longer
	// possible: the player only emits Ended for end-file reason "eof"
	// (or "error"), never for track switches or manual stops.
	m.endedListening = false

	// A track mpv could not play advances the queue like any other end,
	// so one dead stream is skipped rather than stalling everything. A
	// run of them is different: it means nothing can play, and carrying
	// on tears through the whole queue in seconds with nothing heard
	// and nothing said. Ubuntu's packaged yt-dlp is old enough to do
	// this to every track at once.
	if msg.Natural {
		m.failedInARow = 0
	} else {
		m.failedInARow++
		if m.failedInARow >= maxFailedInARow {
			m.failedInARow = 0
			m.playerState = player.StateStopped
			if m.player != nil {
				m.player.Stop()
			}
			m.position, m.duration = 0, 0
			m.updatePresence()
			m.setStatus("Nothing will play — mpv could not open " +
				strconv.Itoa(maxFailedInARow) + " tracks in a row. Update yt-dlp: yt-dlp -U")
			return m, nil
		}
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
				m.updatePresence()
				m.setStatus("Autoplay fetching suggestions…")
				return m, fetchAutoplayCmd(m.db)
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
				m.updatePresence()
				m.setStatus("Autoplay loading suggestions…")
				return m, nil
			}

			m.playerState = player.StateStopped
			m.player.Stop()
			m.position = 0
			m.duration = 0
			m.lastPosition = 0
			m.lastPositionAt = time.Time{}
			m.updatePresence()
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
			m.updatePresence()
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

// ── Artist page loaded ───────────────────────────────────────────────

func (m Model) handleArtistLoaded(msg ArtistLoadedMsg) (tea.Model, tea.Cmd) {
	m.isLoadingArtist = false
	// Context for an album opened with i: fill the page in underneath,
	// leaving the album, its art and the cursor exactly as they are.
	// If the user has moved on — escaped out, or opened something else
	// — the album it belongs to is no longer open and it is dropped.
	if msg.ForAlbum != "" {
		if msg.Error != nil || m.openAlbum == nil || m.openAlbum.BrowseID != msg.ForAlbum {
			return m, nil
		}
		a := msg.Artist
		m.openArtist = &a
		m.artistSongs = msg.Songs
		m.artistShowsAlbums = false
		m.artistArtURL = a.ThumbURL
		return m, nil
	}
	// A superseded open (the user pressed I on another track) must not
	// overwrite the newer one.
	if msg.Seq != m.artistSeq {
		return m, nil
	}
	if msg.Error != nil {
		m.err = msg.Error
		m.setStatus("Cannot open artist: " + msg.Error.Error())
		return m, nil
	}
	a := msg.Artist
	m.openArtist = &a
	// An artist page opens on its songs — this is a music player, and
	// the first thing wanted is something to play. Unless there are
	// none, in which case the releases are all there is to show.
	m.artistShowsAlbums = len(msg.Songs) == 0
	m.artistSongs = msg.Songs
	m.artistArtURL = a.ThumbURL
	// Leaving any open album behind: the artist replaces it.
	m.openAlbum = nil
	m.albumTracks = nil
	m.albumMode = m.artistShowsAlbums
	m.resetStreamCursor()
	m.setStatus(fmt.Sprintf("%s — %d songs, %d releases  ([A] switch · [esc] back)",
		a.Name, len(msg.Songs), len(a.Albums)))

	// The artist's photo goes through the album-art pipeline: the strip
	// draws it from the same field, and bumping albumSeq keeps the
	// transmit/delete bookkeeping that pipeline already does.
	//
	// The old image is dropped first. It belongs to whatever was open
	// before, and leaving it up means the artist's name sits over the
	// last album's cover until the fetch lands — or for good, if it
	// never does.
	var cmds []tea.Cmd
	if a.ThumbURL != m.albumArtURL {
		m.albumArtImg = nil
		m.albumArtURL = ""
		m.albumSeq++
		if c := loadAlbumArtCmd(a.ThumbURL, m.albumSeq); c != nil {
			cmds = append(cmds, c)
		}
	}
	return m, tea.Batch(cmds...)
}
