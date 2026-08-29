// Package ytmusic is a minimal client for YouTube Music's InnerTube API,
// the same private API the music.youtube.com web app calls. It needs no
// API key, OAuth token, or cookies: requests are plain HTTPS POSTs
// carrying a WEB_REMIX client context, exactly like an anonymous browser
// tab. (The official "YouTube Data API v3" is the one that needs keys;
// this is not that.)
//
// Search returns real videoIds, so playback and downloads address the
// exact recording the user picked instead of fuzzy-matching an
// "artist - title" search later. Radio returns the queue YouTube Music
// itself would autoplay after a track, via the deterministic
// RDAMVM<videoId> radio playlist.
//
// Caveat: Google bot-walls datacenter IPs. From residential connections
// (this app's use case) anonymous access works; failures degrade
// gracefully to the yt-dlp fallback paths.
package ytmusic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	baseURL = "https://music.youtube.com/youtubei/v1"
	// clientVersion is refreshed occasionally; old versions keep working
	// for a long time.
	clientVersion = "1.20250310.01.00"
	// songsFilterParams restricts /search to the Songs shelf
	// (protobuf filter blob, verified against the live endpoint).
	songsFilterParams = "EgWKAQIIAWoQEAMQBBAJEAoQBRAREBAQFQ%3D%3D"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Track is one song from search or radio results.
type Track struct {
	VideoID  string
	Title    string
	Artist   string
	Album    string
	Duration int // seconds
	CoverURL string
}

// WatchURL returns the playable URL for a videoId. mpv resolves it via
// its yt-dlp hook; yt-dlp downloads it directly.
func WatchURL(videoID string) string {
	return "https://music.youtube.com/watch?v=" + videoID
}

var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// IsVideoID reports whether s looks like a YouTube videoId. Used to
// tell new-style track IDs apart from legacy TIDAL numeric IDs and
// library file paths.
func IsVideoID(s string) bool {
	return videoIDRe.MatchString(s)
}

// Search runs a songs-filtered YouTube Music search.
func Search(query string, limit int) ([]Track, error) {
	body := map[string]interface{}{
		"context": clientContext(),
		"query":   query,
		"params":  songsFilterParams,
	}
	root, err := post("search", body)
	if err != nil {
		return nil, err
	}

	shelf := findKey(root, "musicShelfRenderer")
	if shelf == nil {
		return nil, fmt.Errorf("ytmusic search: no results shelf in response")
	}
	contents, _ := dig(shelf, "contents").([]interface{})

	var tracks []Track
	for _, c := range contents {
		item := dig(c, "musicResponsiveListItemRenderer")
		if item == nil {
			continue
		}
		t, ok := parseSearchItem(item)
		if !ok {
			continue
		}
		tracks = append(tracks, t)
		if limit > 0 && len(tracks) >= limit {
			break
		}
	}
	return tracks, nil
}

// Radio returns the autoplay queue seeded from a track: what YouTube
// Music would play next. The seed track itself is excluded.
func Radio(videoID string, limit int) ([]Track, error) {
	body := map[string]interface{}{
		"context":                       clientContext(),
		"videoId":                       videoID,
		"playlistId":                    "RDAMVM" + videoID,
		"isAudioOnly":                   true,
		"enablePersistentPlaylistPanel": true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
		"params":                        "wAEB",
	}
	root, err := post("next", body)
	if err != nil {
		return nil, err
	}

	panel := findKey(root, "playlistPanelRenderer")
	if panel == nil {
		return nil, fmt.Errorf("ytmusic radio: no playlist panel in response")
	}
	contents, _ := dig(panel, "contents").([]interface{})

	var tracks []Track
	for _, c := range contents {
		item := dig(c, "playlistPanelVideoRenderer")
		if item == nil {
			// Wrapped variant used for some entries.
			item = dig(c, "playlistPanelVideoWrapperRenderer", "primaryRenderer", "playlistPanelVideoRenderer")
		}
		if item == nil {
			continue
		}
		t, ok := parseRadioItem(item)
		if !ok || t.VideoID == videoID {
			continue
		}
		tracks = append(tracks, t)
		if limit > 0 && len(tracks) >= limit {
			break
		}
	}
	return tracks, nil
}

// ─── request plumbing ───────────────────────────────────────────────

func clientContext() map[string]interface{} {
	return map[string]interface{}{
		"client": map[string]interface{}{
			"clientName":    "WEB_REMIX",
			"clientVersion": clientVersion,
			"hl":            "en",
		},
	}
}

func post(endpoint string, body map[string]interface{}) (interface{}, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("ytmusic %s: marshal: %w", endpoint, err)
	}
	req, err := http.NewRequest("POST", baseURL+"/"+endpoint+"?prettyPrint=false", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("ytmusic %s: request: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://music.youtube.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ytmusic %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ytmusic %s: HTTP %d", endpoint, resp.StatusCode)
	}

	var root interface{}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, fmt.Errorf("ytmusic %s: decode: %w", endpoint, err)
	}
	return root, nil
}

