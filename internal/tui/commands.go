package tui

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"ytmgo/internal/coverart"
	"ytmgo/internal/db"
	"ytmgo/internal/downloader"
	"ytmgo/internal/library"
	"ytmgo/internal/lyrics"
	"ytmgo/internal/mpris"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/visualizer"
	"ytmgo/internal/ytmusic"
	"ytmgo/internal/ytresolve"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Intervals ─────────────────────────────────────────────────────────

// progressTickInterval drives the periodic 500ms tick for idle tip rotation
// and dev-mode position simulation.
const progressTickInterval = time.Second / 2

// playerTickInterval drives the smooth progress interpolation in the
// player bar. Each tick re-renders the whole frame, so the rate is a
// direct CPU cost: 100ms is plenty for a bar that advances about one
// cell per second, and halves the work the old 50ms did.
const playerTickInterval = 100 * time.Millisecond

// ─── Search ─────────────────────────────────────────────────────────────

// searchCmd fires a YouTube Music search in a goroutine and sends
// results back.
func searchCmd(query string, limit int) tea.Cmd {
	return func() tea.Msg {
		results, err := search.Search(query, limit)
		if err != nil {
			return SearchResultsMsg{Error: err}
		}
		if results == nil {
			results = []search.Result{} // never nil
		}
		return SearchResultsMsg{Results: results}
	}
}

// historySeeds returns the most recent unique videoIds from play
// history (newest first). Legacy entries (TIDAL numeric IDs, library
// file paths) are skipped: they can't seed a YouTube Music radio.
func historySeeds(database *db.DB, max int) []string {
	if database == nil {
		return nil
	}
	entries, err := database.LoadPlayHistory(50, 0)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var seeds []string
	for _, e := range entries {
		if seen[e.TrackID] || !ytmusic.IsVideoID(e.TrackID) {
			continue
		}
		seen[e.TrackID] = true
		seeds = append(seeds, e.TrackID)
		if len(seeds) >= max {
			break
		}
	}
	return seeds
}

// fetchRecommendationsCmd fires a request for YouTube Music radio
// recommendations seeded from the user's listening history.
// seq is the generation counter — stale responses are ignored.
func fetchRecommendationsCmd(seq, limit int, db *db.DB) tea.Cmd {
	return func() tea.Msg {
		results, err := search.FetchRecommendations(limit, historySeeds(db, 4))
		if err != nil {
			return RecommendationsMsg{Error: err, Seq: seq}
		}
		if results == nil {
			results = []search.Result{}
		}
		return RecommendationsMsg{Results: results, Seq: seq}
	}
}

// fetchAutoplayCmd fetches a small batch of recommendations to continue
// playback when the queue runs dry. Uses a fixed small limit so autoplay
// never floods the queue with dozens of tracks.
func fetchAutoplayCmd(db *db.DB) tea.Cmd {
	const autoplayBatch = 1
	return func() tea.Msg {
		results, err := search.FetchRecommendations(autoplayBatch, historySeeds(db, 4))
		if err != nil || len(results) == 0 {
			return nil // silent — no results to autoplay
		}
		tracks := make([]queue.Track, len(results))
		for i, r := range results {
			tracks[i] = r.ToTrack()
		}
		return AutoplayResultsMsg{Tracks: tracks}
	}
}

// ─── Albums ─────────────────────────────────────────────────────────

// searchAlbumsCmd runs an albums-filtered YouTube Music search.
func searchAlbumsCmd(query string, limit int) tea.Cmd {
	return func() tea.Msg {
		albums, err := ytmusic.SearchAlbums(query, limit)
		return AlbumResultsMsg{Albums: albums, Error: err}
	}
}

