package tui

import (
	"time"

	"ytmgo/internal/downloader"
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
		Seq     int // generation counter; stale responses are skipped
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
	recsSeq                int       // bumped each time R is pressed or a search starts
	updateAvailable        string    // "" = unknown, "latest" = up to date, "v0.X.Y" = update
	updateCheckManual      bool      // true when U was pressed to trigger the check

	// ── Library (local downloaded files) ──
	library       []queue.Track
	libraryCursor int
	libraryOffset int
	libraryLoaded bool // true after the first directory scan completes

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
	// suppressAutoAdvance prevents the SongEnded handler from calling
	// Next() when the old mpv was intentionally killed by a new
	// Play() call in playSelectedQueueItem. Without this, the stale
	// endedCmd from the previous playback fires a SongEndedMsg that
	// advances past the track the user just selected.
	suppressAutoAdvance bool

	// ── Mouse double-click tracking ──
	lastClickAt    time.Time
	lastClickY     int
	lastClickPanel Panel

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

	// ── Quote/tip rotation (shown in status bar when idle) ──
	currentQuote string
	fallbackIdx  int
	quoteSeq     int   // bumped each rotation; stale API responses dropped
	tipIndex     int   // used when ShowQuotes is off (classic tips)
	tickCount    int   // counts ticks between rotations
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
		currentQuote:           fallbackQuotes[0],
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
	"Press `D` twice to clear the entire queue",
	"`1` `2` `3` jump between Stream · Library · Settings",
	"`↑↓` or `j`/`k` to navigate lists",
	"`space` toggles play / pause",
	"`ctrl+↑` / `ctrl+↓` to reorder the queue",
	"`s` toggles shuffle · `r` cycles repeat",
	"Stream mode plays without downloading — toggle in Settings",
	"Press `x` on any track to download it for offline use",
	"Already have MP3s? Point Download Dir at them in Settings",
	"Set Default Volume in Settings so every track starts at your level",
	"Use a cookie browser in Settings for age-restricted tracks",
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
		m.player.Stop()
	}
	if m.downloader != nil {
		m.downloader.Close()
	}
}


