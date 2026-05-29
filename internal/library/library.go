package library

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ytmgo/internal/queue"
)

// Track is an alias so library tracks are compatible with the queue.
type Track = queue.Track

// ScanDir scans a directory for audio files and extracts metadata.
// Each file's duration is read via ffprobe; title/artist are parsed from
// the filename (since yt-dlp doesn't embed tags by default without
// --embed-metadata, and existing files won't have them retroactively).
func ScanDir(dir string) ([]Track, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Track{}, nil
		}
		return nil, fmt.Errorf("reading library dir: %w", err)
	}

	var tracks []Track
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".mp3" && ext != ".m4a" && ext != ".flac" && ext != ".ogg" && ext != ".wav" {
			continue
		}

		fpath := filepath.Join(dir, e.Name())
		duration := probeDuration(fpath)
		title, artist := parseFilename(e.Name())

		tracks = append(tracks, Track{
			ID:          fpath,
			Title:       title,
			Artist:      artist,
			Duration:    formatDuration(duration),
			DurationSec: duration,
			FilePath:    fpath,
			Downloaded:  true,
		})
	}
	return tracks, nil
}

// probeDuration returns the duration in seconds using ffprobe.
func probeDuration(path string) int {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_entries", "format=duration",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0
	}
	if result.Format.Duration == "" {
		return 0
	}
	var secs float64
	if _, err := fmt.Sscanf(result.Format.Duration, "%f", &secs); err != nil {
		return 0
	}
	return int(secs)
}

// parseFilename tries to extract "Artist - Title" from a filename.
// Falls back to using the whole stem as the title.
func parseFilename(name string) (title, artist string) {
	stem := strings.TrimSuffix(name, filepath.Ext(name))

	// Try "Artist - Title" pattern (most common)
	if idx := strings.Index(stem, " - "); idx > 0 {
		artist = strings.TrimSpace(stem[:idx])
		title = strings.TrimSpace(stem[idx+3:])
		// Clean up common suffixes like (Official Video), (Lyrics), etc.
		title = cleanTitle(title)
		return title, artist
	}

	// Fallback: whole name is the title
	title = cleanTitle(stem)
	return title, ""
}

// cleanTitle removes common suffixes from video titles.
func cleanTitle(t string) string {
	suffixes := []string{
		"(Official Music Video)",
		"(Official Video)",
		"(Official Lyric Video)",
		"(Lyric Video)",
		"(Lyrics)",
		"(Audio)",
		"(Official Audio)",
		"[Official Music Video]",
		"[Official Video]",
		"[Lyrics]",
		"|",
	}
	for _, s := range suffixes {
		if idx := strings.Index(t, s); idx >= 0 {
			t = strings.TrimSpace(t[:idx])
		}
	}
	return t
}

func formatDuration(secs int) string {
	if secs <= 0 {
		return "0:00"
	}
	m := secs / 60
	s := secs % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
