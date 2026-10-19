package gui

import (
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
)

func TestShareLink(t *testing.T) {
	a := &App{settings: settings.Defaults(), saveSettings: func(*settings.Settings) error { return nil }}
	song := queue.Track{ID: "dQw4w9WgXcQ"}
	if got := a.ShareLink(song); got != "https://www.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("link %q", got)
	}
	a.SetCopyMusicLinks(true)
	if got := a.ShareLink(song); got != "https://music.youtube.com/watch?v=dQw4w9WgXcQ" {
		t.Errorf("music link %q", got)
	}
	if got := a.ShareLink(queue.Track{ID: "/music/a.m4a"}); got != "" {
		t.Errorf("a local file has link %q", got)
	}
}

// Off, Discord hears nothing more and is cleared.
func TestPresenceFollowsTheSetting(t *testing.T) {
	updates, closed := 0, 0
	a := &App{settings: settings.Defaults(), saveSettings: func(*settings.Settings) error { return nil },
		discord: &discord{update: func(*queue.Track, bool, float64) { updates++ }, close: func() { closed++ }}}
	a.Presence(&queue.Track{Title: "T"}, true, 3)
	a.SetDiscord(false)
	a.Presence(nil, false, 0)
	if updates != 1 || closed != 1 {
		t.Errorf("%d updates, %d closes; want 1 and 1", updates, closed)
	}
}
