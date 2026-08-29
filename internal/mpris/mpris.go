// Package mpris exposes ytmgo as an org.mpris.MediaPlayer2 player on
// the D-Bus session bus, so media keys, playerctl, and desktop widgets
// (GNOME/KDE media controls, lock screens) can see and control playback.
//
// Data flows both ways:
//   - the TUI pushes state in with Publish (title, artist, art, status)
//   - external control (play/pause/next/prev) arrives on Commands()
//
// Everything is best-effort: no session bus (headless, macOS) means
// Start returns an error the caller treats as "feature unavailable".
package mpris

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

const (
	busName     = "org.mpris.MediaPlayer2.ytmgo"
	objectPath  = dbus.ObjectPath("/org/mpris/MediaPlayer2")
	rootIface   = "org.mpris.MediaPlayer2"
	playerIface = "org.mpris.MediaPlayer2.Player"
)

// Command is an external control request from a D-Bus client.
type Command int

const (
	CmdPlayPause Command = iota
	CmdPlay
	CmdPause
	CmdStop
	CmdNext
	CmdPrev
)

// Snapshot is the player state the TUI publishes.
type Snapshot struct {
	TrackID  string
	Title    string
	Artist   string
	CoverURL string
	Duration float64 // seconds
	Position float64 // seconds
	Playing  bool
	Paused   bool
	Volume   int // 0-100
	Shuffle  bool
	LoopOne  bool
	LoopAll  bool
}

// Service is a running MPRIS endpoint.
type Service struct {
	conn  *dbus.Conn
	props *prop.Properties
	cmds  chan Command

	mu   sync.Mutex
	snap Snapshot
}

// Start connects to the session bus and claims the MPRIS name.
func Start() (*Service, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("mpris: session bus: %w", err)
	}

	s := &Service{
		conn: conn,
		cmds: make(chan Command, 8),
	}

	propsSpec := map[string]map[string]*prop.Prop{
		rootIface: {
			"CanQuit":             {Value: false, Emit: prop.EmitTrue},
			"CanRaise":            {Value: false, Emit: prop.EmitTrue},
			"HasTrackList":        {Value: false, Emit: prop.EmitTrue},
			"Identity":            {Value: "ytmgo", Emit: prop.EmitTrue},
			"SupportedUriSchemes": {Value: []string{}, Emit: prop.EmitTrue},
			"SupportedMimeTypes":  {Value: []string{}, Emit: prop.EmitTrue},
		},
		playerIface: {
			"PlaybackStatus": {Value: "Stopped", Emit: prop.EmitTrue},
			"LoopStatus":     {Value: "None", Emit: prop.EmitTrue},
			"Rate":           {Value: 1.0, Emit: prop.EmitTrue},
			"MinimumRate":    {Value: 1.0, Emit: prop.EmitTrue},
			"MaximumRate":    {Value: 1.0, Emit: prop.EmitTrue},
			"Shuffle":        {Value: false, Emit: prop.EmitTrue},
			"Metadata":       {Value: map[string]dbus.Variant{}, Emit: prop.EmitTrue},
			"Volume":         {Value: 1.0, Emit: prop.EmitTrue},
			// Per the MPRIS spec, Position changes are not signalled.
			"Position":      {Value: int64(0), Emit: prop.EmitFalse},
			"CanGoNext":     {Value: true, Emit: prop.EmitTrue},
			"CanGoPrevious": {Value: true, Emit: prop.EmitTrue},
			"CanPlay":       {Value: true, Emit: prop.EmitTrue},
			"CanPause":      {Value: true, Emit: prop.EmitTrue},
			"CanSeek":       {Value: false, Emit: prop.EmitTrue},
			"CanControl":    {Value: true, Emit: prop.EmitTrue},
		},
	}
	props, err := prop.Export(conn, objectPath, propsSpec)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export properties: %w", err)
	}
	s.props = props

	root := rootHandler{}
	playerH := playerHandler{s: s}
	if err := conn.Export(root, objectPath, rootIface); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export root: %w", err)
	}
	if err := conn.Export(playerH, objectPath, playerIface); err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: export player: %w", err)
	}

	node := &introspect.Node{
		Name: string(objectPath),
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{Name: rootIface, Methods: introspect.Methods(root)},
			{Name: playerIface, Methods: introspect.Methods(playerH)},
		},
	}
	_ = conn.Export(introspect.NewIntrospectable(node), objectPath, "org.freedesktop.DBus.Introspectable")

	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("mpris: request name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner {
		// Another ytmgo instance already owns the name.
		conn.Close()
		return nil, fmt.Errorf("mpris: name %s already taken", busName)
	}

	return s, nil
}

