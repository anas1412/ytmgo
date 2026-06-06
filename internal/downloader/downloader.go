package downloader

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"ytmgo/internal/ytresolve"
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
	TrackID   string
	Title     string
	Uploader  string // artist name, used for yt-dlp search + output filename
	URL       string // optional YouTube URL (if known); empty = resolve via ytresolve
	OutDir    string
	CoverURL  string // TIDAL album art URL; empty = skip cover embedding
	Status    Status
	Progress  float64 // 0-100
	FilePath  string  // set when done
	Err       error
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
	mu       sync.Mutex
	jobs     []*Job
	jobCh    chan *Job
	progress chan ProgressEvent
	cancel   chan struct{}
	wg       sync.WaitGroup
	format   string // output format: "m4a" or "mp3"
}

// New creates a downloader. format is the output audio format ("m4a" or "mp3").
func New(outDir, format string) *Downloader {
	if format == "" {
		format = "m4a"
	}
	d := &Downloader{
		jobCh:    make(chan *Job, 100),
		progress: make(chan ProgressEvent, 200),
		cancel:   make(chan struct{}),
		format:   format,
	}
	go d.worker(outDir)
	return d
}

// SetFormat updates the output audio format for new downloads.
// Existing jobs in the queue are unaffected.
func (d *Downloader) SetFormat(format string) {
	if format != "" {
		d.mu.Lock()
		d.format = format
		d.mu.Unlock()
	}
}

// Progress returns the channel of progress events. Consumer must read it.
func (d *Downloader) Progress() <-chan ProgressEvent {
	return d.progress
}

// Enqueue adds a job. The URL is optional — if empty, the downloader resolves
// the YouTube URL from the Uploader (artist) and Title via yt-dlp search.
// If the file already exists on disk, it's marked done immediately.
// coverURL is a TIDAL album art URL; pass empty string to skip cover embedding.
func (d *Downloader) Enqueue(trackID, title, uploader, url, outDir, coverURL string) {
	ext := "." + d.format
	// Check if file already downloaded
	expected := filepath.Join(outDir, sanitizeFilename(uploader+" - "+title)+ext)
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
		CoverURL: coverURL,
		Status:   StatusPending,
	}
	d.mu.Lock()
	d.jobs = append(d.jobs, job)
	d.mu.Unlock()
	d.jobCh <- job
}

// IsDownloaded checks whether a file for this track already exists on disk.
// uploader+title must match the actual yt-dlp output format ({artist} - {title}).
func (d *Downloader) IsDownloaded(trackID, title, uploader, outDir string) bool {
	ext := "." + d.format
	// Check full name: {uploader} - {title}.{ext} (matches actual yt-dlp output)
	if uploader != "" && title != "" {
		expected := filepath.Join(outDir, sanitizeFilename(uploader+" - "+title)+ext)
		if _, err := os.Stat(expected); err == nil {
			return true
		}
	}
	// Fallback: scan directory for any file whose name contains the track ID
	// (catches files downloaded with a different title/artist combination).
	if fp := findExisting(outDir, trackID); fp != "" {
		return true
	}
	return false
}

