package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/library"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Page & Panel focus ────────────────────────────────────────────

// Page identifies which top-level page is shown.
type Page int

const (
	PageStream   Page = iota // search / recommendations / queue / player
	PageLibrary              // downloaded tracks + download queue
	PageSettings             // configuration
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
		Title    string // carries through from the downloader Job so the
		Uploader string // TUI can build a library entry on completion
		Progress float64 // 0–100
		Status   downloader.Status // StatusDone or StatusSkipped when Done
		Done     bool
		FilePath string // local path once downloaded
		Error    error
	}

	// SearchResultsMsg carries results back from a yt-dlp search.
	SearchResultsMsg struct {
		Results []search.Result
		Error   error
	}

	// RecommendationsMsg carries the list of recommended tracks.
	RecommendationsMsg struct {
		Results []search.Result
		Error   error
	}

	// LibraryScanMsg carries the list of downloaded tracks found on disk.
	LibraryScanMsg struct {
		Tracks []queue.Track
	}

	// SettingsSavedMsg is sent after settings are persisted to disk.
	SettingsSavedMsg struct {
		Error error
	}
)

// tickMsg triggers periodic UI updates (progress bar animation).
type tickMsg struct{}

// playerTickMsg fires at 50ms while a track is playing, purely to
// trigger a redraw so the smooth-progress interpolation is visible.
// The model does nothing with it — only View reads time.Now() against
// lastPositionAt to render a gliding bar. Stops firing when paused.
type playerTickMsg struct{}

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
	searchInput            textinput.Model
	searchFocused          bool
	searchCursor           int
	searchOffset           int
	results                []search.Result
	isSearching            bool
	showingRecommendations bool

	// ── Library (local downloaded files) ──
	library       []queue.Track
	libraryCursor int
	libraryOffset int

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
	modeFlashUntil time.Time
	// suppressAutoAdvance prevents the SongEnded handler from calling
	// Next() when the old mpv was intentionally killed by a new
	// Play() call in playSelectedQueueItem. Without this, the stale
	// endedCmd from the previous playback fires a SongEndedMsg that
	// advances past the track the user just selected.
	suppressAutoAdvance bool

	// ── Downloads ──
	downloader *downloader.Downloader

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

	// ── Idle tip rotation (status bar shows tips when nothing else is happening) ──
	tipIndex  int
	tickCount int
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

	defSettings, _ := settings.Load()
	if defSettings.DefaultVolume < 1 {
		defSettings.DefaultVolume = 80
	}
	vol := defSettings.DefaultVolume

	return Model{
		activePage:             PageStream,
		activePanel:            PanelSearch,
		searchInput:            ti,
		results:                []search.Result{},
		queue:                  queue.New(),
		playerState:            player.StateStopped,
		volume:                 vol,
		showingRecommendations: true,
		settings:               defSettings,
		settingsEditInput:      sti,
	}
}

// ─── Commands ────────────────────────────────────────────────────────

// searchCmd fires a yt-dlp search in a goroutine and sends results back.
func searchCmd(query string, limit int, cookieBrowser, userAgent string) tea.Cmd {
	return func() tea.Msg {
		results, err := search.Search(query, limit, cookieBrowser, userAgent)
		if err != nil {
			return SearchResultsMsg{Error: err}
		}
		if results == nil {
			results = []search.Result{} // never nil
		}
		return SearchResultsMsg{Results: results}
	}
}

// fetchRecommendationsCmd fires a request for YouTube home page recommendations.
func fetchRecommendationsCmd(cookieBrowser, userAgent string) tea.Cmd {
	return func() tea.Msg {
		results, err := search.FetchRecommendations(40, cookieBrowser, userAgent)
		if err != nil {
			return RecommendationsMsg{Error: err}
		}
		if results == nil {
			results = []search.Result{}
		}
		return RecommendationsMsg{Results: results}
	}
}

// scanLibraryCmd scans the downloads directory for existing audio files.
func scanLibraryCmd(dir string) tea.Cmd {
	return func() tea.Msg {
		tracks, err := library.ScanDir(dir)
		if err != nil {
			// Non-fatal — just return empty library
			return LibraryScanMsg{Tracks: []queue.Track{}}
		}
		return LibraryScanMsg{Tracks: tracks}
	}
}

// positionCmd reads one position update from the mpv IPC poller.
func positionCmd(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		pos, ok := <-p.Positions()
		if !ok {
			return nil
		}
		return PositionMsg{Position: pos.Position, Duration: pos.Duration}
	}
}

