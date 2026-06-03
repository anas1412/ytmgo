package tui

import (
	"encoding/json"
	"net/http"
	"time"

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

// checkUpdateCmd fetches the latest release tag from GitHub. Returns nil
// (no message) on failure so the update handler is never called — zero
// impact on the user experience when offline or rate limited.
func checkUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		if currentVersion == "dev" || currentVersion == "" {
			return nil // local/dev builds, skip
		}
		resp, err := http.Get("https://api.github.com/repos/anas1412/ytmgo/releases/latest")
		if err != nil {
			return nil
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil // rate limited, not found, etc.
		}
		var result struct {
			TagName string `json:"tag_name"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil
		}
		if result.TagName == "" || result.TagName == currentVersion {
			return nil // same version or empty — no update
		}
		return UpdateCheckMsg{LatestVersion: result.TagName}
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

// saveSettingsCmd persists settings to disk in a goroutine.
func saveSettingsCmd(s *settings.Settings) tea.Cmd {
	return func() tea.Msg {
		if err := s.Save(); err != nil {
			return SettingsSavedMsg{Error: err}
		}
		return SettingsSavedMsg{}
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
