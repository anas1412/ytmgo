package player

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"
)

const socketPath = "/tmp/ytmgo_mpv.sock"

type State int

const (
	StateStopped State = iota
	StatePlaying
	StatePaused
)

// PositionUpdate is sent periodically from the mpv poller
type PositionUpdate struct {
	Position float64 // seconds
	Duration float64 // seconds
}

// Player controls a single mpv instance
type Player struct {
	mu         sync.Mutex
	cmd        *exec.Cmd
	state      State
	volume     int
	socketPath string
	posCh      chan PositionUpdate
	endCh      chan struct{} // closed when mpv exits naturally
	stopPoll   chan struct{}
}

func New() *Player {
	return &Player{
		volume:     80,
		socketPath: socketPath,
		posCh:      make(chan PositionUpdate, 10),
		endCh:      make(chan struct{}, 1),
	}
}

// Positions returns the channel of position updates
func (p *Player) Positions() <-chan PositionUpdate {
	return p.posCh
}

// Ended returns a channel that receives when the track ends naturally
func (p *Player) Ended() <-chan struct{} {
	return p.endCh
}

// Play starts a new track. Kills any existing mpv first.
func (p *Player) Play(filePath string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Kill existing process safely
	p.stopInternal()

	// Remove stale socket
	os.Remove(p.socketPath)

	// Create fresh end channel
	p.endCh = make(chan struct{}, 1)

	cmd := exec.Command("mpv",
			    "--no-video",
		     "--audio-display=no",
		     fmt.Sprintf("--volume=%d", p.volume),
			    fmt.Sprintf("--input-ipc-server=%s", p.socketPath),
			    "--quiet",
		     "--really-quiet",
		     filePath,
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("mpv failed to start: %w (is mpv installed?)", err)
	}

	p.cmd = cmd
	p.state = StatePlaying
	p.stopPoll = make(chan struct{})

	// Watch for mpv exit
	endCh := p.endCh
	stopPoll := p.stopPoll
	go func() {
		cmd.Wait()
		select {
			case endCh <- struct{}{}:
			default:
		}
	}()

	// Start polling position via IPC
	go p.pollPosition(stopPoll)

	return nil
}

// Pause toggles pause. Returns true on success, false if the IPC command
// failed (mpv didn't actually pause/resume).
func (p *Player) Pause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.state == StatePlaying {
		if err := p.sendCommand([]interface{}{"set_property", "pause", true}); err != nil {
			return false
		}
		p.state = StatePaused
		return true
	} else if p.state == StatePaused {
		if err := p.sendCommand([]interface{}{"set_property", "pause", false}); err != nil {
			return false
		}
		p.state = StatePlaying
		return true
	}
	return false
}

// Stop kills mpv completely
func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopInternal()
}

// Seek seeks by delta seconds (can be negative). Errors are silently
// ignored since seeking is best-effort.
func (p *Player) Seek(delta float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_ = p.sendCommand([]interface{}{"seek", delta, "relative"})
}

// SetVolume sets volume 0-100
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
	_ = p.sendCommand([]interface{}{"set_property", "volume", v})
}

// Volume returns current volume
func (p *Player) Volume() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.volume
}

// State returns current player state
func (p *Player) State() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state
}

// stopInternal kills mpv. Caller must hold p.mu.
func (p *Player) stopInternal() {
	if p.stopPoll != nil {
		select {
			case <-p.stopPoll: // already closed
			default:
				close(p.stopPoll)
		}
		p.stopPoll = nil
	}
	if p.cmd != nil && p.cmd.Process != nil {
		p.cmd.Process.Kill()
		p.cmd = nil
	}
	p.state = StateStopped
	os.Remove(p.socketPath)
}

func (p *Player) pollPosition(stop <-chan struct{}) {
	// Wait for socket to appear
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(p.socketPath); err == nil {
			break
		}
		select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
			case <-stop:
				return
			case <-ticker.C:
				pos := p.getProperty("time-pos")
				dur := p.getProperty("duration")
				if pos >= 0 {
					select {
			case p.posCh <- PositionUpdate{Position: pos, Duration: dur}:
			default:
					}
				}
		}
	}
}

func (p *Player) sendCommand(args []interface{}) error {
	msg := map[string]interface{}{"command": args}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("sendCommand marshal: %w", err)
	}
	data = append(data, '\n')

	conn, err := net.DialTimeout("unix", p.socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("sendCommand dial: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("sendCommand write: %w", err)
	}
	return nil
}

func (p *Player) getProperty(prop string) float64 {
	msg := map[string]interface{}{
		"command":    []interface{}{"get_property", prop},
		"request_id": 1,
	}
	data, _ := json.Marshal(msg)
	data = append(data, '\n')

	conn, err := net.DialTimeout("unix", p.socketPath, 200*time.Millisecond)
	if err != nil {
		return -1
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(300 * time.Millisecond))
	conn.Write(data)

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var resp struct {
			Data  interface{} `json:"data"`
			Error string      `json:"error"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &resp); err == nil {
			if resp.Error == "success" {
				if v, ok := resp.Data.(float64); ok {
					return v
				}
			}
		}
	}
	return -1
}
