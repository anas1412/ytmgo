package gui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"testing"
)

// Each failure the page has a message for, built the way it actually
// arrives: Go's own error types where the chain survives, and the text
// yt-dlp prints where it does not.
func TestKindOf(t *testing.T) {
	dns := &url.Error{Op: "Post", URL: "https://music.youtube.com/youtubei/v1/search",
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: &net.DNSError{Err: "Temporary failure in name resolution", Name: "music.youtube.com"}}}
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"offline: DNS, wrapped as the search wraps it", fmt.Errorf("youtube music search failed: %w", fmt.Errorf("ytmusic search: %w", dns)), KindOffline},
		{"offline: the same, chain lost to %v", fmt.Errorf("ytmusic search: %v", dns), KindOffline},
		{"offline: connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, KindOffline},
		{"offline: yt-dlp cannot reach YouTube", errors.New("yt-dlp could not open the track: ERROR: Unable to download webpage: <urlopen error [Errno -3] Temporary failure in name resolution>"), KindOffline},
		{"timeout: context", fmt.Errorf("search: %w", context.DeadlineExceeded), KindTimeout},
		{"timeout: http client", &url.Error{Op: "Post", URL: "x", Err: timeoutErr{}}, KindTimeout},
		{"timeout: yt-dlp, as ytresolve says it", errors.New("yt-dlp timed out after 1m0s"), KindTimeout},
		{"unavailable: removed", errors.New("yt-dlp could not open the track: ERROR: [youtube] abc: Video unavailable"), KindUnavailable},
		{"unavailable: age", errors.New("yt-dlp could not open the track: ERROR: [youtube] abc: Sign in to confirm your age"), KindUnavailable},
		{"missing tool", fmt.Errorf("yt-dlp could not open the track: %w", &exec.Error{Name: "yt-dlp", Err: exec.ErrNotFound}), KindMissingTool},
		{"other", errors.New("yt-dlp returned no stream"), KindOther},
	} {
		if got := kindOf(c.err); got != c.want {
			t.Errorf("%s: kind %q, want %q", c.name, got, c.want)
		}
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "Client.Timeout exceeded while awaiting headers" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// The page parses what crosses the binding, so it has to be JSON with
// both fields, and nil has to stay nil.
func TestProblemTravelsAsJSON(t *testing.T) {
	err := problem(errors.New("yt-dlp timed out after 1m0s"))
	var p Problem
	if json.Unmarshal([]byte(err.Error()), &p) != nil || p.Kind != KindTimeout || p.Detail == "" {
		t.Errorf("crossed as %q", err.Error())
	}
	if problem(nil) != nil {
		t.Error("problem(nil) is not nil")
	}
}
