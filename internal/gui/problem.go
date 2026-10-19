package gui

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os/exec"
	"strings"
)

// Kinds of failure the page knows how to explain. They name what the
// user can do something about, not where in the code it broke.
const (
	KindOffline     = "offline"      // no network, or a name that will not resolve
	KindTimeout     = "timeout"      // the network is there but did not answer in time
	KindUnavailable = "unavailable"  // removed, private, age-restricted, blocked
	KindMissingTool = "missing-tool" // yt-dlp (or another tool) is not installed
	KindOther       = "other"
)

// Problem is an error the page can act on: Kind picks the message and
// the action, Detail keeps the original text behind "Show details".
// Wails hands the page only an error's text, so it travels as JSON.
type Problem struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

func (p *Problem) Error() string {
	b, _ := json.Marshal(p)
	return string(b)
}

// problem turns any error from the packages below into a Problem.
func problem(err error) error {
	if err == nil {
		return nil
	}
	return &Problem{Kind: kindOf(err), Detail: err.Error()}
}

func kindOf(err error) string {
	var dns *net.DNSError
	var op *net.OpError
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return KindMissingTool
	case errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &timeout) && timeout.Timeout():
		return KindTimeout
	case errors.As(err, &dns), errors.As(err, &op):
		return KindOffline
	}

	// What is left only says what happened in its text: yt-dlp reports
	// through stderr, and some callers wrap with %v, losing the chain.
	msg := strings.ToLower(err.Error())
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(msg, w) {
				return true
			}
		}
		return false
	}
	switch {
	case has("video unavailable", "private video", "sign in to confirm your age",
		"not available in your country", "has been removed", "copyright"):
		return KindUnavailable
	case has("timed out", "timeout"):
		return KindTimeout
	case has("name resolution", "no such host", "dial tcp", "getaddrinfo",
		"network is unreachable", "unable to download webpage", "failed to resolve"):
		return KindOffline
	case has("executable file not found"):
		return KindMissingTool
	}
	return KindOther
}
