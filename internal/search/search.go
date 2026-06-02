package search

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"ytmgo/internal/queue"
	"ytmgo/internal/ytdlp"
)

// ─── Result ───────────────────────────────────────────────────────────

// Result is a single search/recommendation result.
type Result struct {
	ID       string
	Title    string
	Uploader string
	Duration int // seconds
	URL      string
}

// ToTrack converts a search Result to a queue.Track.
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

// ─── Public API ───────────────────────────────────────────────────────

// Search runs yt-dlp to search YouTube and returns up to limit results.
// Uses the configured cookie browser / user-agent for personalized results.
func Search(query string, limit int, cookieBrowser, userAgent string) ([]Result, error) {
	searchQuery := fmt.Sprintf("ytsearch%d:%s", limit, query)
	args := []string{
		searchQuery,
		"--dump-json",
		"--flat-playlist",
		"--no-download",
		"--quiet",
		"--no-warnings",
	}
	if ca := ytdlp.CookiesArg(cookieBrowser); ca != "" {
		args = append(args, ca)
	}
	if ua := ytdlp.UserAgentArg(userAgent); ua != "" {
		args = append(args, ua)
	}

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp search failed: %w (is yt-dlp installed?)", err)
	}
	return parseYtdlpResults(out), nil
}

// FetchRecommendations runs yt-dlp with a "music" search query, filtered to
// tracks under 10 minutes. Returns up to limit results.
func FetchRecommendations(limit int, cookieBrowser, userAgent string) ([]Result, error) {
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
	if ca := ytdlp.CookiesArg(cookieBrowser); ca != "" {
		args = append(args, ca)
	}
	if ua := ytdlp.UserAgentArg(userAgent); ua != "" {
		args = append(args, ua)
	}

	cmd := exec.Command("yt-dlp", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp recommendations failed: %w", err)
	}
	return parseYtdlpResults(out), nil
}

// MockSearch returns fake results for dev mode (no yt-dlp required).
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

// ─── yt-dlp output parsing ────────────────────────────────────────────

// parseYtdlpResults parses yt-dlp's line-delimited JSON output into Result slices.
func parseYtdlpResults(data []byte) []Result {
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
			r.Duration = parseDurationYtdlp(v)
		}
	}
	return r
}

func parseDurationYtdlp(s string) int {
	parts := strings.Split(s, ":")
	total := 0
	for _, p := range parts {
		total *= 60
		n := 0
		for _, c := range p {
			if c >= '0' && c <= '9' {
				n = n*10 + int(c-'0')
			} else {
				break
			}
		}
		total += n
	}
	return total
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
