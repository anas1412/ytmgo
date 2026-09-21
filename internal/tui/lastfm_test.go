package tui

import (
	"errors"
	"testing"
	"time"

	"ytmgo/internal/lastfm"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
)

func scrobbleModel(t *testing.T) Model {
	t.Helper()
	isolateUserDirs(t)
	m := InitialModel()
	m.npOn = false
	m.settings.LastFMSessionKey = "sk"
	m.playerState = player.StatePlaying
	m.queue.Add(queue.Track{ID: "aaaaaaaaaaa", Title: "Song", Artist: "Artist", DurationSec: 200})
	m.queue.SetCurrentIndex(0)
	return m
}

// A track scrobbles once it crosses the line, and then never again for
// the same play — position keeps arriving after the threshold.
func TestScrobbleFiresOnce(t *testing.T) {
	m := scrobbleModel(t)
	// player has no mpv here, so handlePosition returns just our cmd.
	m.player = nil

	nm, cmd := m.handlePosition(PositionMsg{Position: 50, Duration: 200})
	m = nm.(Model)
	if cmd != nil || m.scrobbled {
		t.Fatal("scrobbled before halfway")
	}
	nm, cmd = m.handlePosition(PositionMsg{Position: 100, Duration: 200})
	m = nm.(Model)
	if cmd == nil || !m.scrobbled {
		t.Fatal("did not scrobble at halfway")
	}
	nm, cmd = m.handlePosition(PositionMsg{Position: 150, Duration: 200})
	m = nm.(Model)
	if cmd != nil {
		t.Fatal("scrobbled the same play twice")
	}
}

// Without a session key nothing is ever sent, whatever the position.
func TestNoScrobbleWithoutSession(t *testing.T) {
	m := scrobbleModel(t)
	m.player = nil
	m.settings.LastFMSessionKey = ""
	nm, cmd := m.handlePosition(PositionMsg{Position: 190, Duration: 200})
	if cmd != nil || nm.(Model).scrobbled {
		t.Fatal("scrobbled with no Last.fm account connected")
	}
}

// Connecting stores the key and the name, forgets the token, and saves.
func TestSessionConnects(t *testing.T) {
	m := scrobbleModel(t)
	m.settings.LastFMSessionKey = ""
	m.lastfmToken = "tok"
	nm, cmd := m.handleLastFMSession(LastFMSessionMsg{Session: lastfm.Session{User: "anas", Key: "k1"}})
	m = nm.(Model)
	if m.settings.LastFMSessionKey != "k1" || m.settings.LastFMUser != "anas" {
		t.Fatalf("session not stored: %+v", m.settings)
	}
	if m.lastfmToken != "" {
		t.Error("token kept after it was spent")
	}
	if cmd == nil {
		t.Error("connecting did not save settings")
	}
}

// Not-yet-approved keeps the token; any other failure drops it.
func TestSessionNotApprovedKeepsToken(t *testing.T) {
	m := scrobbleModel(t)
	m.lastfmToken = "tok"
	nm, _ := m.handleLastFMSession(LastFMSessionMsg{Error: lastfm.ErrNotAuthorized})
	if nm.(Model).lastfmToken != "tok" {
		t.Fatal("token dropped while the user was still approving it")
	}
	nm, _ = m.handleLastFMSession(LastFMSessionMsg{Error: errors.New("boom")})
	if nm.(Model).lastfmToken != "" {
		t.Fatal("token kept after a real failure")
	}
}

// The approval is watched for: an unapproved answer to the background
// poll schedules the next poll, while the same answer to the user's own
// Enter only tells them — one chain, never two.
func TestApprovalIsPolled(t *testing.T) {
	m := scrobbleModel(t)
	m.lastfmToken = "tok"
	m.lastfmTokenAt = time.Now()
	if _, cmd := m.handleLastFMSession(LastFMSessionMsg{Error: lastfm.ErrNotAuthorized, Polled: true}); cmd == nil {
		t.Fatal("polled 'not yet' did not schedule the next poll")
	}
	if _, cmd := m.handleLastFMSession(LastFMSessionMsg{Error: lastfm.ErrNotAuthorized, Polled: false}); cmd != nil {
		t.Fatal("the user's own Enter started a second poll chain")
	}
}

// A poll for a token that is no longer pending — disconnected, or a
// fresh link requested — must do nothing, or it would connect the wrong
// approval or keep the old chain alive beside the new one.
func TestStalePollIsIgnored(t *testing.T) {
	m := scrobbleModel(t)
	m.lastfmToken = "new"
	if _, cmd := m.handleLastFMPoll(LastFMPollMsg{Token: "old"}); cmd != nil {
		t.Fatal("a poll for a superseded token still checked it")
	}
	if _, cmd := m.handleLastFMPoll(LastFMPollMsg{Token: "new"}); cmd == nil {
		t.Fatal("a poll for the pending token did not check it")
	}
	m.lastfmToken = ""
	if _, cmd := m.handleLastFMPoll(LastFMPollMsg{Token: "new"}); cmd != nil {
		t.Fatal("a poll kept going after disconnect")
	}
}

// After the timeout the row is handed back rather than polled forever.
func TestPollGivesUp(t *testing.T) {
	m := scrobbleModel(t)
	m.lastfmToken = "tok"
	m.lastfmTokenAt = time.Now().Add(-lastfmPollTimeout - time.Second)
	nm, cmd := m.handleLastFMSession(LastFMSessionMsg{Error: lastfm.ErrNotAuthorized, Polled: true})
	if cmd != nil {
		t.Fatal("still polling past the timeout")
	}
	if nm.(Model).lastfmToken != "" {
		t.Fatal("token kept past the timeout; the row would show 'waiting' forever")
	}
}
