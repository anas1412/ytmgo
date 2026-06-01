package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
	"ytmgo/internal/innertube"
	"ytmgo/internal/queue"
	"ytmgo/internal/ytdlp"
)

// ─── Result ───────────────────────────────────────────────────────────

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

// ─── InnerTube client (lazy singleton) ────────────────────────────────

var (
	itubeOnce sync.Once
	itube     *innertube.Client
)

func innertubeClient() *innertube.Client {
	itubeOnce.Do(func() {
		itube = innertube.NewClient()
	})
	return itube
}

// ─── Public API ───────────────────────────────────────────────────────

// Search performs a YouTube Music search using the InnerTube API (fast).
// Falls back to yt-dlp if the API call fails.
func Search(query string, limit int, cookieBrowser, userAgent string) ([]Result, error) {
	// Fast path: InnerTube API.
	results, err := searchViaInnertube(query, limit)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fallback: yt-dlp subprocess.
	return searchViaYtdlp(query, limit, cookieBrowser, userAgent)
}

// FetchRecommendations fetches music recommendations via InnerTube API.
// Falls back to yt-dlp if the API call fails.
func FetchRecommendations(limit int, cookieBrowser, userAgent string) ([]Result, error) {
	// Fast path: InnerTube API.
	results, err := recommendationsViaInnertube(limit)
	if err == nil && len(results) > 0 {
		return results, nil
	}

	// Fallback: yt-dlp subprocess.
	return recommendationsViaYtdlp(limit, cookieBrowser, userAgent)
}

// StreamRecommendations fetches recommendations and streams them to ch
// one at a time as each result is decoded from the HTTP response body.
// Each result appears in the UI promptly instead of arriving in a batch.
func StreamRecommendations(limit int, ch chan<- Result, done <-chan struct{}, cookieBrowser, userAgent string) {
	defer close(ch)

	// Fast path: innerTube streaming (sends results incrementally as JSON
	// is decoded off the wire rather than waiting for the full response).
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-done:
			cancel()
		case <-ctx.Done():
		}
	}()

	itubeCh := make(chan innertube.Result)
	go func() {
		defer close(itubeCh)
		err := innertubeClient().SearchStream(ctx, "music", limit, itubeCh)
		if err != nil {
			return
		}
	}()

	// Drain the streaming channel, converting and re-sending to ch.
	// Stop on explicit done signal or stream exhaustion.
	for r := range itubeCh {
		select {
		case ch <- convertResult(r):
		case <-done:
			cancel()
			return
		}
	}

	// Streaming finished (or never started). Cancel the context so the
	// monitor goroutine can exit, then check if we were cancelled.
	cancel()
	select {
	case <-done:
		return
	default:
	}

	results, err := recommendationsViaYtdlp(limit, cookieBrowser, userAgent)
	if err != nil {
		return
	}
	for _, r := range results {
		select {
		case ch <- r:
		case <-done:
			return
		}
	}
}

// convertResult converts an innertube.Result to a search.Result.
func convertResult(r innertube.Result) Result {
	return Result{
		ID:       r.ID,
		Title:    r.Title,
		Uploader: r.Uploader,
		Duration: r.Duration,
		URL:      r.URL,
	}
}

// MockSearch returns fake results for testing.
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

// ─── InnerTube backends ───────────────────────────────────────────────

func searchViaInnertube(query string, limit int) ([]Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := innertubeClient().Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	return convertResults(raw), nil
}

func recommendationsViaInnertube(limit int) ([]Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, err := innertubeClient().Home(ctx, limit)
	if err != nil {
		return nil, err
	}
	return convertResults(raw), nil
}

func convertResults(raw []innertube.Result) []Result {
	if raw == nil {
		return nil
	}
	results := make([]Result, len(raw))
	for i, r := range raw {
		results[i] = Result{
			ID:       r.ID,
			Title:    r.Title,
			Uploader: r.Uploader,
			Duration: r.Duration,
			URL:      r.URL,
		}
	}
	return results
}

// ─── yt-dlp fallbacks ─────────────────────────────────────────────────

func searchViaYtdlp(query string, limit int, cookieBrowser, userAgent string) ([]Result, error) {
	searchQuery := fmt.Sprintf("ytsearch%d:%s", limit, query)
	args := []string{searchQuery,
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

func recommendationsViaYtdlp(limit int, cookieBrowser, userAgent string) ([]Result, error) {
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

// parseYtdlpResults parses yt-dlp JSON lines into Result slices.
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
