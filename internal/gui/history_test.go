package gui

import (
	"testing"

	"ytmgo/internal/db"
)

// A play comes back playable: a video id gets its watch URL, an older
// id none; a song the library has a file for plays from it.
func TestHistoryIsPlayable(t *testing.T) {
	f := file(t, "Song", "Queen")
	a, _ := libApp(t, f)
	a.Library()
	a.plays = func(limit, _ int) ([]db.PlayHistoryEntry, error) {
		if limit != historyShown {
			t.Errorf("asked for %d plays", limit)
		}
		return []db.PlayHistoryEntry{
			{TrackID: "dQw4w9WgXcQ", Title: "Song", Artist: "Queen", PlayedAt: "2026-10-19T10:00:00Z"},
			{TrackID: "12345", Title: "Old", Artist: "X"},
		}, nil
	}
	h, err := a.History()
	if err != nil || len(h) != 2 {
		t.Fatalf("%v, %v", h, err)
	}
	if h[0].URL != "https://music.youtube.com/watch?v=dQw4w9WgXcQ" || h[0].PlayedAt == "" {
		t.Errorf("first: %+v", h[0])
	}
	if !h[0].Downloaded || h[0].FilePath != f.FilePath {
		t.Errorf("not matched to its file: %+v", h[0].Track)
	}
	if h[1].URL != "" {
		t.Errorf("a legacy id got a URL: %q", h[1].URL)
	}
}
