// Package discordrpc provides a thin wrapper around Discord Rich Presence
// via github.com/hugolgst/rich-go. All functions are safe to call even
// when Discord is not running — Init will fail silently and subsequent
// calls become no-ops.
package discordrpc

import (
	"fmt"
	"sync"
	"time"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"

	"github.com/hugolgst/rich-go/client"
)

// clientID is the Discord Application ID for ytmgo.
// Set this to the Application ID from discord.com/developers.
// Users see "ytmgo" as the app name on their Discord profile.
const clientID = "1512079698518081608"

var (
	mu       sync.Mutex
	loggedIn bool
)

// Init attempts to connect to the local Discord client via RPC.
// Returns an error if Discord is not running or the socket is
// unreachable — callers should treat errors as non-fatal.
func Init() error {
	mu.Lock()
	defer mu.Unlock()
	if loggedIn {
		return nil
	}
	if err := client.Login(clientID); err != nil {
		return fmt.Errorf("discord rpc login: %w", err)
	}
	loggedIn = true
	return nil
}

// idleActivity is the presence shown while the app is open but nothing is
// playing. It keeps the ytmgo logo visible on the Discord profile.
var idleActivity = client.Activity{
	Details:    "Browsing Music",
	State:      "Idle",
	LargeImage: "ytmgo-logo",
	LargeText:  "ytmgo — Terminal YouTube Music",
}

// ShowIdle sets the presence to a default idle state (no track playing).
// It is a no-op if Init has not been called successfully.
func ShowIdle() {
	mu.Lock()
	logged := loggedIn
	mu.Unlock()
	if !logged {
		return
	}
	client.SetActivity(idleActivity)
}

// Update sets the Rich Presence to reflect the currently playing track.
// It is a no-op if Init has not been called successfully.
func Update(track queue.Track, state player.State, position float64) {
	mu.Lock()
	logged := loggedIn
	mu.Unlock()
	if !logged {
		return
	}

	activity := client.Activity{
		Details:    track.Title,
		State:      track.Artist,
		LargeImage: "ytmgo-logo",
		LargeText:  "ytmgo — Terminal YouTube Music",
	}

	if state == player.StatePlaying || state == player.StatePaused {
		start := time.Now().Add(-time.Duration(position * float64(time.Second)))
		activity.Timestamps = &client.Timestamps{
			Start: &start,
		}
	}

	// Best-effort SetActivity — log error but don't propagate
	if err := client.SetActivity(activity); err != nil {
		mu.Lock()
		loggedIn = false
		mu.Unlock()
	}
}

// Clear removes the Rich Presence from the user's profile and
// disconnects from the Discord RPC socket. Safe to call multiple times.
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	if !loggedIn {
		return
	}
	client.Logout()
	loggedIn = false
}

// Close is an alias for Clear, provided for symmetry with the
// player.Player and other lifecycle-managed components.
func Close() {
	Clear()
}
