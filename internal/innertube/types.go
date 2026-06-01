// Package innertube implements a Go client for YouTube Music's InnerTube API.
// It provides search and browse (home/recommendations) endpoints without
// requiring yt-dlp or any external dependencies — just stdlib net/http.
package innertube

import "time"

// Result is a single search or recommendation result.
type Result struct {
	ID       string // YouTube video ID
	Title    string
	Uploader string
	Duration int    // seconds
	URL      string // full watch URL
}

// ─── API Constants ────────────────────────────────────────────────────

const (
	baseURL   = "https://music.youtube.com/youtubei/v1"
	apiKey    = "AIzaSyC9XL3ZjWddXya6X74dJoCTL-WEYFDNX30"
	clientName    = "WEB_REMIX"
	clientVersion = "1.20231204.01.00"
	defaultHL     = "en"
	defaultGL     = "US"
)

// ─── HTTP Defaults ────────────────────────────────────────────────────

const (
	defaultTimeout = 15 * time.Second
	maxIdleConns   = 4
)