// ─── response parsing ───────────────────────────────────────────────

func parseSearchItem(item interface{}) (Track, bool) {
	t := Track{
		VideoID: digString(item, "flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer",
			"text", "runs", 0, "navigationEndpoint", "watchEndpoint", "videoId"),
		Title: digString(item, "flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer",
			"text", "runs", 0, "text"),
	}
	if t.VideoID == "" {
		// Fallback: the play-button overlay carries the same endpoint.
		t.VideoID = digString(item, "overlay", "musicItemThumbnailOverlayRenderer", "content",
			"musicPlayButtonRenderer", "playNavigationEndpoint", "watchEndpoint", "videoId")
	}
	if t.VideoID == "" || t.Title == "" {
		return t, false
	}

	// Second column runs: [Artist, " • ", Album, " • ", "4:58"].
	runs, _ := dig(item, "flexColumns", 1, "musicResponsiveListItemFlexColumnRenderer",
		"text", "runs").([]interface{})
	var fields []string
	for _, r := range runs {
		s := digString(r, "text")
		if strings.TrimSpace(s) == "•" || s == "" {
			continue
		}
		fields = append(fields, s)
	}
	for i, f := range fields {
		switch {
		case i == 0:
			t.Artist = f
		case parseClock(f) > 0 && i == len(fields)-1:
			t.Duration = parseClock(f)
		case t.Album == "":
			t.Album = f
		}
	}

	t.CoverURL = largestThumbnail(dig(item, "thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"))
	return t, true
}

func parseRadioItem(item interface{}) (Track, bool) {
	t := Track{
		VideoID: digString(item, "videoId"),
		Title:   digString(item, "title", "runs", 0, "text"),
	}
	if t.VideoID == "" || t.Title == "" {
		return t, false
	}

	// Byline runs: [Artist, " • ", Album, " • ", "2016"].
	runs, _ := dig(item, "longBylineText", "runs").([]interface{})
	var fields []string
	for _, r := range runs {
		s := digString(r, "text")
		if strings.TrimSpace(s) == "•" || s == "" {
			continue
		}
		fields = append(fields, s)
	}
	if len(fields) > 0 {
		t.Artist = fields[0]
	}
	if len(fields) > 1 {
		t.Album = fields[1]
	}

	t.Duration = parseClock(digString(item, "lengthText", "runs", 0, "text"))
	t.CoverURL = largestThumbnail(dig(item, "thumbnail", "thumbnails"))
	return t, true
}

// sizeRe matches the size suffix of googleusercontent thumbnail URLs.
var sizeRe = regexp.MustCompile(`=w\d+-h\d+`)

// largestThumbnail picks the biggest listed thumbnail and upgrades the
// size parameters (googleusercontent serves arbitrary sizes on request).
func largestThumbnail(v interface{}) string {
	thumbs, _ := v.([]interface{})
	if len(thumbs) == 0 {
		return ""
	}
	url := digString(thumbs[len(thumbs)-1], "url")
	if sizeRe.MatchString(url) {
		url = sizeRe.ReplaceAllString(url, "=w544-h544")
	}
	return url
}

// parseClock converts "4:58" or "1:02:33" to seconds; 0 if not a clock.
func parseClock(s string) int {
	parts := strings.Split(strings.TrimSpace(s), ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0
	}
	total := 0
	for _, p := range parts {
		var n int
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil || n < 0 {
			return 0
		}
		total = total*60 + n
	}
	return total
}

// ─── loose JSON traversal ───────────────────────────────────────────
//
// InnerTube responses are deep trees full of renderer wrappers that
// shift between client versions. Loose traversal over interface{} with
// nil-safe steps is far more resilient than mirroring the whole tree
// in typed structs.

// dig walks a decoded JSON value by map keys (string) and array
// indices (int). Returns nil as soon as a step doesn't match.
func dig(v interface{}, path ...interface{}) interface{} {
	for _, p := range path {
		switch step := p.(type) {
		case string:
			m, ok := v.(map[string]interface{})
			if !ok {
				return nil
			}
			v = m[step]
		case int:
			arr, ok := v.([]interface{})
			if !ok || step < 0 || step >= len(arr) {
				return nil
			}
			v = arr[step]
		default:
			return nil
		}
		if v == nil {
			return nil
		}
	}
	return v
}

func digString(v interface{}, path ...interface{}) string {
	s, _ := dig(v, path...).(string)
	return s
}

// findKey returns the first value stored under key anywhere in the tree.
func findKey(v interface{}, key string) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if sub, ok := t[key]; ok {
			return sub
		}
		for _, vv := range t {
			if r := findKey(vv, key); r != nil {
				return r
			}
		}
	case []interface{}:
		for _, vv := range t {
			if r := findKey(vv, key); r != nil {
				return r
			}
		}
	}
	return nil
}
