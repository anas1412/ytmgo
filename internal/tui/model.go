package tui

import (
	"image"
	"os"
	"time"

	"ytmgo/internal/db"
	"ytmgo/internal/discordrpc"
	"ytmgo/internal/downloader"
	"ytmgo/internal/mpris"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/visualizer"
	"ytmgo/internal/ytmusic"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Page & Panel focus ────────────────────────────────────────────

// Page identifies which top-level page is shown.
type Page int

const (
	PageStream    Page = iota // 0 — search / recommendations / queue / player
	PageFavorites             // 1 — bookmarked tracks
	PageLibrary               // 2 — downloaded tracks + download queue
	PageHistory               // 3 — listening history
	PageSettings              // 4 — configuration
)

// Panel identifies which panel within a page has keyboard focus.
type Panel int

const (
	PanelSearch Panel = iota // left — search results / library
	PanelQueue               // right — queue / download queue
)

// ─── Messages (design stubs — backend integration later) ────────────

type (
	// PositionMsg carries mpv playback position updates (simulated with tick).
	PositionMsg struct {
		Position float64
		Duration float64
	}

	// SongEndedMsg fires when the current track finishes naturally.
	SongEndedMsg struct{}

	// DownloadProgressMsg reports status from the downloader worker.
	DownloadProgressMsg struct {
		TrackID  string
		Title    string            // carries through from the downloader Job so the
		Uploader string            // TUI can build a library entry on completion
		Progress float64           // 0–100
		Status   downloader.Status // StatusDone or StatusSkipped when Done
		Done     bool
		FilePath string // local path once downloaded
		Error    error
	}

	// URLResolvedMsg is sent when an async YouTube URL resolution completes.
	// The Action field tells the handler what to do with the resolved URL.
	URLResolvedMsg struct {
		URL      string
		Error    error
		TrackID  string
		Title    string
		Uploader string
		CoverURL string
		Action   string      // "play" or "download"
		Track    queue.Track // populated for play action
	}

	// URLPrefetchedMsg is sent when a background URL prefetch completes.
	// Unlike URLResolvedMsg (which triggers playback), this just populates
	// the in-memory + database URL caches so playback starts instantly
	// when the track is actually reached in the queue.
	URLPrefetchedMsg struct {
		TrackID string
		URL     string
	}

	// SearchResultsMsg carries results back from a song search.
	SearchResultsMsg struct {
		Results []search.Result
		Error   error
	}

	// RecommendationsMsg carries the list of recommended tracks.
	RecommendationsMsg struct {
		Results []search.Result
		Error   error
		Seq     int // generation counter; stale responses are skipped
	}

	// AlbumResultsMsg carries results back from an album search.
	AlbumResultsMsg struct {
		Albums []ytmusic.Album
		Error  error
	}

	// AlbumTracksMsg carries a fetched album's tracklist.
	AlbumTracksMsg struct {
		Album  ytmusic.Album
		Tracks []search.Result
		Error  error
	}

	// LibraryScanMsg carries the list of downloaded tracks found on disk.
	LibraryScanMsg struct {
		Tracks []queue.Track
	}

	// SettingsSavedMsg is sent after settings are persisted to disk.
	SettingsSavedMsg struct {
		Error error
	}

	// UpdateCheckMsg carries the latest version from GitHub.
	UpdateCheckMsg struct {
		LatestVersion string // empty when check was skipped/failed
	}

	// UpdateResultMsg is sent after the install script finishes running.
	UpdateResultMsg struct {
		Error error
	}

	// QuoteMsg carries a random quote fetched from the API.
	QuoteMsg struct {
		Quote  string
		Author string
		Seq    int // generation counter; stale responses are skipped
	}

	// DbReadyMsg is sent after queue and favorites are loaded from the DB.
	DbReadyMsg struct {
		QueueTracks []queue.Track
		Shuffle     bool
		Repeat      bool
		RepeatAll   bool
		Favorites   []queue.Track
		Error       error
	}

	// PlayRecordedMsg is sent after a play history entry is recorded.
	PlayRecordedMsg struct {
		Error error
	}

	// AutoplayResultsMsg is sent when autoplay fetches recommendations
	// after the queue has been exhausted.
	AutoplayResultsMsg struct {
		Tracks []queue.Track
	}

	// MprisReadyMsg delivers the connected MPRIS service.
	MprisReadyMsg struct {
		Svc *mpris.Service
	}

	// MprisCmdMsg is an external control request (media keys, playerctl).
	MprisCmdMsg struct {
		Cmd mpris.Command
	}

	// VizFrameMsg carries one spectrum frame from cava.
	VizFrameMsg struct {
		Frame visualizer.Frame
	}

	// VizStoppedMsg fires when cava exits on its own, so the UI can
	// switch back rather than freeze on the last frame.
	VizStoppedMsg struct {
		Err error
	}

	// CoverLoadedMsg carries a decoded cover image (or why it failed).
	CoverLoadedMsg struct {
		URL string
		Img image.Image
		Err error
	}
)

// tickMsg triggers periodic UI updates (progress bar animation).
type tickMsg struct{}

// playerTickMsg fires at 50ms while a track is playing, purely to
// trigger a redraw so the smooth-progress interpolation is visible.
// The model does nothing with it — only View reads time.Now() against
// lastPositionAt to render a gliding bar. Stops firing when paused.
type playerTickMsg struct{}

// pendingDownloadReserve stores the context needed to continue after an
// async YouTube URL resolution completes. Set by callers before firing
// resolveURLCmd, read by the URLResolvedMsg handler.
type pendingDownloadResolve struct {
	TrackID     string
	Title       string
	Uploader    string
	CoverURL    string
	DownloadDir string
	Track       queue.Track // populated for play action
	Action      string      // "play" or "download"
}

// ─── Model ──────────────────────────────────────────────────────────

// Model is the root Bubble Tea model for the ytmgo TUI.
type Model struct {
	// ── Window ──
	width  int
	height int
	ready  bool // true after first WindowSizeMsg

	// ── Page Navigation ──
	activePage  Page
	activePanel Panel
	quitting    bool

	// ── Confirmation (for destructive actions) ──
	confirmAction string // "" = none, "clear-queue", "delete-track"
	confirmData   string // context for the confirm message (e.g. track title)

	// ── Search ──
	searchInput     textinput.Model
	searchFocused   bool
	searchCursor    int
	searchOffset    int
	results         []search.Result
	recommendations []search.Result // last fetched batch; restored when the search is cleared
	isSearching     bool

	// ── Albums (Stream page, toggled with A) ──
	// The left panel shows exactly one list at a time, so albums reuse
	// searchCursor/searchOffset rather than carrying their own.
	albumMode              bool            // search returns albums instead of songs
	albums                 []ytmusic.Album // album search results (cached across A toggles)
	albumQuery             string          // query behind m.albums, so toggling back doesn't refetch
	openAlbum              *ytmusic.Album  // non-nil: showing this album's tracks
	albumTracks            []search.Result // tracks of openAlbum, as playable results
	isLoadingAlbum         bool
	showingRecommendations bool
	recsSeq                int    // bumped each time R is pressed or a search starts
	updateAvailable        string // "" = unknown, "latest" = up to date, "v0.X.Y" = update
	updateCheckManual      bool   // true when U was pressed to trigger the check

	// ── Library (local downloaded files) ──
	library       []queue.Track
	libraryCursor int
	libraryOffset int
	libraryLoaded bool // true after the first directory scan completes

	// ── Favorites (bookmarked tracks) ──
	favorites   []queue.Track
	favCursor   int
	favOffset   int
	favoriteSet map[string]bool // track ID → true, for O(1) lookup

	// ── History (listening history) ──
	history       []db.PlayHistoryEntry
	historyCursor int
	historyOffset int
	historyLoaded bool // true after the first history load completes

	// ── Database ──
	db *db.DB

	// ── Queue ──
	queue       *queue.Queue
	queueCursor int
	queueOffset int

	// ── Player ──
	player      *player.Player
	playerState player.State
	position    float64 // seconds
	duration    float64 // seconds
	volume      int

	// Smooth progress interpolation: store the last position from the
	// player and when it arrived, so the view can glide the bar between
	// coarse IPC updates instead of jumping.
	lastPosition   float64
	lastPositionAt time.Time

	// Mode-toggle flash: for a short window after the user presses `s` or
	// `r`, the SHFL / REPT labels render in a brighter style so the
	// keypress feels acknowledged. Decays naturally as time passes.
	// modeFlashUntil and modeFlashTarget coordinate the brief bright
	// flash on the mode label (SHFL or REPT) after pressing `s`/`r`.
	// Only the label matching modeFlashTarget lights up — the other
	// stays at its normal active/inactive style.
	modeFlashUntil  time.Time
	modeFlashTarget string // "shuffle", "repeat", or ""
	// Channel-listener guards. positionCmd/endedCmd/playerTickCmd each
	// keep exactly one listener alive on the persistent player's
	// channels; without these flags every Play would stack another
	// listener. positionListening/endedListening arm once and stay armed
	// (their handlers re-arm themselves); playerTicking drops when the
	// tick loop dies on pause/stop and re-arms on the next play/resume.
	positionListening bool
	endedListening    bool
	playerTicking     bool

	// autoplayFired prevents duplicate autoplay fetches while one
	// is already in-flight. Set true when fetchAutoplayCmd fires,
	// reset false when handleAutoplayResults processes the response
	// (both success and failure). The manual resets in keyboard.go
	// and mouse.go serve as a safety valve when a fetch fails
	// silently (returned nil — no AutoplayResultsMsg sent).
	autoplayFired bool

	// ── Mouse double-click tracking ──
	lastClickAt    time.Time
	lastClickY     int
	lastClickPanel Panel

	// ── Downloads ──
	downloader *downloader.Downloader

	// ── MPRIS (media keys / desktop integration) ──
	mpris *mpris.Service

	// ── Now-playing panel (v) ──
	// One sub-panel beneath the results list, split left/right: album
	// art beside the spectrum. Mirrors the queue/downloads split on the
	// right, so the results list stays visible while it is open.
	npOn         bool
	viz          *visualizer.Visualizer
	vizFrame     visualizer.Frame
	coverImg     image.Image
	coverURL     string // URL currently decoded into coverImg
	coverLoading bool
	coverErr     string
	// Bubble Tea calls View more often than it flushes, and only the
	// last frame before a flush is written. Emitting the kitty transmit
	// or delete from inside View therefore loses it whenever that frame
	// is discarded — which left the artwork stuck on the first track and
	// still on screen after closing. The decision lives in Update
	// instead, and each escape is repeated over a few frames so a
	// dropped one cannot swallow it.
	coverSendN  int // frames still carrying the image transmit
	coverClearN int // frames still carrying the image delete

	// ── Async URL resolution ──
	// pendingResolve stores the context of an in-flight YouTube URL
	// resolution. Set before returning resolveURLCmd, cleared when the
	// URLResolvedMsg handler runs. Only one resolve can be pending at
	// a time — the most recent one wins.
	pendingResolve *pendingDownloadResolve

	// resolvedURLs caches resolved YouTube URLs (track ID → URL) so
	// repeated plays of the same track skip the yt-dlp call. Populated
	// by handleURLResolved / handleURLPrefetched and seeded from the
	// SQLite url_cache table on first access.
	resolvedURLs map[string]string

	// ── Settings ──
	settings          *settings.Settings
	settingsCursor    int
	settingsOffset    int
	settingsEditField bool
	settingsEditInput textinput.Model

	// ── Status ──
	statusMessage      string
	statusMessageSetAt time.Time
	err                error

	// ── Quote/tip rotation (shown in status bar when idle) ──
	currentQuote string
	fallbackIdx  int
	quoteSeq     int // bumped each rotation; stale API responses dropped
	tipIndex     int // used when ShowQuotes is off (classic tips)
	tickCount    int // counts ticks between rotations
}

// ─── Status helpers ─────────────────────────────────────────────────

// setStatus records a status message and starts the auto-clear timer.
// Passing "" is equivalent to clearStatus — the timer is reset so no
// auto-clear fires on the next tick.
func (m *Model) setStatus(msg string) {
	m.statusMessage = msg
	if msg == "" {
		m.statusMessageSetAt = time.Time{}
	} else {
		m.statusMessageSetAt = time.Now()
	}
}

// clearStatus immediately clears the status message and its timer.
func (m *Model) clearStatus() {
	m.statusMessage = ""
	m.statusMessageSetAt = time.Time{}
}

// ─── Initial model ──────────────────────────────────────────────────

// InitialModel returns a Model with empty state — all data comes from
// real backend calls (search, download, mpv).
func InitialModel() Model {
	ti := textinput.New()
	ti.Placeholder = "Search"
	ti.PromptStyle = textinputStyle
	ti.TextStyle = textinputStyle
	ti.PlaceholderStyle = textinputPlaceholder
	ti.CharLimit = 80
	ti.Width = 40

	sti := textinput.New()
	sti.Placeholder = ""
	sti.PromptStyle = textinputStyle
	sti.TextStyle = textinputStyle
	sti.PlaceholderStyle = textinputPlaceholder
	sti.CharLimit = 200
	sti.Width = 40

	// Open the database synchronously so settings are available immediately.
	// Queue + favorites are loaded asynchronously via initQueueFavoritesCmd.
	var database *db.DB
	defSettings := settings.Defaults()
	database, err := db.Open()
	if err == nil {
		if s, err := database.LoadSettings(); err == nil {
			defSettings = s
		}
	}
	if defSettings.DefaultVolume < 1 {
		defSettings.DefaultVolume = 80
	}
	vol := defSettings.DefaultVolume

	return Model{
		activePage:             PageStream,
		activePanel:            PanelSearch,
		searchInput:            ti,
		results:                []search.Result{},
		favorites:              []queue.Track{},
		favoriteSet:            map[string]bool{},
		history:                []db.PlayHistoryEntry{},
		queue:                  queue.New(),
		playerState:            player.StateStopped,
		volume:                 vol,
		showingRecommendations: true,
		settings:               defSettings,
		settingsEditInput:      sti,
		currentQuote:           fallbackQuotes[0],
		db:                     database,
		resolvedURLs:           map[string]string{},
	}
}

// startTrackPlayback is the single source of truth for launching a new
// playback session. It centralises the model setup, calls Player.Play,
// and — critically — mirrors the player's authoritative state back to
// the model on success. This avoids the optimistic `m.playerState =
// player.StatePlaying` write that the old call sites used, which could
// drift from what the player actually does (causing the play/pause icon
// to stay stale until the user pressed Space to force a re-sync).
//
// Returns the tea.Cmd to attach (position + ended + play history + queue save)
// on success, or nil on failure. Callers can combine this with their own
// commands (e.g. downloadCmd) using tea.Batch.
func (m *Model) startTrackPlayback(playURL string, t queue.Track) tea.Cmd {
	m.duration = float64(t.DurationSec)
	m.position = 0
	m.setStatus("Now playing: " + t.Title)
	// Seed the smooth-progress anchor at zero so the bar starts
	// gliding from the correct origin on the first render.
	m.lastPosition = 0
	m.lastPositionAt = time.Now()
	m.ensurePlayer()
	if err := m.player.Play(playURL); err != nil {
		m.err = err
		m.playerState = player.StateStopped
		return nil
	}
	// Mirror the player's state — it is the single source of truth.
	m.playerState = m.player.State()
	m.updatePresence()
	// playerTickCmd drives the 50ms redraws that make the progress
	// bar glide instead of jumping. It self-perpetuates from within
	// Update as long as playerState == StatePlaying.
	// recordPlayCmd logs this play in the database silently.
	var cmds []tea.Cmd
	if !m.positionListening {
		m.positionListening = true
		cmds = append(cmds, positionCmd(m.player))
	}
	if !m.endedListening {
		m.endedListening = true
		cmds = append(cmds, endedCmd(m.player))
	}
	if !m.playerTicking {
		m.playerTicking = true
		cmds = append(cmds, playerTickCmd())
	}
	if m.db != nil {
		cmds = append(cmds, recordPlayCmd(m.db, t))
	}
	// Keep the cover panel in step with what is playing.
	if cmd := m.refreshCoverCmd(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// Prefetch the URL for the next track in the queue so playback
	// of that track starts instantly (no "Fetching URL…" delay).
	if next, ok := m.queue.PeekNext(); ok {
		if cmd := m.prefetchCmd(next); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// resolveAndPlayCmd resolves the playback URL for a track and starts
// playback. If the track is already downloaded (local file exists), it
// plays directly. Otherwise it checks the in-memory URL cache, then the
// database cache, and only falls back to an async yt-dlp resolve if
// neither cache has the URL.
//
// Returns nil if the track cannot be played (empty queue, etc.).
func (m *Model) resolveAndPlayCmd(t queue.Track) tea.Cmd {
	if t.Downloaded && t.FilePath != "" {
		if _, err := os.Stat(t.FilePath); err == nil {
			m.pendingResolve = nil
			return m.startTrackPlayback(t.FilePath, t)
		}
	}

	// Direct URL known (YouTube Music results carry the exact watch
	// URL): play immediately, nothing to resolve.
	if t.URL != "" {
		m.pendingResolve = nil
		return m.startTrackPlayback(t.URL, t)
	}

	// Check in-memory URL cache.
	if url, ok := m.resolvedURLs[t.ID]; ok && url != "" {
		m.pendingResolve = nil
		return m.startTrackPlayback(url, t)
	}

	// Check database cache (seeds the in-memory cache for this session).
	if m.db != nil {
		url, err := m.db.LoadCachedURL(t.ID)
		if err == nil && url != "" {
			m.resolvedURLs[t.ID] = url
			m.pendingResolve = nil
			return m.startTrackPlayback(url, t)
		}
	}

	// No cache hit — resolve the YouTube URL asynchronously.
	m.pendingResolve = &pendingDownloadResolve{
		TrackID: t.ID,
		Track:   t,
		Title:   t.Title,
		Action:  "play",
	}
	m.setStatus("Fetching URL…")
	return resolveURLCmd(t.Artist, t.Title, m.pendingResolve)
}

// prefetchCmd returns a command that resolves the YouTube URL for the
// given track and caches it, or nil if no resolution is needed (track is
// already downloaded, URL already cached, or yt-dlp would be wasted).
func (m *Model) prefetchCmd(t queue.Track) tea.Cmd {
	if t.Downloaded && t.FilePath != "" {
		// Track has a local file — no URL needed.
		return nil
	}
	if t.URL != "" {
		// Direct watch URL already known — nothing to resolve.
		return nil
	}
	// Check in-memory cache.
	if _, ok := m.resolvedURLs[t.ID]; ok {
		return nil
	}
	// Check database cache and populate in-memory cache.
	if m.db != nil {
		url, err := m.db.LoadCachedURL(t.ID)
		if err == nil && url != "" {
			m.resolvedURLs[t.ID] = url
			return nil
		}
	}
	return prefetchURLCmd(t.ID, t.Artist, t.Title)
}

// streamList reports what the Stream page's left panel is showing and
// how many rows it has. Albums reuse searchCursor, so every cursor
// bound and click needs the active list's length rather than
// len(m.results).
func (m Model) streamListLen() int {
	switch {
	case m.openAlbum != nil:
		return len(m.albumTracks)
	case m.albumMode:
		return len(m.albums)
	default:
		return len(m.results)
	}
}

// resetStreamCursor puts the left panel back at the top, used whenever
// the list it shows is swapped out.
func (m *Model) resetStreamCursor() {
	m.searchCursor = 0
	m.searchOffset = 0
}

// showRecommendations restores the recommendations list in the results
// panel: instantly from the cached batch when one exists, otherwise by
// fetching a fresh one. Used when the user clears the search.
func (m *Model) showRecommendations() tea.Cmd {
	m.showingRecommendations = true
	m.searchCursor = 0
	m.searchOffset = 0
	m.isSearching = false
	m.err = nil
	if len(m.recommendations) > 0 {
		m.results = m.recommendations
		return nil
	}
	m.recsSeq++
	m.results = nil
	m.setStatus("Loading recommendations…")
	return fetchRecommendationsCmd(m.recsSeq, m.settings.SearchLimit, m.db)
}

// setVolumeTo sets the playback volume to an absolute value and
// persists it as the default, so the level survives restarts.
func (m *Model) setVolumeTo(v int) tea.Cmd {
	m.volume = min(100, max(0, v))
	if m.player != nil {
		m.player.SetVolume(m.volume)
	}
	m.settings.DefaultVolume = m.volume
	m.updateMPRIS()
	return saveSettingsCmd(m.db, m.settings)
}

// changeVolume adjusts playback volume by delta (see setVolumeTo).
func (m *Model) changeVolume(delta int) tea.Cmd {
	return m.setVolumeTo(m.volume + delta)
}

// updatePresence syncs playback state to every external surface:
// Discord Rich Presence and the MPRIS D-Bus endpoint.
func (m *Model) updatePresence() {
	m.updateDiscordRPC()
	m.updateMPRIS()
}

// updateMPRIS publishes the current playback state to the MPRIS
// service (no-op until the service is up).
func (m *Model) updateMPRIS() {
	if m.mpris == nil {
		return
	}
	snap := mpris.Snapshot{
		Position: m.position,
		Duration: m.duration,
		Volume:   m.volume,
		Shuffle:  m.queue.IsShuffle(),
		LoopOne:  m.queue.IsRepeat(),
		LoopAll:  m.queue.IsRepeatAll(),
	}
	if t, ok := m.queue.Current(); ok && m.playerState != player.StateStopped {
		snap.TrackID = t.ID
		snap.Title = t.Title
		snap.Artist = t.Artist
		snap.CoverURL = t.CoverURL
		if snap.Duration == 0 {
			snap.Duration = float64(t.DurationSec)
		}
		snap.Playing = m.playerState == player.StatePlaying
		snap.Paused = m.playerState == player.StatePaused
	}
	m.mpris.Publish(snap)
}

// updateDiscordRPC syncs the current playback state to Discord Rich
// Presence. Shows the idle state when nothing is playing, track info
// when a track is active. No-op when the feature is disabled.
func (m *Model) updateDiscordRPC() {
	if !m.settings.DiscordRPCEnabled {
		discordrpc.Close()
		return
	}
	t, ok := m.queue.Current()
	if !ok || m.playerState == player.StateStopped || t.Title == "" {
		discordrpc.ShowIdle()
		return
	}
	discordrpc.Update(t, m.playerState, m.position)
}

// refreshCoverCmd loads the current track's art when the cover panel is
// showing and the art on screen belongs to a different track. Returns
// nil when there is nothing to do.
func (m *Model) refreshCoverCmd() tea.Cmd {
	if !m.npOn {
		return nil
	}
	t, ok := m.queue.Current()
	if !ok || t.CoverURL == "" {
		m.coverImg = nil
		m.coverURL = ""
		return nil
	}
	if t.CoverURL == m.coverURL || (m.coverLoading && t.CoverURL == m.coverURL) {
		return nil
	}
	m.coverLoading = true
	m.coverErr = ""
	return loadCoverCmd(t.CoverURL)
}

// reinitDiscordRPC tears down and re-initialises the Discord RPC
// connection. Called when the user toggles the feature in Settings.
func (m *Model) reinitDiscordRPC() {
	discordrpc.Close()
	if m.settings.DiscordRPCEnabled {
		discordrpc.Init()
		m.updatePresence()
	}
}

// discordRPCInitCmd returns a tea.Cmd that initialises Discord RPC in
// the background so the TUI starts rendering immediately. Logs in and
// sets idle presence; errors are silently ignored.
func discordRPCInitCmd(enabled bool) tea.Cmd {
	if !enabled {
		return nil
	}
	return func() tea.Msg {
		if err := discordrpc.Init(); err != nil {
			return nil
		}
		discordrpc.ShowIdle()
		return nil
	}
}

// ─── Fallback quotes (used when API fetch fails) ─────────────────────
// Rotated through as a fallback whenever the internet quote fetch fails.
var fallbackQuotes = []string{
	`"Music is the shorthand of emotion" — Leo Tolstoy`,
	`"Without music, life would be a mistake" — Friedrich Nietzsche`,
	`"One good thing about music, when it hits you, you feel no pain" — Bob Marley`,
	`"Music can change the world" — Beethoven`,
	`"Where words fail, music speaks" — Hans Christian Andersen`,
	`"Life is like jazz — best when you improvise" — George Gershwin`,
	`"Music is the universal language of mankind" — H. W. Longfellow`,
	`"The only truth is music" — Jack Kerouac`,
	`"After silence, that which comes nearest to expressing the inexpressible is music" — Aldous Huxley`,
	`"Music gives a soul to the universe, wings to the mind" — Plato`,
	`"If music be the food of love, play on" — Shakespeare`,
	`"Everything in the universe has rhythm" — unknown`,
	`"Let the music play" — unknown`,
	`"When in doubt, turn up the volume" — unknown`,
	`"Music is what feelings sound like" — unknown`,
}

// quoteRotateEvery is how many 500ms ticks between quote rotations.
// 60 ticks = 30 seconds — slow enough to read a quote.
const quoteRotateEvery = 60

// ─── Classic tips (shown when ShowQuotes is off) ─────────────────────

var idleTips = []string{
	"Press `?` for all keyboard shortcuts",
	"`Tab` cycles focus · `o` opens the download folder",
	"Press `R` for fresh recommendations",
	"`D` then `Enter` clears the entire queue",
	"`1`-`5` switch between Stream · Favs · Library · History · Settings",
	"`↑↓` or `j`/`k` to navigate · `g`/`G` jump to top/bottom",
	"`space` toggles play / pause",
	"`ctrl+↑` / `ctrl+↓` to reorder the queue",
	"`s` toggles shuffle · `r` cycles repeat",
	"Stream mode plays without downloading — toggle in Settings",
	"Press `x` on any track to download it for offline use",
	"Already have MP3s? Point Download Dir at them in Settings",
	"Media keys work too — ytmgo speaks MPRIS on Linux",
}

// idleTipRotateEvery is how many 500ms ticks between tip rotations.
// 16 ticks = 8 seconds.
const idleTipRotateEvery = 16

// currentTip returns the tip to show right now.
func (m Model) currentTip() string {
	tip := idleTips[m.tipIndex%len(idleTips)]
	return tip
}

// advanceTip moves to the next tip in the rotation.
func (m *Model) advanceTip() {
	m.tipIndex++
	if m.tipIndex >= len(idleTips) {
		m.tipIndex = 0
	}
	m.tickCount = 0
}

// Shutdown cleans up background processes. Call on program exit.
func (m Model) Shutdown() {
	if m.player != nil {
		m.player.Shutdown()
	}
	if m.downloader != nil {
		m.downloader.Close()
	}
	discordrpc.Close()
	m.mpris.Close()
	m.viz.Close()
	if m.db != nil {
		m.db.Close()
	}
}
