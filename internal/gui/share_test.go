package gui

import (
	"strings"
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
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
	a := &App{settings: settings.Defaults(), q: queue.New(), saveSettings: func(*settings.Settings) error { return nil },
		discord: &discord{update: func(*queue.Track, bool, float64) { updates++ }, close: func() { closed++ }}}
	a.Presence(&queue.Track{Title: "T"}, true, 3)
	a.SetDiscord(false)
	a.Presence(nil, false, 0)
	if updates != 1 || closed != 1 {
		t.Errorf("%d updates, %d closes; want 1 and 1", updates, closed)
	}
}

// A library song has no cover on the web, so Discord gets one: the
// same song's from the queue, or else from YouTube Music, asked once.
func TestPresenceFindsACoverForLibrarySongs(t *testing.T) {
	var shown []string
	searches := 0
	a := &App{settings: settings.Defaults(), q: queue.New(),
		saveQueue: func([]queue.Track, int, bool, bool, bool) error { return nil },
		discord:   &discord{update: func(t *queue.Track, _ bool, _ float64) { shown = append(shown, t.CoverURL) }, close: func() {}},
		search: func(q string, limit int) ([]search.Result, error) {
			searches++
			if q != "Djo End of Beginning" || limit != 1 {
				t.Errorf("searched %q, %d", q, limit)
			}
			return []search.Result{{CoverURL: "https://yt/djo.jpg"}}, nil
		},
	}
	a.Add(queue.Track{ID: "yt1", Title: "Homage (Official Video)", Artist: "Mild High Club - Topic", CoverURL: "https://yt/homage.jpg"})

	a.Presence(&queue.Track{ID: "/m/a.m4a", Title: "Homage", Artist: "Mild High Club", FilePath: "/m/a.m4a"}, true, 0)
	file := queue.Track{ID: "/m/b.m4a", Title: "End of Beginning", Artist: "Djo", FilePath: "/m/b.m4a"}
	a.Presence(&file, true, 0)
	a.Presence(&file, false, 3)
	a.Presence(&queue.Track{ID: "yt2", Title: "X", CoverURL: "https://yt/own.jpg"}, true, 0)

	want := []string{"https://yt/homage.jpg", "https://yt/djo.jpg", "https://yt/djo.jpg", "https://yt/own.jpg"}
	if strings.Join(shown, " ") != strings.Join(want, " ") {
		t.Errorf("Discord showed %v, want %v", shown, want)
	}
	if searches != 1 {
		t.Errorf("searched %d times, want once", searches)
	}
}
