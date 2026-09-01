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
	"errors"
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
	VideoID       string
	Title         string
	Artist        string
	Album         string
	AlbumBrowseID string // MPREb_… — the id AlbumTracks takes (empty when unknown)
	Duration      int    // seconds
	CoverURL      string
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
		return nil, nil // no matches — YouTube omits the shelf entirely
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
	// Each run may carry a navigationEndpoint; the album run's points
	// at the album page (MPREb_…), which open-album-from-song uses.
	runs, _ := dig(item, "flexColumns", 1, "musicResponsiveListItemFlexColumnRenderer",
		"text", "runs").([]interface{})
	var fields []string
	var fieldAlbumIDs []string // parallel to fields: album browse id of the run, if any
	for _, r := range runs {
		s := digString(r, "text")
		if strings.TrimSpace(s) == "•" || s == "" {
			continue
		}
		fields = append(fields, s)
		fieldAlbumIDs = append(fieldAlbumIDs, albumBrowseIDFromRun(r))
	}
	for i, f := range fields {
		switch {
		case i == 0:
			t.Artist = f
		case parseClock(f) > 0 && i == len(fields)-1:
			t.Duration = parseClock(f)
		case t.Album == "":
			t.Album = f
			t.AlbumBrowseID = fieldAlbumIDs[i]
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
	var fieldAlbumIDs []string // parallel to fields, as in parseSearchItem
	for _, r := range runs {
		s := digString(r, "text")
		if strings.TrimSpace(s) == "•" || s == "" {
			continue
		}
		fields = append(fields, s)
		fieldAlbumIDs = append(fieldAlbumIDs, albumBrowseIDFromRun(r))
	}
	if len(fields) > 0 {
		t.Artist = fields[0]
	}
	if len(fields) > 1 {
		t.Album = fields[1]
		t.AlbumBrowseID = fieldAlbumIDs[1]
	}

	t.Duration = parseClock(digString(item, "lengthText", "runs", 0, "text"))
	t.CoverURL = largestThumbnail(dig(item, "thumbnail", "thumbnails"))
	return t, true
}

// albumBrowseIDFromRun returns the album browse id (MPREb_…) carried by
// a byline run's navigation endpoint, or "" when the run doesn't point
// at an album. Artist runs point at channels (MPLA…), so filtering on
// the MPRE prefix picks out just the album run.
func albumBrowseIDFromRun(run interface{}) string {
	if id := digString(run, "navigationEndpoint", "browseEndpoint", "browseId"); strings.HasPrefix(id, "MPRE") {
		return id
	}
	return ""
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

// ─── lyrics ─────────────────────────────────────────────────────────

// ErrNoLyrics reports that YouTube Music carries no lyrics for a track.
var ErrNoLyrics = errors.New("ytmusic: no lyrics")

// PlainLyrics fetches a track's lyrics from YouTube Music: the next
// endpoint exposes a lyrics engagement panel whose browse id (MPLYR…)
// the browse endpoint then serves as a description shelf. Plain text
// only — YouTube Music does not provide timings, so synced lyrics come
// from elsewhere.
func PlainLyrics(videoID string) (string, error) {
	root, err := post("next", map[string]interface{}{
		"context": clientContext(),
		"videoId": videoID,
	})
	if err != nil {
		return "", err
	}
	browseID := lyricsBrowseID(root)
	if browseID == "" {
		return "", ErrNoLyrics
	}

	root, err = post("browse", map[string]interface{}{
		"context":  clientContext(),
		"browseId": browseID,
	})
	if err != nil {
		return "", err
	}
	shelf := findKey(root, "musicDescriptionShelfRenderer")
	if shelf == nil {
		return "", ErrNoLyrics
	}
	runs, _ := dig(shelf, "description", "runs").([]interface{})
	var b strings.Builder
	for _, r := range runs {
		b.WriteString(digString(r, "text"))
	}
	text := strings.TrimSpace(b.String())
	if text == "" {
		return "", ErrNoLyrics
	}
	return text, nil
}

// lyricsBrowseID hunts the next response for the lyrics engagement
// panel (identified by its MPLYR panel identifier) and returns the
// browse id carried inside it.
func lyricsBrowseID(root interface{}) string {
	var found string
	var walk func(v interface{})
	walk = func(v interface{}) {
		if found != "" {
			return
		}
		switch t := v.(type) {
		case map[string]interface{}:
			if id, ok := t["panelIdentifier"].(string); ok && strings.Contains(id, "MPLYR") {
				if b := digString(findKey(t, "browseEndpoint"), "browseId"); strings.HasPrefix(b, "MPLYR") {
					found = b
					return
				}
			}
			for _, vv := range t {
				walk(vv)
			}
		case []interface{}:
			for _, vv := range t {
				walk(vv)
			}
		}
	}
	walk(root)
	return found
}

// ─── albums ─────────────────────────────────────────────────────────

// albumsFilterParams restricts /search to the Albums shelf.
const albumsFilterParams = "EgWKAQIYAWoKEAkQChAFEAMQBA%3D%3D"

// Album is one album from a search, or a fetched album with its tracks.
type Album struct {
	BrowseID string // MPREb_… — the id AlbumTracks takes
	Title    string
	Artist   string
	Year     string
	CoverURL string
	Tracks   []Track // populated by AlbumTracks
}

// SearchAlbums runs an albums-filtered YouTube Music search.
func SearchAlbums(query string, limit int) ([]Album, error) {
	body := map[string]interface{}{
		"context": clientContext(),
		"query":   query,
		"params":  albumsFilterParams,
	}
	root, err := post("search", body)
	if err != nil {
		return nil, err
	}

	shelf := findKey(root, "musicShelfRenderer")
	if shelf == nil {
		return nil, nil // no matches — YouTube omits the shelf entirely
	}
	contents, _ := dig(shelf, "contents").([]interface{})

	var albums []Album
	for _, c := range contents {
		item := dig(c, "musicResponsiveListItemRenderer")
		if item == nil {
			continue
		}
		a, ok := parseAlbumItem(item)
		if !ok {
			continue
		}
		albums = append(albums, a)
		if limit > 0 && len(albums) >= limit {
			break
		}
	}
	return albums, nil
}

// AlbumTracks fetches an album page and returns its metadata plus the
// full tracklist, in album order.
func AlbumTracks(browseID string) (Album, error) {
	body := map[string]interface{}{
		"context":  clientContext(),
		"browseId": browseID,
	}
	root, err := post("browse", body)
	if err != nil {
		return Album{}, err
	}

	album := Album{BrowseID: browseID}
	if h := findKey(root, "musicResponsiveHeaderRenderer"); h != nil {
		album.Title = digString(h, "title", "runs", 0, "text")
		album.Artist = digString(h, "straplineTextOne", "runs", 0, "text")
		// subtitle runs read ["Album", " • ", "2015"] — the year is last.
		if runs, _ := dig(h, "subtitle", "runs").([]interface{}); len(runs) > 0 {
			album.Year = digString(runs[len(runs)-1], "text")
		}
		album.CoverURL = largestThumbnail(dig(h, "thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"))
	}

	shelf := findKey(root, "musicShelfRenderer")
	if shelf == nil {
		return album, fmt.Errorf("ytmusic album: no tracklist in response")
	}
	contents, _ := dig(shelf, "contents").([]interface{})
	for _, c := range contents {
		item := dig(c, "musicResponsiveListItemRenderer")
		if item == nil {
			continue
		}
		t := Track{
			VideoID: digString(item, "playlistItemData", "videoId"),
			Title: digString(item, "flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer",
				"text", "runs", 0, "text"),
			Artist:        album.Artist,
			Album:         album.Title,
			AlbumBrowseID: album.BrowseID,
			CoverURL:      album.CoverURL,
		}
		if t.VideoID == "" {
			t.VideoID = digString(item, "overlay", "musicItemThumbnailOverlayRenderer", "content",
				"musicPlayButtonRenderer", "playNavigationEndpoint", "watchEndpoint", "videoId")
		}
		// Duration sits in the fixed (right-hand) column.
		t.Duration = parseClock(digString(item, "fixedColumns", 0,
			"musicResponsiveListItemFixedColumnRenderer", "text", "runs", 0, "text"))
		if t.VideoID == "" || t.Title == "" {
			continue
		}
		album.Tracks = append(album.Tracks, t)
	}
	if len(album.Tracks) == 0 {
		return album, fmt.Errorf("ytmusic album: no playable tracks found")
	}
	return album, nil
}

func parseAlbumItem(item interface{}) (Album, bool) {
	a := Album{
		Title: digString(item, "flexColumns", 0, "musicResponsiveListItemFlexColumnRenderer",
			"text", "runs", 0, "text"),
		BrowseID: digString(item, "navigationEndpoint", "browseEndpoint", "browseId"),
	}
	if !strings.HasPrefix(a.BrowseID, "MPRE") {
		// Fall back to the thumbnail overlay, which also carries it.
		a.BrowseID = digString(item, "overlay", "musicItemThumbnailOverlayRenderer", "content",
			"musicPlayButtonRenderer", "playNavigationEndpoint", "watchPlaylistEndpoint", "playlistId")
	}
	if a.Title == "" || !strings.HasPrefix(a.BrowseID, "MPRE") {
		return a, false
	}
	// Second column reads ["Album", " • ", "Artist", " • ", "2015"].
	runs, _ := dig(item, "flexColumns", 1, "musicResponsiveListItemFlexColumnRenderer", "text", "runs").([]interface{})
	var fields []string
	for _, r := range runs {
		s := digString(r, "text")
		if strings.TrimSpace(s) == "•" || s == "" || s == "Album" || s == "EP" || s == "Single" {
			continue
		}
		fields = append(fields, s)
	}
	if len(fields) > 0 {
		a.Artist = fields[0]
	}
	if len(fields) > 1 {
		a.Year = fields[len(fields)-1]
	}
	a.CoverURL = largestThumbnail(dig(item, "thumbnail", "musicThumbnailRenderer", "thumbnail", "thumbnails"))
	return a, true
}