// endedCmd waits for mpv to finish playing the current track.
func endedCmd(p *player.Player) tea.Cmd {
	return func() tea.Msg {
		<-p.Ended()
		return SongEndedMsg{}
	}
}

// ensurePlayer creates the player if it doesn't exist yet.
func (m *Model) ensurePlayer() {
	if m.player == nil {
		m.player = player.New()
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
// Returns the tea.Cmd to attach (position + ended listeners) on success,
// or nil on failure. Callers can combine this with their own commands
// (e.g. downloadCmd) using tea.Batch.
func (m *Model) startTrackPlayback(playURL, title string, durationSec int) tea.Cmd {
	m.duration = float64(durationSec)
	m.position = 0
	m.setStatus("Now playing: " + title)
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
	// playerTickCmd drives the 50ms redraws that make the progress
	// bar glide instead of jumping. It self-perpetuates from within
	// Update as long as playerState == StatePlaying.
	return tea.Batch(positionCmd(m.player), endedCmd(m.player), playerTickCmd())
}

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

const progressTickInterval = time.Second / 2

// playerTickInterval drives the smooth progress interpolation in the
// player bar. 50ms = 20fps, which is the lowest rate at which motion
// reads as continuous on a terminal.
const playerTickInterval = 50 * time.Millisecond

// idleTips are short hints shown in the status bar when nothing else is happening.
// Mix of keyboard shortcuts, feature discoverability, and personality. Rotates
// every tipRotateEvery ticks (8 seconds at 500ms tick).
var idleTips = []string{
	// Keyboard / shortcuts
	"Press `?` for all keyboard shortcuts",
	"`Tab` cycles focus · `o` opens the download folder",
	"Press `R` for fresh recommendations",
	"Press `D` twice to clear the entire queue",
	"`1` `2` `3` jump between Stream · Library · Settings",
	"`↑↓` or `j`/`k` to navigate lists",
	"`space` toggles play / pause",
	"`ctrl+↑` / `ctrl+↓` to reorder the queue",
	"`s` toggles shuffle · `r` cycles repeat",

	// Features
	"Stream mode plays without downloading — toggle in Settings",
	"Press `x` on any track to download it for offline use",
	"Queue + Downloads are always visible on the right →",
	"Already have MP3s? Point Download Dir at them in Settings",
	"Set Default Volume in Settings so every track starts at your level",
	"Use a cookie browser in Settings for age-restricted tracks",

	// State-aware (formatted each tick)
	"__SESSIONS__", // placeholder — replaced at render time with session stats
}

// idleTipRotateEvery is how many 500ms ticks between tip rotations.
// 16 ticks = 8 seconds.
const idleTipRotateEvery = 16

// currentTip returns the tip to show right now. Placeholders are resolved
// against current model state (queue length, downloads tracked, etc.).
func (m Model) currentTip() string {
	tip := idleTips[m.tipIndex%len(idleTips)]
	if tip == "__SESSIONS__" {
		queue := m.queue.Len()
		dlCount := 0
		if m.downloader != nil {
			dlCount = len(m.downloader.Jobs())
		}
		if queue == 0 && dlCount == 0 {
			return "Tip: search for an artist to get started"
		}
		return fmt.Sprintf("Session: %d in queue · %d downloads tracked", queue, dlCount)
	}
	return tip
}

// advanceTip moves to the next tip in the rotation. Returns the new index.
func (m *Model) advanceTip() {
	m.tipIndex++
	if m.tipIndex >= len(idleTips) {
		m.tipIndex = 0
	}
	m.tickCount = 0
}

// searchResultToTrack converts a search result to a queue track.
func searchResultToTrack(r search.Result) queue.Track {
	d := r.Duration
	m := d / 60
	s := d % 60
	return queue.Track{
		ID:          r.ID,
		Title:       r.Title,
		Artist:      r.Uploader,
		Duration:    fmt.Sprintf("%d:%02d", m, s),
		DurationSec: d,
		URL:         r.URL,
	}
}

// ─── Library matching ───────────────────────────────────────────────
//
// Search results and library entries describe the same song with
// different field shapes:
//
//	search:  ID=YouTube video ID  | Title=raw (e.g. "Song (Official Video)")
//	                                Artist=uploader (e.g. "Channel - Topic")
//	library: ID=file path         | Title=cleaned (e.g. "Song")
//	                                Artist=filename prefix (e.g. "Channel")
//
// YouTube IDs and file paths can never match, so library lookup is
// done via a normalized "artist|title" signature that strips the
// common YouTube decorations (" - Topic" channel suffix, "(Official
// Video)" title suffix, etc.) before comparing.

// normalizeForMatch lower-cases, trims, and strips the YouTube-specific
// decorations that differ between a search result and a library entry
// even when they refer to the same track.
func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// YouTube auto-generated channels append " - Topic" to the channel
	// name. The library filename parser doesn't capture this suffix.
	s = strings.TrimSuffix(s, " - topic")
	// Common title decorations that the library filename parser strips
	// (see library.cleanTitle) but search results preserve.
	for _, suf := range []string{
		"(official music video)", "(official video)", "(official lyric video)",
		"(lyric video)", "(lyrics)", "(audio)", "(official audio)",
		"[official music video]", "[official video]", "[lyrics]",
	} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimSpace(s)
}

// trackMatchKey returns the signature used for library lookups.
// Format: "artist|title" with both fields normalized.
func trackMatchKey(t queue.Track) string {
	return normalizeForMatch(t.Artist) + "|" + normalizeForMatch(t.Title)
}

// findLibraryMatch searches the local library for a track whose
// signature matches t. Returns the matching library track and true,
// or a zero Track and false. Library entries with empty FilePath are
// skipped (they aren't actually playable).
func findLibraryMatch(library []queue.Track, t queue.Track) (queue.Track, bool) {
	sig := trackMatchKey(t)
	for i := range library {
		lt := library[i]
		if lt.FilePath == "" {
			continue
		}
		if trackMatchKey(lt) == sig {
			return lt, true
		}
	}
	return queue.Track{}, false
}

// resolveTrack converts a search result to a queue Track, consulting
// the local library so that an already-downloaded file is preferred
// over re-streaming from YouTube. The returned track has Downloaded
// and FilePath set when a library match exists, and the play sites
// (playSelectedQueueItem, SongEnded auto-advance, startTrackPlayback)
// all check those fields first.
func (m *Model) resolveTrack(r search.Result) queue.Track {
	t := searchResultToTrack(r)
	if lib, ok := findLibraryMatch(m.library, t); ok {
		t.Downloaded = true
		t.FilePath = lib.FilePath
	}
	return t
}

// backfillQueueFromLibrary walks the current queue and back-fills
// FilePath/Downloaded for any track that was added from search before
// the library scan completed (i.e. when resolveTrack saw an empty
// library). Call this from the LibraryScanMsg handler so the
// play-locally-first behavior is consistent regardless of timing.
func (m *Model) backfillQueueFromLibrary() {
	if m.queue == nil || len(m.library) == 0 {
		return
	}
	// Tracks() returns a copy so we can iterate safely while the
	// queue continues to be used. For each unresolved track whose
	// signature matches a library entry, patch the in-queue track
	// in place via the queue's matching-by-key update helper.
	for _, t := range m.queue.Tracks() {
		if t.Downloaded && t.FilePath != "" {
			continue
		}
		lib, ok := findLibraryMatch(m.library, t)
		if !ok {
			continue
		}
		fp := lib.FilePath
		sig := trackMatchKey(t)
		m.queue.UpdateTrackByMatch(sig, trackMatchKey, func(qt *queue.Track) {
			qt.Downloaded = true
			qt.FilePath = fp
		})
	}
}

// downloadDir returns the directory where downloaded tracks are stored.
//
// Resolution order:
//  1. If the user has set a custom path via the Settings page, use it.
//  2. Otherwise, fall back to the platform-appropriate user data directory
//     (XDG_DATA_HOME/ytmgo/downloads on Linux,
//     ~/Library/Application Support/ytmgo/downloads on macOS).
//
// The legacy default value "downloads" is treated as "unset" so existing
// users get the new XDG location instead of a stray "downloads" folder
// next to the binary after upgrading.
func (m *Model) downloadDir() string {
	if dir := m.settings.DownloadDir; dir != "" && dir != "downloads" {
		os.MkdirAll(dir, 0755)
		return dir
	}
	base, err := userDataDir()
	if err != nil {
		return "downloads" // last-ditch fallback
	}
	dir := filepath.Join(base, "ytmgo", "downloads")
	os.MkdirAll(dir, 0755)
	return dir
}

// userDataDir returns the platform-appropriate base directory for app data
// (NOT configuration — for that, see settings.configPath).
//   - Linux:   $XDG_DATA_HOME, or ~/.local/share if unset
//   - macOS:   ~/Library/Application Support
func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	return filepath.Join(home, ".local", "share"), nil
}

