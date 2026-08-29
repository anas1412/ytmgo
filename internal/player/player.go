// Package player controls a single persistent mpv process over its JSON
// IPC socket.
//
// mpv is spawned once with --idle=yes and kept alive for the whole
// session; tracks are switched with `loadfile` instead of killing and
// respawning the process. One long-lived socket connection carries
// everything:
//
//   - position/duration/pause arrive as observe_property change events
//     (no polling, no reconnect-per-query)
//   - track end arrives as an end-file event with a reason, so a track
//     switch or manual stop is never mistaken for a natural end
//
// The reason field is what makes auto-advance safe: only "eof" (and
// "error", so a broken stream is skipped instead of stalling the queue)
// is forwarded to Ended(). "stop"/"quit"/"redirect" are ignored.
package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

// observe_property ids registered on connect.
const (
	obsTimePos  = 1
	obsDuration = 2
	obsPause    = 3
)

// PositionUpdate is sent for every mpv time-pos change (~1/s).
type PositionUpdate struct {
	Position float64 // seconds
	Duration float64 // seconds
}

// Player controls one persistent mpv instance.
type Player struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	conn       net.Conn
	socketPath string
	state      State
	volume     int
	duration   float64 // last observed duration for the loaded track
	closing    bool    // set by Shutdown; suppresses teardown events
	posCh      chan PositionUpdate
	endCh      chan struct{}
}

func New() *Player {
	return &Player{
		volume:     80,
		socketPath: socketPath(),
		posCh:      make(chan PositionUpdate, 10),
		endCh:      make(chan struct{}, 1),
	}
}

// socketPath returns a per-process socket path in the user runtime dir,
// so multiple ytmgo instances never fight over one socket and the path
// isn't predictable in a world-writable /tmp.
func socketPath() string {
	base := os.Getenv("XDG_RUNTIME_DIR")
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, fmt.Sprintf("ytmgo-mpv-%d.sock", os.Getpid()))
}

// Positions returns the channel of position updates.
func (p *Player) Positions() <-chan PositionUpdate {
	return p.posCh
}

// Ended returns a channel that receives when a track ends naturally
// (or errors out mid-stream, so the queue can skip it).
func (p *Player) Ended() <-chan struct{} {
	return p.endCh
}

// Play loads a new track into the persistent mpv instance, spawning
// mpv first if it isn't running yet.
func (p *Player) Play(filePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureRunning(); err != nil {
		return err
	}

	p.duration = 0
	if err := p.send("loadfile", filePath, "replace"); err != nil {
		return fmt.Errorf("mpv loadfile: %w", err)
	}
	// A previous pause state persists across loadfile; always resume.
	_ = p.send("set_property", "pause", false)
	_ = p.send("set_property", "volume", p.volume)
	p.state = StatePlaying
	return nil
}

// Pause toggles pause. Returns true on success, false if the IPC command
// failed (mpv didn't actually pause/resume).
func (p *Player) Pause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch p.state {
	case StatePlaying:
		if err := p.send("set_property", "pause", true); err != nil {
			return false
		}
		p.state = StatePaused
		return true
	case StatePaused:
		if err := p.send("set_property", "pause", false); err != nil {
			return false
		}
		p.state = StatePlaying
		return true
	}
	return false
}

// Stop unloads the current track. mpv stays alive and idle, ready for
// the next Play without respawn cost.
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.send("stop")
	p.state = StateStopped
}

// Seek seeks by delta seconds (can be negative). Best-effort.
func (p *Player) Seek(delta float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.send("seek", delta, "relative")
}

// SetVolume sets volume 0-100.
func (p *Player) SetVolume(v int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.volume = v
	_ = p.send("set_property", "volume", v)
}

// Volume returns current volume.
func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// State returns current player state.
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// Shutdown quits mpv and cleans up. Call once on program exit.
func (p *Player) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closing = true
	_ = p.send("quit")
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		// Grace period for the quit command, then force-kill.
		done := make(chan struct{})
		cmd := p.cmd
		go func() {
			cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
		}
		p.cmd = nil
	}
	p.state = StateStopped
	os.Remove(p.socketPath)
}

// ─── internals ──────────────────────────────────────────────────────

