package gui

import (
	"errors"

	"ytmgo/internal/lyrics"
	"ytmgo/internal/queue"
)

// LyricsView is a song's lyrics for the Lyrics tab. Found is false when
// the song has none; Synced lines carry the second they are sung at.
type LyricsView struct {
	Found  bool        `json:"found"`
	Synced bool        `json:"synced"`
	Lines  []LyricLine `json:"lines"`
}

type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

// Lyrics finds t's lyrics as the TUI does: the cache first, so lyrics
// seen once work offline, then LRCLIB, then YouTube Music. A definite
// miss is cached too; a failure is not, so the next try asks again.
func (a *App) Lyrics(t queue.Track) (LyricsView, error) {
	if text, synced, found, err := a.cachedLyrics(t.ID); err == nil && found {
		if text == "" {
			return LyricsView{Lines: []LyricLine{}}, nil
		}
		return lyricsView(lyrics.FromText(text, synced)), nil
	}
	l, err := a.fetchLyrics(t.ID, t.Title, t.Artist, t.DurationSec)
	if errors.Is(err, lyrics.ErrNotFound) {
		a.cacheLyrics(t.ID, "", false)
		return LyricsView{Lines: []LyricLine{}}, nil
	}
	if err != nil {
		return LyricsView{}, problem(err)
	}
	a.cacheLyrics(t.ID, l.Raw, l.Synced)
	return lyricsView(l), nil
}

func lyricsView(l *lyrics.Lyrics) LyricsView {
	v := LyricsView{Found: len(l.Lines) > 0, Synced: l.Synced, Lines: make([]LyricLine, len(l.Lines))}
	for i, line := range l.Lines {
		v.Lines[i] = LyricLine{Time: line.Time, Text: line.Text}
	}
	return v
}
