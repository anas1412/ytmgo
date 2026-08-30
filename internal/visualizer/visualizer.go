// Package visualizer renders a spectrum by delegating the hard part —
// audio capture, FFT and smoothing — to cava, which is packaged
// everywhere and already solves it well.
//
// cava is optional. When it isn't installed (or can't reach an audio
// source) Start fails and the caller simply doesn't offer the feature;
// nothing else in the app is affected.
//
// One caveat inherited from cava: it reads the system's audio monitor,
// so the bars react to whatever is playing on the machine, not only to
// ytmgo.
package visualizer

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// Frame is one snapshot of the spectrum: one value per bar, 0..100.
type Frame []int

// Visualizer runs a cava process and publishes frames.
type Visualizer struct {
	cmd     *exec.Cmd
	frames  chan Frame
	cfgPath string
	stderr  *bytes.Buffer

	mu      sync.Mutex
	latest  Frame
	closed  bool
	exitErr error
}

// Available reports whether cava is installed.
func Available() bool {
	_, err := exec.LookPath("cava")
	return err == nil
}

// Start launches cava configured to emit `bars` values per frame as
// plain text on stdout. Caller must Close when done.
func Start(bars int) (*Visualizer, error) {
	if !Available() {
		return nil, fmt.Errorf("cava is not installed")
	}
	if bars < 4 {
		bars = 4
	}
	if bars > 512 {
		bars = 512 // cava's own ceiling
	}
	// cava splits the spectrum across two channels and exits without a
	// message when the count is odd, so round down to even here rather
	// than leaving callers to discover it.
	bars -= bars % 2

	cfg, err := writeConfig(bars)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command("cava", "-p", cfg)
	// Keep stderr: when cava exits early (no audio server, bad config)
	// its message is the only clue, and a silent death would otherwise
	// leave the UI waiting for frames forever.
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		os.Remove(cfg)
		return nil, fmt.Errorf("cava stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		os.Remove(cfg)
		return nil, fmt.Errorf("cava start: %w", err)
	}

	v := &Visualizer{cmd: cmd, cfgPath: cfg, frames: make(chan Frame, 2), stderr: &errBuf}
	go v.read(stdout)
	// Record why cava stopped, so the caller can report it instead of
	// showing an empty spectrum indefinitely.
	go func() {
		err := cmd.Wait()
		v.mu.Lock()
		if !v.closed {
			msg := strings.TrimSpace(errBuf.String())
			switch {
			case msg != "":
				v.exitErr = fmt.Errorf("cava exited: %s", firstLine(msg))
			case err != nil:
				v.exitErr = fmt.Errorf("cava exited: %w", err)
			default:
				v.exitErr = fmt.Errorf("cava exited unexpectedly")
			}
		}
		v.mu.Unlock()
	}()
	return v, nil
}

// Err returns why cava stopped, or nil while it is still running.
func (v *Visualizer) Err() error {
	if v == nil {
		return nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.exitErr
}

// firstLine keeps error messages to one line for the status bar.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Frames returns the channel of spectrum frames.
func (v *Visualizer) Frames() <-chan Frame { return v.frames }

// Latest returns the most recent frame, for renderers that would rather
// poll than consume the channel.
func (v *Visualizer) Latest() Frame {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.latest
}

// Close stops cava and removes its temporary config.
func (v *Visualizer) Close() {
	if v == nil {
		return
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return
	}
	v.closed = true
	v.mu.Unlock()

	if v.cmd != nil && v.cmd.Process != nil {
		v.cmd.Process.Kill()
		v.cmd.Wait()
	}
	if v.cfgPath != "" {
		os.Remove(v.cfgPath)
	}
}

// read parses cava's ascii frames: values separated by ';', one line
// per frame. Frames are dropped rather than queued when the consumer is
// behind, so the display always shows current audio.
func (v *Visualizer) read(stdout interface{ Read([]byte) (int, error) }) {
	defer close(v.frames)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSuffix(scanner.Text(), ";"), ";")
		frame := make(Frame, 0, len(fields))
		for _, f := range fields {
			n, err := strconv.Atoi(strings.TrimSpace(f))
			if err != nil {
				continue
			}
			frame = append(frame, n)
		}
		if len(frame) == 0 {
			continue
		}
		v.mu.Lock()
		v.latest = frame
		v.mu.Unlock()
		select {
		case v.frames <- frame:
		default: // consumer is behind — drop, don't lag the audio
		}
	}
}

// writeConfig writes a minimal cava config emitting raw ascii frames.
func writeConfig(bars int) (string, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	path := filepath.Join(dir, fmt.Sprintf("ytmgo-cava-%d.conf", os.Getpid()))
	body := fmt.Sprintf(`[general]
framerate = 30
bars = %d
autosens = 1

[output]
method = raw
raw_target = /dev/stdout
data_format = ascii
ascii_max_range = 100
bar_delimiter = 59
frame_delimiter = 10

[smoothing]
noise_reduction = 30
`, bars)
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return "", fmt.Errorf("cava config: %w", err)
	}
	return path, nil
}
