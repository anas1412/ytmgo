package gui

import (
	"errors"
	"testing"
	"time"

	"ytmgo/internal/lastfm"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
)

type lfmCalls struct {
	approved  bool
	scrobbled []lastfm.Track
	saved     int
}

func lfmApp() (*App, *lfmCalls) {
	c := &lfmCalls{}
	a := &App{settings: settings.Defaults(), saveSettings: func(*settings.Settings) error { c.saved++; return nil }}
	a.lfm = lastfmAPI{
		configured: func() bool { return true },
		token:      func() (string, error) { return "tok", nil },
		session: func(string) (lastfm.Session, error) {
			if !c.approved {
				return lastfm.Session{}, lastfm.ErrNotAuthorized
			}
			return lastfm.Session{User: "anas", Key: "key"}, nil
		},
		nowPlaying: func(string, lastfm.Track) error { return nil },
		scrobble: func(_ string, t lastfm.Track, _ time.Time) error {
			c.scrobbled = append(c.scrobbled, t)
			return nil
		},
	}
	return a, c
}

// Connecting as the TUI does: a link to approve, then waiting without
// error until it is approved, then the session saved.
func TestConnectLastFM(t *testing.T) {
	a, c := lfmApp()
	s, err := a.ConnectLastFM()
	if err != nil || !s.Waiting || s.AuthURL == "" {
		t.Fatalf("connect: %+v, %v", s, err)
	}
	if s, err = a.CheckLastFM(); err != nil || !s.Waiting || s.User != "" {
		t.Errorf("not yet approved: %+v, %v", s, err)
	}
	c.approved = true
	if s, err = a.CheckLastFM(); err != nil || s.Waiting || s.User != "anas" || a.settings.LastFMSessionKey != "key" || c.saved != 1 {
		t.Errorf("approved: %+v, %v, saved %d", s, err, c.saved)
	}
	if s = a.DisconnectLastFM(); s.User != "" || a.settings.LastFMSessionKey != "" {
		t.Errorf("disconnect: %+v", s)
	}
}

// A refusal that is not "not yet" ends the wait with an error.
func TestCheckLastFMFailure(t *testing.T) {
	a, _ := lfmApp()
	a.lfm.session = func(string) (lastfm.Session, error) { return lastfm.Session{}, errors.New("lastfm: token expired") }
	a.ConnectLastFM()
	if s, err := a.CheckLastFM(); err == nil || s.Waiting {
		t.Errorf("%+v, %v", s, err)
	}
}

// Scrobbling only while connected, with the play-count album dropped.
func TestScrobble(t *testing.T) {
	a, c := lfmApp()
	song := queue.Track{Title: "Homage", Artist: "Mild High Club", Album: "4.2M plays", DurationSec: 178}
	a.Scrobble(song, 100)
	if len(c.scrobbled) != 0 || a.ScrobbleAfter(178) != -1 {
		t.Fatal("scrobbled with nobody connected")
	}
	a.settings.LastFMSessionKey = "key"
	if got := a.ScrobbleAfter(178); got != 89 {
		t.Errorf("ScrobbleAfter(178) = %v, want 89", got)
	}
	a.Scrobble(song, 100)
	if len(c.scrobbled) != 1 || c.scrobbled[0].Album != "" || c.scrobbled[0].Duration != 178 {
		t.Errorf("scrobbled %+v", c.scrobbled)
	}
}

func TestSetSearchLimit(t *testing.T) {
	a, _ := lfmApp()
	for in, want := range map[int]int{20: 20, 23: 25, 2: 5, 400: 100} {
		if got := a.SetSearchLimit(in); got != want {
			t.Errorf("SetSearchLimit(%d) = %d, want %d", in, got, want)
		}
	}
}
