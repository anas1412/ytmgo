package search

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"ytmgo/internal/queue"
)

// Result is a search result
type Result struct {
	ID       string
	Title    string
	Uploader string
	Duration int // seconds
	URL      string
}

func (r Result) ToTrack() queue.Track {
	return queue.Track{
		ID:          r.ID,
		Title:       r.Title,
		Artist:      r.Uploader,
		Duration:    formatDuration(r.Duration),
		DurationSec: r.Duration,
		URL:         r.URL,
	}
}

// Search performs a YouTube search using yt-dlp and returns up to `limit` results
func Search(query string, limit int) ([]Result, error) {
	// yt-dlp "ytsearch5:query" --dump-json --no-download --flat-playlist --quiet
	searchQuery := fmt.Sprintf("ytsearch%d:%s", limit, query)
	args := []string{searchQuery,
		"--dump-json",
		"--flat-playlist",
		"--no-download",
		"--quiet",
		"--no-warnings",
	}
	if ca := cookiesFromBrowserArg(); ca != "" {
		args = append(args, ca)
	}
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		// try fallback: maybe yt-dlp is not installed
		return nil, fmt.Errorf("yt-dlp search failed: %w (is yt-dlp installed?)", err)
	}

	return parseResults(out), nil
}

// FetchRecommendations fetches music recommendations from YouTube search.
// Cookies provide personalization. Results are filtered to individual songs
// (under 10 minutes) — no mixes, compilations, or playlists.
func FetchRecommendations(limit int) ([]Result, error) {
	// Use ytsearch (YouTube search) instead of the home page so we get
	// music-categorized results with uploader metadata. Then filter out
	// compilations/mixes by duration (< 10 min = individual song).
	query := fmt.Sprintf("ytsearch%d:music", limit)
	args := []string{
		query,
		"--no-playlist",
		"--match-filter", "duration < 600",
		"--flat-playlist",
		"--dump-json",
		"--no-download",
		"--quiet",
		"--no-warnings",
	}
	if ca := cookiesFromBrowserArg(); ca != "" {
		args = append(args, ca)
	}
	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp recommendations failed: %w", err)
	}
	return parseResults(out), nil
}

// StreamRecommendations runs yt-dlp and sends all parsed results to `ch`.
// Results arrive in one batch (ytsearch returns everything at once).
func StreamRecommendations(limit int, ch chan<- Result, done <-chan struct{}) {
	defer close(ch)

	query := fmt.Sprintf("ytsearch%d:music", limit)
	args := []string{
		query,
		"--no-playlist",
		"--match-filter", "duration < 600",
		"--flat-playlist",
		"--dump-json",
		"--no-download",
		"--quiet",
		"--no-warnings",
	}
	if ca := cookiesFromBrowserArg(); ca != "" {
		args = append(args, ca)
	}

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return
	}

	for _, r := range parseResults(out) {
		select {
		case ch <- r:
		case <-done:
			return
		}
	}
}

// parseEntry extracts a Result from a yt-dlp JSON map entry.
func parseEntry(entry map[string]interface{}) Result {
	var r Result
	if id, ok := entry["id"].(string); ok {
		r.ID = id
		r.URL = "https://www.youtube.com/watch?v=" + id
	}
	if title, ok := entry["title"].(string); ok {
		r.Title = title
	}
	if up, ok := entry["uploader"].(string); ok {
		r.Uploader = up
	} else if ch, ok := entry["channel"].(string); ok {
		r.Uploader = ch
	}
	if dur, ok := entry["duration"]; ok {
		switch v := dur.(type) {
		case float64:
			r.Duration = int(v)
		case string:
			r.Duration = parseDuration(v)
		}
	}
	return r
}

// parseResults parses yt-dlp JSON lines into Result slices.
func parseResults(data []byte) []Result {
	var results []Result
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		r := parseEntry(entry)
		if r.ID != "" && r.Title != "" {
			results = append(results, r)
		}
	}
	return results
}

// MockSearch returns fake results for testing when yt-dlp is unavailable
func MockSearch(query string, limit int) []Result {
	songs := []struct{ title, artist string }{
		{"Bohemian Rhapsody", "Queen"},
		{"Hotel California", "Eagles"},
		{"Stairway to Heaven", "Led Zeppelin"},
		{"Smells Like Teen Spirit", "Nirvana"},
		{"Imagine", "John Lennon"},
		{"Purple Rain", "Prince"},
		{"Like a Rolling Stone", "Bob Dylan"},
		{"Hey Jude", "The Beatles"},
		{"What's Going On", "Marvin Gaye"},
		{"Respect", "Aretha Franklin"},
	}
	var results []Result
	for i, s := range songs {
		if i >= limit {
			break
		}
		id := fmt.Sprintf("mock_%d_%d", i, time.Now().UnixNano())
		results = append(results, Result{
			ID:       id,
			Title:    fmt.Sprintf("%s (matching: %s)", s.title, query),
				 Uploader: s.artist,
				 Duration: 180 + i*30,
				 URL:      "https://youtube.com/watch?v=" + id,
		})
	}
	return results
}

func formatDuration(secs int) string {
	d := time.Duration(secs) * time.Second
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// cookiesFromBrowserArg returns a --cookies-from-browser flag for yt-dlp
// if a supported browser config is found, or empty string otherwise.
func cookiesFromBrowserArg() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Origin-Nightly"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Nightly"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser"),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return "--cookies-from-browser=brave:" + p
		}
	}
	return ""
}

func parseDuration(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		total *= 60
		n, _ := strconv.Atoi(p)
		total += n
	}
	return total
}
