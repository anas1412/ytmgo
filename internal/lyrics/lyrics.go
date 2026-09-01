// Package lyrics fetches song lyrics.
//
// Synced (timestamped) lyrics come from LRCLIB (lrclib.net), a free and
// open lyrics database that needs no key: its /api/get endpoint matches
// on track title, artist, and duration (±2s), and returns LRC-format
// text with one timestamp per line. When LRCLIB has nothing for a
// track, YouTube Music's own InnerTube endpoint provides plain text as
// a fallback — timing data only ever comes from LRCLIB.
//
// ErrNotFound marks a definitive miss (neither source knows the track),
// which callers may cache; transient failures come back as ordinary
// errors so a network blip never poisons a cache with "no lyrics".
package lyrics

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"ytmgo/internal/ytmusic"
)

// ErrNotFound reports that no lyrics exist for the track.
var ErrNotFound = errors.New("lyrics: not found")

// Line is one lyric line. For synced lyrics Time is the offset, in
// seconds from the start of the track, when the line becomes current;
// for plain lyrics it is unused (0).
type Line struct {
	Time float64
	Text string
}

// Lyrics is a fetched lyrics payload. Raw keeps the text as fetched
// (LRC or plain) so callers can cache it verbatim and rebuild with
// FromText instead of refetching.
type Lyrics struct {
	Raw    string
	Lines  []Line
	Synced bool   // Lines carry timestamps
	Source string // "lrclib", "youtube", or "cache"
}

// FromText rebuilds Lyrics from cached text: LRC when marked synced,
// plain lines otherwise.
func FromText(text string, synced bool) *Lyrics {
	if synced {
		if lines := ParseLRC(text); len(lines) > 0 {
			return &Lyrics{Raw: text, Lines: lines, Synced: true, Source: "cache"}
		}
		// Marked synced but does not parse — degrade to plain.
	}
	return plainFromText(text, "cache")
}

// Fetch returns lyrics for a track. title and artist come from the
// track metadata; durationSec (0 when unknown) sharpens LRCLIB's
// matching. trackID enables the InnerTube plain-text fallback; local
// file paths and legacy ids simply skip it.
func Fetch(trackID, title, artist string, durationSec int) (*Lyrics, error) {
	t, a := cleanQuery(title, artist)
	if t == "" {
		return nil, ErrNotFound
	}

	l, err := fetchLRCLIB(t, a, durationSec)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err // transient — the caller may retry later
	}

	if ytmusic.IsVideoID(trackID) {
		text, err := ytmusic.PlainLyrics(trackID)
		if err == nil && text != "" {
			return plainFromText(text, "youtube"), nil
		}
	}
	return nil, ErrNotFound
}

// ─── LRCLIB ──────────────────────────────────────────────────────────

const lrclibURL = "https://lrclib.net/api/get"

// userAgent identifies the app to LRCLIB, whose fair-use policy asks
// for a contactable client string.
const userAgent = "ytmgo (https://github.com/anas1412/ytmgo)"

var httpClient = &http.Client{Timeout: 15 * time.Second}

type lrclibResponse struct {
	PlainLyrics  string `json:"plainLyrics"`
	SyncedLyrics string `json:"syncedLyrics"`
	Instrumental bool   `json:"instrumental"`
}

func fetchLRCLIB(title, artist string, durationSec int) (*Lyrics, error) {
	q := url.Values{}
	q.Set("track_name", title)
	q.Set("artist_name", artist)
	if durationSec > 0 {
		q.Set("duration", strconv.Itoa(durationSec))
	}
	req, err := http.NewRequest("GET", lrclibURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lrclib: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotFound
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("lrclib: rate limited")
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("lrclib: HTTP %d", resp.StatusCode)
	}

	var r lrclibResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("lrclib: decode: %w", err)
	}
	if r.Instrumental {
		return nil, ErrNotFound
	}
	if r.SyncedLyrics != "" {
		if lines := ParseLRC(r.SyncedLyrics); len(lines) > 0 {
			return &Lyrics{Raw: r.SyncedLyrics, Lines: lines, Synced: true, Source: "lrclib"}, nil
		}
	}
	if r.PlainLyrics != "" {
		return plainFromText(r.PlainLyrics, "lrclib"), nil
	}
	return nil, ErrNotFound
}

// ─── LRC parsing ─────────────────────────────────────────────────────

// lrcTimeRe matches the "[mm:ss.xx]" timestamp tags of LRC lyrics.
var lrcTimeRe = regexp.MustCompile(`\[(\d{1,3}):(\d{2})(?:[.:](\d{1,3}))?\]`)

// ParseLRC converts LRC-format lyrics into timed lines. Metadata tags
// ([ar:], [ti:], …) carry no timestamp and are dropped; a line with
// several timestamps (repeat notation) expands to one Line each.
func ParseLRC(raw string) []Line {
	var lines []Line
	for _, ln := range strings.Split(raw, "\n") {
		ms := lrcTimeRe.FindAllStringSubmatch(ln, -1)
		if len(ms) == 0 {
			continue
		}
		text := strings.TrimSpace(lrcTimeRe.ReplaceAllString(ln, ""))
		for _, m := range ms {
			min, _ := strconv.Atoi(m[1])
			sec, _ := strconv.Atoi(m[2])
			frac := 0.0
			if m[3] != "" {
				if f, err := strconv.ParseFloat("0."+m[3], 64); err == nil {
					frac = f
				}
			}
			lines = append(lines, Line{
				Time: float64(min*60+sec) + frac,
				Text: text,
			})
		}
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Time < lines[j].Time })
	return lines
}

func plainFromText(text string, source string) *Lyrics {
	var lines []Line
	for _, ln := range strings.Split(text, "\n") {
		lines = append(lines, Line{Text: strings.TrimRight(ln, " \t\r")})
	}
	// Trim leading and trailing blanks; interior blanks are stanza
	// breaks and stay.
	for len(lines) > 0 && lines[0].Text == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1].Text == "" {
		lines = lines[:len(lines)-1]
	}
	return &Lyrics{Raw: text, Lines: lines, Synced: false, Source: source}
}

// ─── query cleanup ───────────────────────────────────────────────────

// querySuffixes are the YouTube title decorations that would defeat
// lyrics matching: LRCLIB keys on clean track names.
var querySuffixes = []string{
	"(official music video)", "(official video)", "(official lyric video)",
	"(lyric video)", "(lyrics)", "(audio)", "(official audio)",
	"(official hd video)", "(official visualizer)", "(hd)",
	"[official music video]", "[official video]", "[lyric video]",
	"[lyrics]", "[official audio]",
}

// cleanQuery strips those decorations from a title and the auto-topic
// suffix from an uploader name, so a search-result row can be matched
// against a lyrics database.
func cleanQuery(title, artist string) (string, string) {
	t := strings.TrimSpace(title)
	for {
		lt := strings.ToLower(t)
		if len(lt) != len(t) {
			break // exotic casing changed the byte length — stop trimming
		}
		matched := ""
		for _, suf := range querySuffixes {
			if strings.HasSuffix(lt, suf) {
				matched = suf
				break
			}
		}
		if matched == "" {
			break
		}
		t = strings.TrimSpace(t[:len(t)-len(matched)])
	}
	a := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(artist), " - Topic"))
	return t, a
}