// openAlbumCmd fetches one album's tracklist and converts it to
// playable results, so the left panel can render it like any other list.
// seq is the generation counter — a slow response for a superseded
// open request is dropped by the handler instead of overwriting the
// album the user asked for later.
func openAlbumCmd(a ytmusic.Album, seq int) tea.Cmd {
	return func() tea.Msg {
		full, err := ytmusic.AlbumTracks(a.BrowseID)
		if err != nil {
			return AlbumTracksMsg{Album: a, Error: err, Seq: seq}
		}
		tracks := make([]search.Result, 0, len(full.Tracks))
		for _, t := range full.Tracks {
			cover := t.CoverURL
			if cover == "" {
				cover = full.CoverURL
			}
			tracks = append(tracks, search.Result{
				ID:            t.VideoID,
				Title:         t.Title,
				Uploader:      t.Artist,
				Album:         full.Title,
				Duration:      t.Duration,
				URL:           ytmusic.WatchURL(t.VideoID),
				CoverURL:      cover,
				AlbumBrowseID: full.BrowseID,
			})
		}
		return AlbumTracksMsg{Album: full, Tracks: tracks, Seq: seq}
	}
}

// AlbumDownloadMsg asks the model to enqueue a fetched album's tracks.
type AlbumDownloadMsg struct {
	Album ytmusic.Album
	Dir   string
	Error error
}

// downloadAlbumCmd fetches an album's tracklist so the model can enqueue
// every track into "<parent>/<Artist> - <Album>/".
func downloadAlbumCmd(a ytmusic.Album, parentDir string) tea.Cmd {
	return func() tea.Msg {
		full, err := ytmusic.AlbumTracks(a.BrowseID)
		if err != nil {
			return AlbumDownloadMsg{Album: a, Error: err}
		}
		folder := full.Title
		if full.Artist != "" {
			folder = full.Artist + " - " + full.Title
		}
		dir := filepath.Join(parentDir, sanitizeDirName(folder))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return AlbumDownloadMsg{Album: full, Error: err}
		}
		return AlbumDownloadMsg{Album: full, Dir: dir}
	}
}

// sanitizeDirName strips characters that are awkward in a folder name.
func sanitizeDirName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// ─── URL prefetch (background cache) ─────────────────────────────────

// prefetchURLCmd resolves a YouTube URL for the given track in the
// background and sends a URLPrefetchedMsg so the result is cached.
// Unlike resolveURLCmd this never triggers playback — the resolved URL
// just populates the cache for instant play when the track is reached.
func prefetchURLCmd(trackID, artist, title string) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				// Silent — a failed prefetch just means the track
				// will be resolved on demand when it's played.
			}
		}()
		url, err := ytresolve.ResolveURL(artist, title)
		if err != nil || url == "" {
			return nil // silent — no cache entry, will resolve on demand
		}
		return URLPrefetchedMsg{TrackID: trackID, URL: url}
	}
}

// ─── Update check ─────────────────────────────────────────────────────────

// checkUpdateCmd fetches the latest release tag from GitHub by following
// the /releases/latest redirect. No API key, no rate limits — just a
// single HTTP HEAD. Returns nil (no message) on any failure so the
// handler is never called — zero UX impact when offline.
func checkUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		if currentVersion == "dev" || currentVersion == "" {
			return nil
		}
		// Don't follow redirect — we want the Location header.
		client := &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get("https://github.com/anas1412/ytmgo/releases/latest")
		if err != nil {
			return nil
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
			return nil
		}
		loc := resp.Header.Get("Location")
		if loc == "" {
			return nil
		}
		latest := path.Base(loc) // e.g. "/…/tag/v0.2.0" → "v0.2.0"
		if latest == "" {
			return nil
		}
		return UpdateCheckMsg{LatestVersion: latest}
	}
}

// ─── Random quote fetch ─────────────────────────────────────────────

// quoteClient fetches idle quotes; bounded so a slow API can't pile up
// hung goroutines behind the 30s rotation.
var quoteClient = &http.Client{Timeout: 10 * time.Second}