// Commands returns the channel of external control requests.
func (s *Service) Commands() <-chan Command {
	return s.cmds
}

// Close releases the bus name and closes the connection.
func (s *Service) Close() {
	if s == nil || s.conn == nil {
		return
	}
	s.conn.ReleaseName(busName)
	s.conn.Close()
}

// trackIDRe strips characters that are invalid in a D-Bus object path.
var trackIDRe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// Publish pushes the current player state to the bus. Cheap when
// nothing visible changed; property-change signals fire only for
// fields that differ from the previous snapshot.
func (s *Service) Publish(snap Snapshot) {
	if s == nil || s.props == nil {
		return
	}
	// A dead bus connection surfaces as panics deep in godbus when
	// emitting signals; playback must never die because a desktop
	// session went away.
	defer func() { recover() }()

	s.mu.Lock()
	prev := s.snap
	s.snap = snap
	s.mu.Unlock()

	status := "Stopped"
	switch {
	case snap.Playing:
		status = "Playing"
	case snap.Paused:
		status = "Paused"
	}
	prevStatus := "Stopped"
	switch {
	case prev.Playing:
		prevStatus = "Playing"
	case prev.Paused:
		prevStatus = "Paused"
	}
	if status != prevStatus {
		s.props.SetMust(playerIface, "PlaybackStatus", status)
	}

	loop := "None"
	switch {
	case snap.LoopOne:
		loop = "Track"
	case snap.LoopAll:
		loop = "Playlist"
	}
	prevLoop := "None"
	switch {
	case prev.LoopOne:
		prevLoop = "Track"
	case prev.LoopAll:
		prevLoop = "Playlist"
	}
	if loop != prevLoop {
		s.props.SetMust(playerIface, "LoopStatus", loop)
	}

	if snap.Shuffle != prev.Shuffle {
		s.props.SetMust(playerIface, "Shuffle", snap.Shuffle)
	}
	if snap.Volume != prev.Volume {
		s.props.SetMust(playerIface, "Volume", float64(snap.Volume)/100.0)
	}

	if snap.TrackID != prev.TrackID || snap.Title != prev.Title ||
		snap.Duration != prev.Duration || snap.CoverURL != prev.CoverURL {
		id := trackIDRe.ReplaceAllString(snap.TrackID, "_")
		if id == "" {
			id = "none"
		}
		md := map[string]dbus.Variant{
			"mpris:trackid": dbus.MakeVariant(dbus.ObjectPath("/ytmgo/track/" + id)),
			"mpris:length":  dbus.MakeVariant(int64(snap.Duration * 1e6)),
			"xesam:title":   dbus.MakeVariant(snap.Title),
			"xesam:artist":  dbus.MakeVariant([]string{snap.Artist}),
		}
		if snap.CoverURL != "" {
			md["mpris:artUrl"] = dbus.MakeVariant(snap.CoverURL)
		}
		s.props.SetMust(playerIface, "Metadata", md)
	}

	// Position is served on demand (EmitFalse: no signal spam).
	s.props.SetMust(playerIface, "Position", int64(snap.Position*1e6))
}

func (s *Service) push(c Command) {
	select {
	case s.cmds <- c:
	default:
	}
}

// ─── D-Bus method handlers ──────────────────────────────────────────

type rootHandler struct{}

func (rootHandler) Raise() *dbus.Error { return nil }
func (rootHandler) Quit() *dbus.Error  { return nil }

type playerHandler struct {
	s *Service
}

func (h playerHandler) PlayPause() *dbus.Error {
	h.s.push(CmdPlayPause)
	return nil
}

func (h playerHandler) Play() *dbus.Error {
	h.s.push(CmdPlay)
	return nil
}

func (h playerHandler) Pause() *dbus.Error {
	h.s.push(CmdPause)
	return nil
}

func (h playerHandler) Stop() *dbus.Error {
	h.s.push(CmdStop)
	return nil
}

func (h playerHandler) Next() *dbus.Error {
	h.s.push(CmdNext)
	return nil
}

func (h playerHandler) Previous() *dbus.Error {
	h.s.push(CmdPrev)
	return nil
}

// Seek, SetPosition, and OpenUri are deliberately not exported:
// CanSeek is false and no URI schemes are supported, so per the MPRIS
// spec clients must not call them.
