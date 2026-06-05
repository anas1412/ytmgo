// Package discordrpc provides a thin wrapper around Discord Rich Presence
// via the Discord IPC protocol. All functions are safe to call even
// when Discord is not running — Init will fail silently and subsequent
// calls become no-ops.
//
// Uses the raw Discord IPC protocol (via rich-go's ipc package) instead
// of rich-go's client package, so we can set the activity name and type
// fields that the client package doesn't expose. This lets us show
// "Listening to Song Title" with a headphone icon instead of
// "Playing ytmgo" with a controller icon.
package discordrpc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"

	"github.com/hugolgst/rich-go/ipc"
)

// clientID is the Discord Application ID for ytmgo.
const clientID = "1512079698518081608"

var (
	mu       sync.Mutex
	loggedIn bool
)

// ─── IPC protocol types ─────────────────────────────────────────────

// activityPayload is the activity object sent in SET_ACTIVITY frames.
// Includes the "name" and "type" fields that rich-go's client.Activity
// doesn't expose. "name" overrides the text shown under the user's
// Discord name in the member list; "type" sets the icon and prefix
// (2 = Listening → headphone icon, 0 = Playing → controller icon).
type activityPayload struct {
	Name       string              `json:"name"`
	Type       int                 `json:"type"`
	Details    string              `json:"details,omitempty"`
	State      string              `json:"state,omitempty"`
	Assets     *assetsPayload      `json:"assets,omitempty"`
	Timestamps *timestampsPayload  `json:"timestamps,omitempty"`
}

type assetsPayload struct {
	LargeImage string `json:"large_image,omitempty"`
	LargeText  string `json:"large_text,omitempty"`
}

type timestampsPayload struct {
	Start uint64 `json:"start,omitempty"`
}

// setActivityFrame is the full SET_ACTIVITY IPC frame payload.
type setActivityFrame struct {
	Cmd   string           `json:"cmd"`
	Args  setActivityArgs  `json:"args"`
	Nonce string           `json:"nonce"`
}

type setActivityArgs struct {
	PID      int               `json:"pid"`
	Activity *activityPayload  `json:"activity"`
}

// ─── Lifecycle ──────────────────────────────────────────────────────

// Init attempts to connect to the local Discord client via RPC.
// Returns an error if Discord is not running or the socket is
// unreachable — callers should treat errors as non-fatal.
func Init() error {
	mu.Lock()
	defer mu.Unlock()
	if loggedIn {
		return nil
	}

	if err := ipc.OpenSocket(); err != nil {
		return fmt.Errorf("discord rpc: open socket: %w", err)
	}

	// Send handshake frame (opcode 0) with the client ID.
	h := map[string]string{"v": "1", "client_id": clientID}
	hData, _ := json.Marshal(h)
	ipc.Send(0, string(hData))

	loggedIn = true
	return nil
}

// Close disconnects from the Discord RPC socket. Safe to call multiple
// times and from any goroutine. Uses recover to guard against the
// panic in rich-go's ipc.CloseSocket on certain socket errors.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if !loggedIn {
		return
	}
	// ipc.CloseSocket can panic on some OS errors — recover safely.
	defer func() { recover() }()
	ipc.CloseSocket()
	loggedIn = false
}

// ─── Activity helpers ───────────────────────────────────────────────

// nonce generates a random hex string for IPC frame identification.
func nonce() string {
	buf := make([]byte, 16)
	rand.Read(buf)
	buf[6] = (buf[6] & 0x0f) | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}

// sendActivity sends a SET_ACTIVITY IPC frame with full control over
// the activity name, type, and other fields.
func sendActivity(name, details, state, largeImage, largeText string, activityType int, start *time.Time) {
	act := &activityPayload{
		Name:    name,
		Type:    activityType,
		Details: details,
		State:   state,
	}
	if largeImage != "" {
		act.Assets = &assetsPayload{
			LargeImage: largeImage,
			LargeText:  largeText,
		}
	}
	if start != nil {
		act.Timestamps = &timestampsPayload{
			Start: uint64(start.UnixNano() / 1e6),
		}
	}

	frame := setActivityFrame{
		Cmd: "SET_ACTIVITY",
		Args: setActivityArgs{
			PID:      os.Getpid(),
			Activity: act,
		},
		Nonce: nonce(),
	}

	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	ipc.Send(1, string(data))
}

// ShowIdle sets the presence to a default idle state (no track playing).
// Discord shows "Listening to Browsing Music" with the ytmgo logo.
func ShowIdle() {
	mu.Lock()
	logged := loggedIn
	mu.Unlock()
	if !logged {
		return
	}
	sendActivity("Browsing Music", "", "", "ytmgo-logo", "ytmgo – YT Music from Terminal", 2, nil)
}

// Update sets the Rich Presence to reflect the currently playing track.
// The activity name is the track title so Discord shows
// "Listening to Song Title" with a headphone icon.
func Update(track queue.Track, state player.State, position float64) {
	mu.Lock()
	logged := loggedIn
	mu.Unlock()
	if !logged {
		return
	}

	var start *time.Time
	if state == player.StatePlaying || state == player.StatePaused {
		t := time.Now().Add(-time.Duration(position * float64(time.Second)))
		start = &t
	}

	sendActivity(track.Title, track.Artist, "", "ytmgo-logo", "ytmgo – YT Music from Terminal", 2, start)
}
