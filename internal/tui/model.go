package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/library"
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

	// RecommendationsMsg carries the YouTube home page recommendations.
	RecommendationsMsg struct {
		Results []search.Result
		Error   error
	}

	// RecStreamMsg carries one incremental result from the streaming
	// recommendation fetcher. Result is nil when the stream has ended.
	// Ch is the stream channel (included every msg so the handler never
	// loses it). Cancel is only set on the first message.
	RecStreamMsg struct {
		Result *search.Result
		Ch     chan search.Result
		Cancel context.CancelFunc
	}

	// LibraryScanMsg carries the list of downloaded tracks found on disk.
	LibraryScanMsg struct {
		Tracks []queue.Track
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
	searchInput           textinput.Model
	searchFocused         bool
	searchCursor          int
	searchOffset          int
	results               []search.Result
	isSearching           bool
	showingRecommendations bool
	recStreamCh            chan search.Result
	recStreamCancel        context.CancelFunc

	// ── Library (local downloaded files) ──
	library       []queue.Track
	libraryCursor int
	libraryOffset int
	showingLibrary bool

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
		activePanel:            PanelSearch,
		searchInput:            ti,
		results:                []search.Result{},
		queue:                  queue.New(),
		playerState:            player.StateStopped,
		volume:                 80,
		showingRecommendations: true,
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

// fetchRecommendationsCmd fires a request for YouTube home page recommendations.
func fetchRecommendationsCmd() tea.Cmd {
	return func() tea.Msg {
		results, err := search.FetchRecommendations(40)
		if err != nil {
			return RecommendationsMsg{Error: err}
		}
		if results == nil {
			results = []search.Result{}
		}
		return RecommendationsMsg{Results: results}
	}
}

// startRecStreamCmd spins up a new recommendation stream. The first
// RecStreamMsg carries the channel and cancel function so the model can
// store them for subsequent reads and cancellation (e.g. when R is
// pressed again).
func startRecStreamCmd() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan search.Result, 10)
		ctx, cancel := context.WithCancel(context.Background())
		go search.StreamRecommendations(20, ch, ctx.Done())

		// Block until the first result arrives or the stream ends.
		r, ok := <-ch
		if !ok {
			return RecStreamMsg{Ch: ch, Cancel: cancel, Result: nil}
		}
		return RecStreamMsg{Ch: ch, Cancel: cancel, Result: &r}
	}
}

// readNextRecCmd returns a tea.Cmd that reads one result from the
// recommendation stream channel and returns it as a RecStreamMsg.
func readNextRecCmd(ch chan search.Result) tea.Cmd {
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return RecStreamMsg{Ch: ch, Result: nil}
		}
		return RecStreamMsg{Ch: ch, Result: &r}
	}
}

// scanLibraryCmd scans the downloads directory for existing audio files.
func scanLibraryCmd() tea.Cmd {
	return func() tea.Msg {
		tracks, err := library.ScanDir(downloadDir())
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
// When the input is empty or not showing library, returns all tracks.
func (m Model) filteredLibrary() []queue.Track {
	if !m.showingLibrary {
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

// Shutdown cleans up background processes. Call on program exit.
func (m Model) Shutdown() {
	if m.recStreamCancel != nil {
		m.recStreamCancel()
	}
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

