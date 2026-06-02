package downloader

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"ytmgo/internal/ytdlp"
)

// Status of a download job
type Status int

const (
	StatusPending    Status = iota
	StatusDownloading
	StatusDone
	StatusFailed
	StatusSkipped // already on disk
)

// Job is a single download request
type Job struct {
	TrackID  string
	Title    string
	Uploader string // channel/artist name, used by the TUI library view
	URL      string
	OutDir   string
	Status   Status
	Progress float64 // 0-100
	FilePath string  // set when done
	Err      error
}

// ProgressEvent is sent to the progress channel
type ProgressEvent struct {
	TrackID  string
	Title    string
	Uploader string
	Progress float64
	Status   Status
	FilePath string
	Err      error
}

// Downloader serializes downloads so only one runs at a time
type Downloader struct {
	mu            sync.Mutex
	jobs          []*Job
	jobCh         chan *Job
	progress      chan ProgressEvent
	cancel        chan struct{}
	wg            sync.WaitGroup
	cookieBrowser string
	userAgent     string
}

var progressRe = regexp.MustCompile(`\[download\]\s+([\d.]+)%`)

func New(outDir, cookieBrowser, userAgent string) *Downloader {
	d := &Downloader{
		jobCh:         make(chan *Job, 100),
		progress:      make(chan ProgressEvent, 200),
		cancel:        make(chan struct{}),
		cookieBrowser: cookieBrowser,
		userAgent:     userAgent,
	}
	go d.worker(outDir)
	return d
}

// Progress returns the channel of progress events. Consumer must read it.
func (d *Downloader) Progress() <-chan ProgressEvent {
	return d.progress
}

// Enqueue adds a job. If the file already exists on disk, it's marked done immediately.
func (d *Downloader) Enqueue(trackID, title, uploader, url, outDir string) {
	// Check if file already downloaded
	expected := filepath.Join(outDir, SanitizeFilename(title)+".mp3")
	if _, err := os.Stat(expected); err == nil {
		d.progress <- ProgressEvent{
			TrackID:  trackID,
			Title:    title,
			Uploader: uploader,
			Progress: 100,
			Status:   StatusSkipped,
			FilePath: expected,
		}
		return
	}
	// Also check by scanning dir for any file containing the track ID
	if fp := findExisting(outDir, trackID); fp != "" {
		d.progress <- ProgressEvent{
			TrackID:  trackID,
			Title:    title,
			Uploader: uploader,
			Progress: 100,
			Status:   StatusSkipped,
			FilePath: fp,
		}
		return
	}

	job := &Job{
		TrackID:  trackID,
		Title:    title,
		Uploader: uploader,
		URL:      url,
		OutDir:   outDir,
		Status:   StatusPending,
	}
	d.mu.Lock()
	d.jobs = append(d.jobs, job)
	d.mu.Unlock()
	d.jobCh <- job
}

// IsDownloaded checks whether a file for this track already exists on disk.
// This mirrors the same check at the top of Enqueue so callers can inspect
// the result without consuming a channel event.
func (d *Downloader) IsDownloaded(trackID, title, outDir string) bool {
	expected := filepath.Join(outDir, SanitizeFilename(title)+".mp3")
	if _, err := os.Stat(expected); err == nil {
		return true
	}
	if fp := findExisting(outDir, trackID); fp != "" {
		return true
	}
	return false
}

// Jobs returns a snapshot of all jobs
func (d *Downloader) Jobs() []*Job {
	d.mu.Lock()
	defer d.mu.Unlock()
	cp := make([]*Job, len(d.jobs))
	copy(cp, d.jobs)
	return cp
}

// Close shuts down the downloader
func (d *Downloader) Close() {
	close(d.cancel)
}

func (d *Downloader) worker(outDir string) {
	for {
		select {
			case <-d.cancel:
				return
			case job := <-d.jobCh:
				d.runJob(job, outDir)
		}
	}
}

func (d *Downloader) runJob(job *Job, outDir string) {
	// Mark as downloading locally so Jobs() reflects the new state
	// on the next view re-render, but DON'T emit a synthetic
	// Progress: 0 event here — doing so makes the view show "0%"
	// for as long as it takes yt-dlp to emit its first real
	// progress line (and forever, if the regex never matches due
	// to ANSI escapes or a different yt-dlp output format). The
	// first genuine progress event will be emitted by the stdout
	// parser below, which is the only signal worth showing.
	job.Status = StatusDownloading

	// Output template: use title sanitized
	outTemplate := filepath.Join(outDir, "%(title)s.%(ext)s")

	args := []string{
		"--extract-audio",
		"--audio-format", "mp3",
		"--audio-quality", "0",
		"--embed-metadata",
		"--embed-thumbnail",
		"--output", outTemplate,
		"--newline",
		// --no-color is essential: when yt-dlp detects a TTY (which
		// it always does inside this TUI), it emits ANSI color
		// escapes in front of [download] progress lines, breaking
		// progressRe. Without this, the progress bar stays at 0%.
		"--no-color",
		"--no-playlist",
		"--no-warnings",
	}
	if ca := ytdlp.CookiesArg(d.cookieBrowser); ca != "" {
		args = append(args, ca)
	}
	if ua := ytdlp.UserAgentArg(d.userAgent); ua != "" {
		args = append(args, ua)
	}
	args = append(args, job.URL)
	cmd := exec.Command("yt-dlp", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.Status = StatusFailed
		job.Err = err
		d.progress <- ProgressEvent{TrackID: job.TrackID, Status: StatusFailed, Err: err}
		return
	}
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		job.Status = StatusFailed
		job.Err = fmt.Errorf("yt-dlp not found or failed to start: %w", err)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Status: StatusFailed, Err: job.Err}
		return
	}

	// Kill the subprocess if the downloader is cancelled mid-flight
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-d.cancel:
			if cmd.Process != nil {
				cmd.Process.Kill()
			}
		case <-done:
		}
	}()

	// Drain stderr in background
	go io.Copy(io.Discard, stderr)

	// Parse progress from stdout
	var lastPct float64
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if m := progressRe.FindStringSubmatch(line); len(m) == 2 {
			pct, _ := strconv.ParseFloat(m[1], 64)
			if pct != lastPct {
				lastPct = pct
				job.Progress = pct
				d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Progress: pct, Status: StatusDownloading}
			}
		}
		// detect destination file
		if strings.Contains(line, "[ExtractAudio] Destination:") {
			parts := strings.SplitN(line, "Destination:", 2)
			if len(parts) == 2 {
				job.FilePath = strings.TrimSpace(parts[1])
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		job.Status = StatusFailed
		job.Err = err
		d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: err}
		return
	}

	// If we didn't capture the file path above, scan the output dir
	if job.FilePath == "" {
		job.FilePath = findExisting(outDir, job.TrackID)
		if job.FilePath == "" {
			// best guess by title
			job.FilePath = filepath.Join(outDir, SanitizeFilename(job.Title)+".mp3")
		}
	}

	job.Status = StatusDone
	job.Progress = 100
	d.progress <- ProgressEvent{
		TrackID:  job.TrackID,
		Title:    job.Title,
		Uploader: job.Uploader,
		Progress: 100,
		Status:   StatusDone,
		FilePath: job.FilePath,
	}
}

// SanitizeFilename replaces filesystem-unsafe characters with underscores.
func SanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
			case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
				b.WriteRune('_')
			default:
				b.WriteRune(r)
		}
	}
	return b.String()
}

func findExisting(dir, trackID string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), trackID) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}
