package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path"
	"time"

	"ytmgo/internal/db"
	"ytmgo/internal/downloader"
	"ytmgo/internal/library"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"

	tea "github.com/charmbracelet/bubbletea"
)

// ─── Intervals ─────────────────────────────────────────────────────────

// progressTickInterval drives the periodic 500ms tick for idle tip rotation
// and dev-mode position simulation.
const progressTickInterval = time.Second / 2

// playerTickInterval drives the smooth progress interpolation in the
// player bar. 50ms = 20fps, which is the lowest rate at which motion
// reads as continuous on a terminal.
const playerTickInterval = 50 * time.Millisecond

// ─── Search ─────────────────────────────────────────────────────────────

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
// seq is the generation counter — stale responses are ignored.
func fetchRecommendationsCmd(seq, limit int, cookieBrowser, userAgent string) tea.Cmd {
	return func() tea.Msg {
		results, err := search.FetchRecommendations(limit, cookieBrowser, userAgent)
		if err != nil {
			return RecommendationsMsg{Error: err, Seq: seq}
		}
		if results == nil {
			results = []search.Result{}
		}
		return RecommendationsMsg{Results: results, Seq: seq}
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

// fetchQuoteCmd fetches a random quote from dummyjson.
// On failure it returns nil so the fallback quote stays displayed.
func fetchQuoteCmd(seq int) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get("https://dummyjson.com/quotes/random")
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

// ─── Player commands ────────────────────────────────────────────────────

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
	return func() tea.Msg {
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
	return func() tea.Msg {
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
	return func() tea.Msg {
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