// ensureRunning spawns mpv and connects the IPC socket if needed.
// Caller must hold p.mu.
func (p *Player) ensureRunning() error {
	if p.conn != nil {
		return nil
	}

	os.Remove(p.socketPath)

	args := []string{
		"--idle=yes",
		"--no-video",
		"--audio-display=no",
		"--ytdl-format=bestaudio/best",
		fmt.Sprintf("--volume=%d", p.volume),
		fmt.Sprintf("--input-ipc-server=%s", p.socketPath),
		"--quiet",
		"--really-quiet",
	}
	// Test hook: lets integration tests run without an audio device
	// (YTMGO_MPV_AO=null).
	if ao := os.Getenv("YTMGO_MPV_AO"); ao != "" {
		args = append(args, "--ao="+ao)
	}

	cmd := exec.Command("mpv", args...)
	cmd.SysProcAttr = procAttr()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpv failed to start: %w (is mpv installed?)", err)
	}
	p.cmd = cmd

	// Watch for unexpected mpv death. A track playing when the process
	// dies is reported as ended so the queue advances and the next Play
	// respawns mpv, instead of the app stalling silently.
	go func() {
		cmd.Wait()
		p.mu.Lock()
		if p.closing || p.cmd != cmd {
			p.mu.Unlock()
			return
		}
		wasActive := p.state != StateStopped
		p.cmd = nil
		if p.conn != nil {
			p.conn.Close()
			p.conn = nil
		}
		p.state = StateStopped
		p.mu.Unlock()
		if wasActive {
			p.emitEnded()
		}
	}()

	conn, err := p.dialSocket()
	if err != nil {
		cmd.Process.Kill()
		p.cmd = nil
		return fmt.Errorf("mpv IPC connect: %w", err)
	}
	p.conn = conn

	go p.readLoop(conn)

	_ = p.send("observe_property", obsTimePos, "time-pos")
	_ = p.send("observe_property", obsDuration, "duration")
	_ = p.send("observe_property", obsPause, "pause")
	return nil
}

// dialSocket waits for the mpv socket to appear and connects to it.
func (p *Player) dialSocket() (net.Conn, error) {
	var lastErr error
	for i := 0; i < 100; i++ { // up to 5s
		conn, err := net.DialTimeout("unix", p.socketPath, time.Second)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}
	return nil, lastErr
}

// send writes one IPC command on the persistent connection.
// Caller must hold p.mu.
func (p *Player) send(args ...interface{}) error {
	if p.conn == nil {
		return fmt.Errorf("mpv not running")
	}
	data, err := json.Marshal(map[string]interface{}{"command": args})
	if err != nil {
		return fmt.Errorf("mpv command marshal: %w", err)
	}
	data = append(data, '\n')
	p.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := p.conn.Write(data); err != nil {
		return fmt.Errorf("mpv command write: %w", err)
	}
	return nil
}

// ipcEvent is one line from the mpv IPC stream. Command responses and
// events share the stream; responses carry "error", events carry "event".
type ipcEvent struct {
	Event  string      `json:"event"`
	ID     int         `json:"id"`
	Data   interface{} `json:"data"`
	Reason string      `json:"reason"`
}

// readLoop consumes events from the persistent connection until it closes.
func (p *Player) readLoop(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var ev ipcEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		switch ev.Event {
		case "property-change":
			p.handlePropertyChange(ev)
		case "end-file":
			// Only a natural end (or a mid-stream error, which would
			// otherwise stall the queue) advances playback. "stop" and
			// "quit" are user/track-switch initiated.
			if ev.Reason == "eof" || ev.Reason == "error" {
				p.mu.Lock()
				p.state = StateStopped
				p.mu.Unlock()
				p.emitEnded()
			}
		}
	}
}

func (p *Player) handlePropertyChange(ev ipcEvent) {
	switch ev.ID {
	case obsTimePos:
		pos, ok := ev.Data.(float64)
		if !ok {
			return // null while idle/loading
		}
		p.mu.Lock()
		dur := p.duration
		p.mu.Unlock()
		select {
		case p.posCh <- PositionUpdate{Position: pos, Duration: dur}:
		default:
		}
	case obsDuration:
		if dur, ok := ev.Data.(float64); ok {
			p.mu.Lock()
			p.duration = dur
			p.mu.Unlock()
		}
	case obsPause:
		if paused, ok := ev.Data.(bool); ok {
			p.mu.Lock()
			if p.state != StateStopped {
				if paused {
					p.state = StatePaused
				} else {
					p.state = StatePlaying
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *Player) emitEnded() {
	select {
	case p.endCh <- struct{}{}:
	default:
	}
}
