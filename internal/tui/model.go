package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// ─── Panel focus ───────────────────────────────────────────────────

// Panel identifies which panel has keyboard focus.
type Panel int

const (
	PanelSearch Panel = iota // left — search results
	PanelQueue               // right — queue
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
		Progress float64 // 0–100
		Done     bool
		FilePath string // local path once downloaded
		Error    error
	}

	// SearchResultsMsg carries results back from a yt-dlp search.
	SearchResultsMsg struct {
		Results []search.Result
		Error   error
	}
)

// tickMsg triggers periodic UI updates (progress bar animation).
type tickMsg struct{}

// ─── Model ──────────────────────────────────────────────────────────

// Model is the root Bubble Tea model for the ytmgo TUI.
type Model struct {
	// ── Window ──
	width  int
	height int
	ready  bool // true after first WindowSizeMsg

	// ── Focus ──
	activePanel Panel
	showHelp    bool
	quitting    bool

	// ── Search ──
	searchInput  textinput.Model
	searchFocused bool
	searchCursor int
	results      []search.Result
	isSearching  bool

	// ── Queue ──
	queue       *queue.Queue
	queueCursor int

	// ── Player ──
	player      *player.Player
	playerState player.State
	position    float64 // seconds
	duration    float64 // seconds
	volume      int

	// ── Downloads ──
	downloader    *downloader.Downloader
	downloading   bool
	downloadTitle string
	downloadPct   float64
	downloadDone  bool
	downloadErr   error

	// ── Status ──
	statusMessage string
	err           error
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

	return Model{
		activePanel:  PanelSearch,
		searchInput:  ti,
		results:      []search.Result{},
		queue:        queue.New(),
		playerState:  player.StateStopped,
		volume:       80,
	}
}

// ─── Commands ────────────────────────────────────────────────────────

// searchCmd fires a yt-dlp search in a goroutine and sends results back.
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

// tickCmd returns a command that fires every 500ms for progress animation.
func tickCmd() tea.Cmd {
	return tea.Tick(progressTickInterval, func(_ time.Time) tea.Msg {
		return tickMsg{}
	})
}

const progressTickInterval = time.Second / 2

// ─── Helpers ────────────────────────────────────────────────────────

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

// downloadDir returns the downloads directory next to the binary.
func downloadDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "downloads" // fallback
	}
	dir := filepath.Join(filepath.Dir(exe), "downloads")
	os.MkdirAll(dir, 0755)
	return dir
}

// ensureDownloader creates the downloader if it doesn't exist yet.
func (m *Model) ensureDownloader() {
	if m.downloader == nil {
		m.downloader = downloader.New(downloadDir())
	}
}

// panelHeight returns how many terminal lines the panel area occupies.
// Must stay in sync with renderPanels() in view.go.
func (m Model) panelHeight() int {
	h := m.height - 16
	if m.err != nil || m.statusMessage != "" {
		h = m.height - 17
	}
	if h < 5 {
		h = 5
	}
	return h
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
			Progress: evt.Progress,
			Done:     evt.Status == downloader.StatusDone || evt.Status == downloader.StatusSkipped,
			FilePath: evt.FilePath,
			Error:    evt.Err,
		}
	}
}