// fetchQuoteCmd fetches a random quote from dummyjson.
// On failure it returns nil so the fallback quote stays displayed.
func fetchQuoteCmd(seq int) tea.Cmd {
	return func() tea.Msg {
		resp, err := quoteClient.Get("https://dummyjson.com/quotes/random")
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		var result struct {
			Quote  string `json:"quote"`
			Author string `json:"author"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil
		}
		if result.Quote == "" {
			return nil
		}
		return QuoteMsg{Quote: result.Quote, Author: result.Author, Seq: seq}
	}
}

// ─── Library scan ───────────────────────────────────────────────────────

// scanLibraryCmd scans the downloads directory for existing audio files.
// Durations are served from the SQLite cache; only new or changed files
// hit ffprobe, and the fresh results are persisted for the next run.
func scanLibraryCmd(dir string, database *db.DB) tea.Cmd {
	return func() tea.Msg {
		var cache library.DurationCache
		if database != nil {
			if c, err := database.LoadLibraryCache(); err == nil {
				cache = c
			}
		}
		tracks, updates, err := library.ScanDir(dir, cache)
		if err != nil {
			// Non-fatal — just return empty library
			return LibraryScanMsg{Tracks: []queue.Track{}}
		}
		if database != nil && len(updates) > 0 {
			_ = database.SaveLibraryCache(updates) // best-effort
		}
		return LibraryScanMsg{Tracks: tracks}
	}
}

// ─── Player commands ────────────────────────────────────────────────────

// positionCmd reads one position update from the mpv IPC poller.
func positionCmd(p *player.Player) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		pos, ok := <-p.Positions()
		if !ok {
			return nil
		}
		return PositionMsg{Position: pos.Position, Duration: pos.Duration}
	}
}

// endedCmd waits for mpv to finish playing the current track.
func endedCmd(p *player.Player) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		<-p.Ended()
		return SongEndedMsg{}
	}
}

// ─── MPRIS ──────────────────────────────────────────────────────────────

// mprisInitCmd starts the MPRIS D-Bus service in the background so
// media keys and desktop widgets can control playback. On systems
// without a session bus this silently does nothing.
func mprisInitCmd() tea.Cmd {
	return func() tea.Msg {
		svc, err := mpris.Start()
		if err != nil {
			return nil
		}
		return MprisReadyMsg{Svc: svc}
	}
}

// listenMprisCmd waits for one external control request and forwards
// it into the update loop. Re-armed by the MprisCmdMsg handler.
func listenMprisCmd(svc *mpris.Service) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		cmd, ok := <-svc.Commands()
		if !ok {
			return nil
		}
		return MprisCmdMsg{Cmd: cmd}
	}
}

// ─── Visualizer ─────────────────────────────────────────────────────

// vizFrameCmd waits for one spectrum frame. Re-armed by its handler so
// frames keep flowing while the visualizer is on.
func vizFrameCmd(v *visualizer.Visualizer) tea.Cmd {
	if v == nil {
		return nil
	}
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		f, ok := <-v.Frames()
		if !ok {
			return VizStoppedMsg{Err: v.Err()}
		}
		return VizFrameMsg{Frame: f}
	}
}

// ─── Cover art ──────────────────────────────────────────────────────

// loadAlbumArtCmd fetches the open album's cover for the browse strip.
func loadAlbumArtCmd(url string, seq int) tea.Cmd {
	if url == "" {
		return nil
	}
	return func() tea.Msg {
		img, err := coverart.Load(url)
		return AlbumArtLoadedMsg{URL: url, Img: img, Err: err, Seq: seq}
	}
}

// loadCoverCmd fetches and decodes one track's album art. Results are
// cached in the coverart package, so re-showing a cover is instant.
func loadCoverCmd(url string) tea.Cmd {
	return func() tea.Msg {
		img, err := coverart.Load(url)
		return CoverLoadedMsg{URL: url, Img: img, Err: err}
	}
}

// ─── Lyrics ─────────────────────────────────────────────────────────

// fetchLyricsCmd fetches lyrics for t: the SQLite cache first, then
// LRCLIB (synced) with an InnerTube plain-text fallback. Definitive
// misses are cached as "" so replays don't refetch; transient errors
// are never cached.
func fetchLyricsCmd(t queue.Track, seq int, database *db.DB) tea.Cmd {
	return func() tea.Msg {
		if database != nil && t.ID != "" {
			if text, synced, found, err := database.LoadCachedLyrics(t.ID); err == nil && found {
				return lyricsMsgFromCache(t.ID, text, synced, seq)
			}
		}
		l, err := lyrics.Fetch(t.ID, t.Title, t.Artist, t.DurationSec)
		if err != nil {
			if errors.Is(err, lyrics.ErrNotFound) && database != nil && t.ID != "" {
				_ = database.SaveCachedLyrics(t.ID, "", false) // known miss
			}
			return LyricsLoadedMsg{TrackID: t.ID, Error: err, Seq: seq}
		}
		if database != nil && t.ID != "" {
			_ = database.SaveCachedLyrics(t.ID, l.Raw, l.Synced) // best-effort
		}
		return LyricsLoadedMsg{TrackID: t.ID, Lyrics: l, Seq: seq}
	}
}

// lyricsMsgFromCache rebuilds a LyricsLoadedMsg from cached text: an
// empty text is a recorded miss, not an error.
func lyricsMsgFromCache(trackID, text string, synced bool, seq int) LyricsLoadedMsg {
	if text == "" {
		return LyricsLoadedMsg{TrackID: trackID, Seq: seq}
	}
	return LyricsLoadedMsg{TrackID: trackID, Lyrics: lyrics.FromText(text, synced), Seq: seq}
}

// ─── Tick commands ──────────────────────────────────────────────────────

// tickCmd returns a command that fires every 500ms for progress animation.
func tickCmd() tea.Cmd {
	return tea.Tick(progressTickInterval, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

// playerTickCmd fires every 50ms while a track is playing, so the
// progress bar can glide instead of jumping between coarse IPC
// position updates. The returned tea.Cmd re-arms itself from within
// Update when the player is still in the playing state.
func playerTickCmd() tea.Cmd {
	return tea.Tick(playerTickInterval, func(_ time.Time) tea.Msg {
		return playerTickMsg{}
	})
}

// loadLyricsCmd starts a lyrics fetch for t unless lyrics for that
// exact track are already loaded or in flight, or the lyrics view is
// off (no work for a panel that is never shown). Returns nil when
// there is nothing to do.
func (m *Model) loadLyricsCmd(t queue.Track) tea.Cmd {
	if !m.lyricsOn {
		return nil
	}
	if t.ID == "" || (t.ID == m.lyricsTrackID && (m.lyricsLoading || len(m.lyricLines) > 0)) {
		return nil
	}
	m.lyricsSeq++
	m.lyricsTrackID = t.ID
	m.lyricsLoading = true
	m.lyricsErr = ""
	m.lyricLines = nil
	m.lyricsSynced = false
	m.lyricsOffset = 0
	m.lyricsFollow = true
	return fetchLyricsCmd(t, m.lyricsSeq, m.db)
}

// ─── Settings ───────────────────────────────────────────────────────────

// saveSettingsCmd persists settings to the database in a goroutine.
func saveSettingsCmd(database *db.DB, s *settings.Settings) tea.Cmd {
	return func() tea.Msg {
		if database == nil {
			return SettingsSavedMsg{Error: fmt.Errorf("db not ready")}
		}
		if err := database.SaveSettings(s); err != nil {
			return SettingsSavedMsg{Error: err}
		}
		return SettingsSavedMsg{}
	}
}

// ─── Database ──────────────────────────────────────────────────────────

// initQueueFavoritesCmd loads queue + favorites from the already-open
// database. The DB is opened synchronously in InitialModel so that
// settings are available immediately — see model.go.
func initQueueFavoritesCmd(database *db.DB) tea.Cmd {
	return func() tea.Msg {
		if database == nil {
			return DbReadyMsg{Error: fmt.Errorf("db not initialized")}
		}
		tracks, shuffle, repeat, repeatAll, err := database.LoadQueue()
		if err != nil {
			return DbReadyMsg{Error: err}
		}
		favs, err := database.LoadFavorites()
		if err != nil {
			return DbReadyMsg{Error: err}
		}
		return DbReadyMsg{
			QueueTracks: tracks,
			Shuffle:     shuffle,
			Repeat:      repeat,
			RepeatAll:   repeatAll,
			Favorites:   favs,
		}
	}
}

// recordPlayCmd records a play history entry in the background.
func recordPlayCmd(database *db.DB, t queue.Track) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		if database == nil {
			return nil
		}
		if err := database.RecordPlay(t); err != nil {
			return PlayRecordedMsg{Error: err}
		}
		return PlayRecordedMsg{}
	}
}

// ─── Queue persistence ─────────────────────────────────────────────────

// saveQueueCmd persists the current queue to the database in a goroutine.
// Returns nil on success (silent saves — only errors produce a message).
func saveQueueCmd(database *db.DB, q *queue.Queue) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		if database == nil {
			return nil
		}
		tracks := q.Tracks()
		if err := database.SaveQueue(tracks, q.CurrentIndex(), q.IsShuffle(), q.IsRepeat(), q.IsRepeatAll()); err != nil {
			return nil // silent — queue is still in memory
		}
		return nil
	}
}

// ─── Favorites persistence ─────────────────────────────────────────────

// saveFavoritesCmd persists the favorites list to the database in a goroutine.
// Returns nil on success (silent saves).
func saveFavoritesCmd(database *db.DB, favorites []queue.Track) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		if database == nil {
			return nil
		}
		if err := database.SaveFavorites(favorites); err != nil {
			return nil // silent — favorites still in memory
		}
		return nil
	}
}

// ─── Downloader ─────────────────────────────────────────────────────────

// downloadCmd returns a command that reads one progress event from the
// downloader channel and forwards it as a DownloadProgressMsg.
func downloadCmd(d *downloader.Downloader) tea.Cmd {
	return func() (msg tea.Msg) {
		defer func() { recover() }()
		evt, ok := <-d.Progress()
		if !ok {
			return nil
		}
		return DownloadProgressMsg{
			TrackID:  evt.TrackID,
			Title:    evt.Title,
			Uploader: evt.Uploader,
			Progress: evt.Progress,
			Status:   evt.Status,
			Done:     evt.Status == downloader.StatusDone || evt.Status == downloader.StatusSkipped,
			FilePath: evt.FilePath,
			Error:    evt.Err,
		}
	}
}

// resolveURLCmd runs ytresolve.ResolveURL in a goroutine and sends the
// result back as an URLResolvedMsg. The caller must set m.pendingResolve
// before calling this.
func resolveURLCmd(artist, title string, pr *pendingDownloadResolve) tea.Cmd {
	return func() (msg tea.Msg) {
		// Recover from any panic in yt-dlp / ytresolve so the TUI
		// doesn't crash — the caller sees a friendly error instead.
		defer func() {
			if r := recover(); r != nil {
				msg = URLResolvedMsg{
					Error:   fmt.Errorf("resolve panic: %v", r),
					Action:  pr.Action,
					TrackID: pr.TrackID,
					Title:   pr.Title,
					Track:   pr.Track,
				}
			}
		}()
		url, err := ytresolve.ResolveURL(artist, title)
		return URLResolvedMsg{
			URL:      url,
			Error:    err,
			Action:   pr.Action,
			TrackID:  pr.TrackID,
			Title:    pr.Title,
			Uploader: pr.Uploader,
			CoverURL: pr.CoverURL,
			Track:    pr.Track,
		}
	}
}

// runUpdateCmd runs the install script via tea.ExecProcess so the user sees
// curl's progress bar and install output in real time.
func runUpdateCmd() tea.Cmd {
	install := exec.Command("bash", "-c", "curl -fsSL https://anas1412.github.io/ytmgo/install.sh | bash")
	return tea.ExecProcess(install, func(err error) tea.Msg {
		if err != nil {
			return UpdateResultMsg{Error: fmt.Errorf("update failed: %w", err)}
		}
		return UpdateResultMsg{}
	})
}
