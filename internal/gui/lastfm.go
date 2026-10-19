package gui

import (
	"errors"
	"strings"
	"time"

	"ytmgo/internal/lastfm"
	"ytmgo/internal/queue"
)

// LastFMState is what the Settings page shows of Last.fm: whether this
// build can scrobble at all, who is connected, and, while the user is
// approving ytmgo on last.fm, the link they were sent to.
type LastFMState struct {
	Available bool   `json:"available"`
	User      string `json:"user"`
	Waiting   bool   `json:"waiting"`
	AuthURL   string `json:"authURL"`
}

// lastfmAPI is the part of internal/lastfm the GUI calls, swapped out in
// tests so they need no network.
type lastfmAPI struct {
	configured func() bool
	token      func() (string, error)
	session    func(token string) (lastfm.Session, error)
	nowPlaying func(sessionKey string, t lastfm.Track) error
	scrobble   func(sessionKey string, t lastfm.Track, startedAt time.Time) error
}

var realLastFM = lastfmAPI{
	configured: lastfm.Configured,
	token:      lastfm.GetToken,
	session:    lastfm.GetSession,
	nowPlaying: lastfm.NowPlaying,
	scrobble:   lastfm.Scrobble,
}

// lfmState reads it. Callers hold a.mu.
func (a *App) lfmState() LastFMState {
	s := LastFMState{Available: a.lfm.configured(), User: a.settings.LastFMUser}
	if a.lfmToken != "" {
		s.Waiting = true
		s.AuthURL = lastfm.AuthURL(a.lfmToken)
	}
	return s
}

// LastFM is how Last.fm stands.
func (a *App) LastFM() LastFMState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lfmState()
}

// ConnectLastFM starts the TUI's flow: ask Last.fm for a token and hand
// back the page where the user approves ytmgo. No password passes
// through ytmgo. The page then calls CheckLastFM until it is approved.
func (a *App) ConnectLastFM() (LastFMState, error) {
	a.mu.Lock()
	if !a.lfm.configured() {
		defer a.mu.Unlock()
		return a.lfmState(), nil
	}
	a.mu.Unlock()
	token, err := a.lfm.token()
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		return a.lfmState(), problem(err)
	}
	a.lfmToken = token
	return a.lfmState(), nil
}

// CheckLastFM asks whether the user has approved ytmgo yet. Not yet is
// not an error: the state stays Waiting. Once approved the session key
// and user are saved, in the settings both apps share.
func (a *App) CheckLastFM() (LastFMState, error) {
	a.mu.Lock()
	token := a.lfmToken
	a.mu.Unlock()
	if token == "" {
		return a.LastFM(), nil
	}
	sess, err := a.lfm.session(token)
	a.mu.Lock()
	defer a.mu.Unlock()
	if errors.Is(err, lastfm.ErrNotAuthorized) {
		return a.lfmState(), nil
	}
	if err != nil {
		a.lfmToken = ""
		return a.lfmState(), problem(err)
	}
	a.lfmToken = ""
	a.settings.LastFMSessionKey = sess.Key
	a.settings.LastFMUser = sess.User
	a.saveSettings(a.settings)
	return a.lfmState(), nil
}

// CancelLastFM drops a pending approval: the page gives up after ten
// minutes, as the TUI does.
func (a *App) CancelLastFM() LastFMState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lfmToken = ""
	return a.lfmState()
}

// DisconnectLastFM forgets the session; scrobbling stops.
func (a *App) DisconnectLastFM() LastFMState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.LastFMSessionKey = ""
	a.settings.LastFMUser = ""
	a.saveSettings(a.settings)
	return a.lfmState()
}

// ScrobbleAfter is how many seconds into a song of this length it
// counts as heard (lastfm.ScrobbleAfter), or -1 for one too short to
// count, or when nobody is connected.
func (a *App) ScrobbleAfter(duration float64) float64 {
	a.mu.Lock()
	connected := a.settings.LastFMSessionKey != ""
	a.mu.Unlock()
	if after, ok := lastfm.ScrobbleAfter(duration); ok && connected {
		return after
	}
	return -1
}

// Scrobble records t on Last.fm as heard, from when it started (unix
// seconds), as the TUI sends it. Not connected, it does nothing.
func (a *App) Scrobble(t queue.Track, startedAt int64) error {
	a.mu.Lock()
	key := a.settings.LastFMSessionKey
	a.mu.Unlock()
	if key == "" {
		return nil
	}
	return problem(a.lfm.scrobble(key, lfmTrack(t), time.Unix(startedAt, 0)))
}

// nowPlaying tells Last.fm what started, if anyone is connected. It runs
// on its own: a slow or failing Last.fm must not hold up playback.
func (a *App) nowPlaying(t queue.Track) {
	a.mu.Lock()
	key := a.settings.LastFMSessionKey
	a.mu.Unlock()
	if key != "" {
		go a.lfm.nowPlaying(key, lfmTrack(t))
	}
}

// lfmTrack is what Last.fm is told about a song. The album is left out
// when it is really a play count, which old rows stored there; the TUI
// sends it as it is.
func lfmTrack(t queue.Track) lastfm.Track {
	album := t.Album
	if strings.HasSuffix(album, " plays") || strings.HasSuffix(album, " views") {
		album = ""
	}
	return lastfm.Track{Artist: t.Artist, Title: t.Title, Album: album, Duration: t.DurationSec}
}