// openInOS opens the given path in the system's default file manager
// (xdg-open on Linux/BSD, open on macOS). Uses Start, not Run, so it
// returns immediately without waiting for the launched process to exit.
func openInOS(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

// ensureDownloader creates the downloader if it doesn't exist yet.
func (m *Model) ensureDownloader() {
	if m.downloader == nil {
		m.downloader = downloader.New(m.downloadDir(), m.settings.CookieBrowser, m.settings.UserAgent)
	}
}

// panelHeight returns how many terminal lines the panel area occupies.
// Total layout: header(1) + panels(h) + player(5) + status(1) + help(1).
// lipgloss Height(N) renders N+2 lines (border adds 2), so panels(h) actually
// consumes h+2 lines. To keep the total exactly m.height, we subtract 2.

func (m Model) panelHeight() int {
	// Fixed overhead: header(1) + status(1) + player(5) + help(1) + border(2) = 10
	overhead := 10
	h := m.height - overhead
	if h < 1 {
		h = 1
	}
	return h
}

// visibleItems returns how many list rows fit in the panel.
// Must stay in sync with renderSearchResults / renderLibrary / renderQueue
// which use (height - 1) / 2 where height = panelHeight - 2.
func (m Model) visibleItems() int {
	n := (m.panelHeight() - 3) / 2
	if n < 1 {
		n = 1
	}
	return n
}

// filteredLibrary returns library tracks that match the search input (case-insensitive).
// When the input is empty or not on the library page, returns all tracks.
func (m Model) filteredLibrary() []queue.Track {
	if m.activePage != PageLibrary {
		return m.library
	}
	q := m.searchInput.Value()
	if q == "" {
		return m.library
	}
	q = strings.ToLower(q)
	var out []queue.Track
	for _, t := range m.library {
		if strings.Contains(strings.ToLower(t.Title), q) || strings.Contains(strings.ToLower(t.Artist), q) {
			out = append(out, t)
		}
	}
	return out
}

// clampSearchOffset adjusts searchOffset so the cursor is visible.
func (m *Model) clampSearchOffset() {
	vis := m.visibleItems()
	if m.searchCursor < m.searchOffset {
		m.searchOffset = m.searchCursor
	}
	if m.searchCursor >= m.searchOffset+vis {
		m.searchOffset = m.searchCursor - vis + 1
	}
}

// clampLibraryOffset adjusts libraryOffset so the cursor is visible.
func (m *Model) clampLibraryOffset() {
	vis := m.visibleItems()
	n := len(m.filteredLibrary())
	if n == 0 {
		m.libraryCursor = 0
		m.libraryOffset = 0
		return
	}
	if m.libraryCursor >= n {
		m.libraryCursor = n - 1
	}
	if m.libraryCursor < 0 {
		m.libraryCursor = 0
	}
	if m.libraryCursor < m.libraryOffset {
		m.libraryOffset = m.libraryCursor
	}
	if m.libraryCursor >= m.libraryOffset+vis {
		m.libraryOffset = m.libraryCursor - vis + 1
	}
}

// clampQueueOffset adjusts queueOffset so the cursor is visible.
func (m *Model) clampQueueOffset() {
	vis := m.visibleItems()
	n := m.queue.Len()
	if n == 0 {
		m.queueCursor = 0
		m.queueOffset = 0
		return
	}
	if m.queueCursor >= n {
		m.queueCursor = n - 1
	}
	if m.queueCursor < 0 {
		m.queueCursor = 0
	}
	if m.queueCursor < m.queueOffset {
		m.queueOffset = m.queueCursor
	}
	if m.queueCursor >= m.queueOffset+vis {
		m.queueOffset = m.queueCursor - vis + 1
	}
}

// settingsVisibleItems returns how many settings items fit in the visible area.
// Uses the same panel-height calculation as renderSettingsList.
func (m Model) settingsVisibleItems() int {
	// Panel content height minus 2 lines of overhead (scroll indicator + help text),
	// divided by 4 lines per item.
	contentH := m.panelHeight() - 3
	vis := (contentH - 2) / 4
	if vis < 1 {
		return 1
	}
	return vis
}

// clampSettingsOffset adjusts settingsOffset so the cursor is visible.
func (m *Model) clampSettingsOffset() {
	vis := m.settingsVisibleItems()
	maxItem := 7 // 8 items indexed 0-7
	if m.settingsCursor > maxItem {
		m.settingsCursor = maxItem
	}
	if m.settingsCursor < 0 {
		m.settingsCursor = 0
	}
	if m.settingsCursor < m.settingsOffset {
		m.settingsOffset = m.settingsCursor
	}
	if m.settingsCursor >= m.settingsOffset+vis {
		m.settingsOffset = m.settingsCursor - vis + 1
	}
}

// switchPage transitions to a new page and resets page-local state.
func (m *Model) switchPage(page Page) {
	m.activePage = page
	m.searchFocused = false

	switch page {
	case PageStream:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = "Search"
		m.activePanel = PanelSearch
		m.searchCursor = 0
		m.searchOffset = 0
		m.settingsEditField = false
	case PageLibrary:
		m.searchInput.SetValue("")
		m.searchInput.Placeholder = "Filter library…"
		m.activePanel = PanelSearch
		m.libraryCursor = 0
		m.libraryOffset = 0
		m.settingsEditField = false
	case PageSettings:
		m.searchInput.Blur()
		m.activePanel = PanelSearch
		m.settingsCursor = 0
		m.settingsOffset = 0
		m.settingsEditField = false
	}
}

// startSettingsEdit prepares the inline text input for editing a string setting.
func (m *Model) startSettingsEdit() {
	m.settingsEditField = true
	current := ""
	switch m.settingsCursor {
	case 4:
		current = m.settings.DownloadDir
	case 5:
		current = m.settings.CookieBrowser
	case 6:
		current = m.settings.UserAgent
	}
	m.settingsEditInput.SetValue(current)
	m.settingsEditInput.Focus()
}

// Shutdown cleans up background processes. Call on program exit.
func (m Model) Shutdown() {
	if m.player != nil {
		m.player.Stop()
	}
	if m.downloader != nil {
		m.downloader.Close()
	}
}

// saveSettingsCmd persists settings to disk in a goroutine.
func saveSettingsCmd(s *settings.Settings) tea.Cmd {
	return func() tea.Msg {
		if err := s.Save(); err != nil {
			return SettingsSavedMsg{Error: err}
		}
		return SettingsSavedMsg{}
	}
}

// downloadCmd returns a command that reads one progress event from the
// downloader channel and forwards it as a DownloadProgressMsg.
func downloadCmd(d *downloader.Downloader) tea.Cmd {
	return func() tea.Msg {
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

// ─── Confirmation State ─────────────────────────────────────────────

// confirmAction values
const (
	confirmNone        = ""
	confirmClearQueue  = "clear-queue"
	confirmDeleteTrack = "delete-track"
)

// isConfirming returns true when a destructive action is awaiting confirmation.
func (m Model) isConfirming() bool {
	return m.confirmAction != confirmNone
}

// startConfirm sets the confirmation state for a destructive action.
func (m *Model) startConfirm(action, data string) {
	m.confirmAction = action
	m.confirmData = data
}

// clearConfirm resets the confirmation state.
func (m *Model) clearConfirm() {
	m.confirmAction = confirmNone
	m.confirmData = ""
}

// executeConfirmedAction runs the confirmed destructive action and returns
// the resulting model and command. Called after the user pressed the key a second time.
func (m *Model) executeConfirmedAction() (tea.Model, tea.Cmd) {
	action := m.confirmAction
	m.clearConfirm()

	switch action {
	case confirmClearQueue:
		m.queue.Clear()
		m.queueCursor = 0
		m.playerState = player.StateStopped
		m.position = 0
		m.duration = 0
		if m.player != nil {
			m.player.Stop()
		}
		m.setStatus("Queue cleared")
		return m, nil

	case confirmDeleteTrack:
		tracks := m.filteredLibrary()
		if m.libraryCursor >= 0 && m.libraryCursor < len(tracks) {
			t := tracks[m.libraryCursor]
			if t.FilePath != "" {
				if err := os.Remove(t.FilePath); err != nil {
					m.setStatus("Failed to delete: " + err.Error())
					return m, nil
				}
			}
			idx := -1
			for i, lt := range m.library {
				if lt.ID == t.ID {
					idx = i
					break
				}
			}
			if idx >= 0 {
				m.library = append(m.library[:idx], m.library[idx+1:]...)
			}
			m.clampLibraryOffset()
			m.setStatus("Deleted: " + t.Title)
		}
		return m, nil
	}

	return m, nil
}