// HasPendingJob checks if a job with the given trackID is already queued
// or currently downloading, preventing duplicate enqueues.
func (d *Downloader) HasPendingJob(trackID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, j := range d.jobs {
		if j.TrackID == trackID && (j.Status == StatusPending || j.Status == StatusDownloading) {
			return true
		}
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

// progressRe matches yt-dlp's download progress lines.
var progressRe = regexp.MustCompile(`\[download\]\s+(\d+\.\d+)%`)

func (d *Downloader) runJob(job *Job, outDir string) {
	job.Status = StatusDownloading

	// Ensure output directory exists
	if err := os.MkdirAll(outDir, 0755); err != nil {
		job.Status = StatusFailed
		job.Err = fmt.Errorf("create download dir: %w", err)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Status: StatusFailed, Err: job.Err}
		return
	}

	// Determine the YouTube URL
	videoURL := job.URL
	if videoURL == "" {
		result, err := ytresolve.Resolve(job.Uploader, job.Title)
		if err != nil {
			job.Status = StatusFailed
			job.Err = fmt.Errorf("ytresolve: %w", err)
			d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
			return
		}
		videoURL = result.WebpageURL
		if videoURL == "" {
			videoURL = result.URL
		}
	}
	if videoURL == "" {
		job.Status = StatusFailed
		job.Err = fmt.Errorf("no video URL for %s - %s", job.Uploader, job.Title)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
		return
	}

	// Output filename pattern: {Artist} - {Title}.%(ext)s
	outPattern := filepath.Join(outDir, sanitizeFilename(job.Uploader+" - "+job.Title)+".%(ext)s")

	// yt-dlp command: extract audio in the selected format.
	// NOTE: we deliberately do NOT pass --print here — in some yt-dlp
	// versions that flag suppresses the actual download (behaves like
	// --get-filename). We know the output path from the template, so we
	// construct it ourselves after yt-dlp completes.
	args := []string{
		"-x",                          // extract audio
		"--audio-format", d.format,    // output format: m4a or mp3
		"--audio-quality", "0",        // best quality
		"--embed-thumbnail",           // embed YouTube thumbnail as cover art
		"--output", outPattern,
		"--no-playlist",
		videoURL,
	}

	cmd := exec.Command("yt-dlp", args...)

	// Capture stderr for progress parsing
	stderr, err := cmd.StderrPipe()
	if err != nil {
		job.Status = StatusFailed
		job.Err = fmt.Errorf("yt-dlp stderr pipe: %w", err)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
		return
	}

	if err := cmd.Start(); err != nil {
		job.Status = StatusFailed
		job.Err = fmt.Errorf("yt-dlp start: %w", err)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
		return
	}

	// Read stderr line by line to track progress
	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if m := progressRe.FindStringSubmatch(line); len(m) > 1 {
				var pct float64
				fmt.Sscanf(m[1], "%f", &pct)
				job.Progress = pct
				d.progress <- ProgressEvent{
					TrackID:  job.TrackID,
					Title:    job.Title,
					Uploader: job.Uploader,
					Progress: pct,
					Status:   StatusDownloading,
				}
			}
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		<-progressDone
		job.Status = StatusFailed
		job.Err = fmt.Errorf("yt-dlp failed: %w", err)
		d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
		return
	}
	<-progressDone

	// The output path is known from the template. Verify it exists so we
	// surface an error if yt-dlp put the file somewhere unexpected.
	outPath := filepath.Join(outDir, sanitizeFilename(job.Uploader+" - "+job.Title)+"."+d.format)
	if _, err := os.Stat(outPath); err != nil {
		// File not found — yt-dlp may have used a different extension
		// (e.g. opus → m4a remux gives .opus on some versions). Scan
		// the output directory for any new file matching the title.
		outPath = ""
		entries, _ := os.ReadDir(outDir)
		titleBase := sanitizeFilename(job.Uploader + " - " + job.Title)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), titleBase) && !e.IsDir() {
				outPath = filepath.Join(outDir, e.Name())
				break
			}
		}
		if outPath == "" {
			// Still nothing — emit a failed status so the user
			// gets a clear error instead of a silent empty folder.
			job.Status = StatusFailed
			job.Err = fmt.Errorf("yt-dlp completed but no output file found for %s - %s (expected: %s.*)",
				job.Uploader, job.Title, titleBase)
			d.progress <- ProgressEvent{TrackID: job.TrackID, Title: job.Title, Uploader: job.Uploader, Status: StatusFailed, Err: job.Err}
			return
		}
	}

	// ── Embed TIDAL cover art ──────────────────────────────────────
	// Download the album art from TIDAL and embed it into the audio
	// file using ffmpeg. Non-fatal — if it fails we still report the
	// download as completed (the audio file is already on disk).
	if job.CoverURL != "" {
		if err := embedCoverArt(outPath, job.CoverURL); err != nil {
			// Log to stderr; user can re-embed later if they want.
			fmt.Fprintf(os.Stderr, "ytmgo: cover art embed failed for %s - %s: %v\n",
				job.Uploader, job.Title, err)
		}
	}

	job.Status = StatusDone
	job.Progress = 100
	job.FilePath = outPath
	d.progress <- ProgressEvent{
		TrackID:  job.TrackID,
		Title:    job.Title,
		Uploader: job.Uploader,
		Progress: 100,
		Status:   StatusDone,
		FilePath: job.FilePath,
	}
}

func sanitizeFilename(s string) string {
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

// embedCoverArt downloads a cover image from coverURL and embeds it into
// the audio file at audioPath using ffmpeg. Requires ffmpeg in PATH
// (already needed by yt-dlp for audio extraction).
func embedCoverArt(audioPath, coverURL string) error {
	// Download the cover image to a temp file
	resp, err := http.Get(coverURL) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download cover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download cover: HTTP %d", resp.StatusCode)
	}

	coverTmp, err := os.CreateTemp("", "ytmgo-cover-*.jpg")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	defer os.Remove(coverTmp.Name())

	if _, err := io.Copy(coverTmp, resp.Body); err != nil {
		coverTmp.Close()
		return fmt.Errorf("write cover: %w", err)
	}
	coverTmp.Close()

	// Embed with ffmpeg: copy audio stream + add cover as attached pic
	audioTmp, err := os.CreateTemp(filepath.Dir(audioPath), ".ytmgo-embed-*"+filepath.Ext(audioPath))
	if err != nil {
		return fmt.Errorf("create temp audio: %w", err)
	}
	tmpName := audioTmp.Name()
	audioTmp.Close()

	cmd := exec.Command("ffmpeg",
		"-i", audioPath,
		"-i", coverTmp.Name(),
		"-map", "0:a",          // audio from first input
		"-map", "1:v",          // cover from second input
		"-c", "copy",            // copy both streams without re-encode
		"-disposition:v:0", "attached_pic",
		"-y",                    // overwrite tmp output
		tmpName,
	)
	if err := cmd.Run(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("ffmpeg embed: %w", err)
	}

	// Replace original with the cover-embedded version
	if err := os.Rename(tmpName, audioPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}
