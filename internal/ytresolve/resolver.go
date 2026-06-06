// Package ytresolve resolves YouTube video URLs from artist + title metadata
// using yt-dlp's search functionality. It is the bridge between TIDAL metadata
// (artist, title) and YouTube streaming URLs for mpv playback.
package ytresolve

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Result is a single YouTube search result from yt-dlp's --dump-json output.
type Result struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	WebpageURL string  `json:"webpage_url"`
	Duration   float64 `json:"duration"`
	Channel    string  `json:"channel"`
}

// Resolve searches YouTube for the given artist and title, and returns the
// first matching video's Result.
//
// It runs: yt-dlp --flat-playlist --dump-json "ytsearch1:{artist} - {title}"
//
// --flat-playlist avoids extracting full video info (fast metadata-only),
// --dump-json prints JSON to stdout for parsing.
func Resolve(artist, title string) (*Result, error) {
	query := fmt.Sprintf("%s - %s", artist, strings.TrimSpace(title))
	args := []string{
		"--flat-playlist",
		"--dump-json",
		fmt.Sprintf("ytsearch1:%s", query),
	}

	cmd := exec.Command("yt-dlp", args...)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("yt-dlp search failed: %s (stderr: %s)", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("yt-dlp search failed: %w", err)
	}

	var result Result
	if err := json.Unmarshal(stdout, &result); err != nil {
		return nil, fmt.Errorf("yt-dlp parse error: %w", err)
	}

	if result.WebpageURL == "" && result.URL != "" {
		result.WebpageURL = result.URL
	}

	return &result, nil
}

// ResolveURL searches YouTube for the given artist and title, and returns the
// first matching video's webpage URL. This is a convenience wrapper around Resolve.
func ResolveURL(artist, title string) (string, error) {
	r, err := Resolve(artist, title)
	if err != nil {
		return "", err
	}
	if r.WebpageURL == "" {
		return "", fmt.Errorf("yt-dlp returned empty URL for %s - %s", artist, title)
	}
	return r.WebpageURL, nil
}
