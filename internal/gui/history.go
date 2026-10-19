package gui

import (
	"ytmgo/internal/db"
	"ytmgo/internal/queue"
	"ytmgo/internal/ytmusic"
)

// HistoryEntry is a play on the History page: the song, and when it
// started (ISO 8601, UTC). History is recorded when a song starts, as in
// the TUI (Played).
type HistoryEntry struct {
	queue.Track
	PlayedAt string `json:"playedAt"`
}

// historyShown is how many plays the History page lists, as the TUI's.
const historyShown = 100

// History lists the newest plays, newest first. The database keeps
// only the id, title, artist and cover of each, so the song plays from
// its watch URL, or from the file when the library has it.
func (a *App) History() ([]HistoryEntry, error) {
	plays, err := a.plays(historyShown, 0)
	if err != nil {
		return nil, problem(err)
	}
	tracks := make([]queue.Track, len(plays))
	for i, p := range plays {
		tracks[i] = historyTrack(p)
	}
	a.mark(tracks)
	out := make([]HistoryEntry, len(plays))
	for i, p := range plays {
		out[i] = HistoryEntry{Track: tracks[i], PlayedAt: p.PlayedAt}
	}
	return out, nil
}

// historyTrack is the TUI's historyEntryTrack: a song id is a YouTube
// video id, so its watch URL follows from it; older entries have none,
// and play only from a file.
func historyTrack(p db.PlayHistoryEntry) queue.Track {
	t := queue.Track{ID: p.TrackID, Title: p.Title, Artist: p.Artist, CoverURL: p.CoverURL}
	if ytmusic.IsVideoID(p.TrackID) {
		t.URL = ytmusic.WatchURL(p.TrackID)
	}
	return t
}

// ClearHistory forgets every play: the TUI's C, after the page asks.
// Recommendations start again from nothing.
func (a *App) ClearHistory() error {
	return problem(a.clearPlays())
}
