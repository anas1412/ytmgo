# TIDAL + yt-dlp Hybrid Plan

> **Goal**: Use **TIDAL** (via community proxies) for real music search and recommendations, and **yt-dlp** for reliable streaming and downloads.

> **Why hybrid?**: TIDAL proxy search/info endpoints work (on `monochrome.tf` etc.) and give proper music catalog metadata (artist, album, duration, cover art). But the `/track/` streaming endpoint requires a TIDAL subscription token that all public proxies have lost — making TIDAL streaming impossible. Meanwhile, yt-dlp is battle-tested for streaming from YouTube. Using TIDAL for **discovery** and yt-dlp for **delivery** gives the best of both worlds.

---

## Architecture

### Before (TIDAL-only — BROKEN)

```
Search:      tidal.Client.SearchTracks()   → monochrome.tf/search/   ✅ Works
Recs:        tidal.Client.FetchRecs()      → monochrome.tf/search/   ✅ Works
Stream URL:  tidal.Client.GetTrackURL()    → monochrome.tf/track/    ❌ "Upstream API error"
Download:    HTTP GET tidal stream URL                                ❌ Same dead endpoint
```

### After (TIDAL search + yt-dlp streaming)

```
Search:      tidal.Client.SearchTracks()        → monochrome.tf/search/  ✅ Works
Recs:        tidal.Client.FetchRecs()           → monochrome.tf/search/  ✅ Works
Stream URL:  ytresolve.Resolve(artist, title)   → yt-dlp "ytsearch1:A - T" → YouTube URL  ✅ Works
Download:    yt-dlp subprocess (artist - title)                         ✅ Works
```

### Data Flow

```
TIDAL Search
    │
    ▼
{ID: "12345", Title: "Wonderwall", Artist: "Oasis", Duration: 278}
    │
    ├──→ TUI displays title, artist, album, duration
    │
    ├──→ User presses Enter → resolvePlayURL()
    │       │
    │       └──→ ytresolve.Resolve("Oasis", "Wonderwall")
    │               │
    │               └──→ yt-dlp --flat-playlist --dump-json "ytsearch1:Oasis - Wonderwall"
    │                       │
    │                       └──→ {id: "bx1Bh8ZvH84", webpage_url: "https://youtube.com/watch?v=bx1Bh8ZvH84"}
    │                               │
    │                               └──→ mpv "https://youtube.com/watch?v=bx1Bh8ZvH84"
    │
    └──→ User presses X → downloader.Enqueue()
            │
            └──→ yt-dlp -x --audio-format m4a "ytsearch1:Oasis - Wonderwall" -o "Wonderwall.m4a"
```

---

## Files Changed

### Phase 0 — New package: `internal/ytresolve/` (YouTube URL Resolver)

Creates a small package that uses `yt-dlp` to find a YouTube video URL from artist + title metadata.

#### New file: `internal/ytresolve/resolver.go`

```go
package ytresolve

import (
    "encoding/json"
    "fmt"
    "os/exec"
)

// Result is a single YouTube search result.
type Result struct {
    ID         string  `json:"id"`
    Title      string  `json:"title"`
    URL        string  `json:"url"`
    WebpageURL string  `json:"webpage_url"`
    Duration   float64 `json:"duration"`
    Channel    string  `json:"channel"`
}

// Resolve searches YouTube for the given artist and title, and returns
// the first matching video's URL. Uses yt-dlp --flat-playlist --dump-json
// for a fast metadata-only search (no actual download).
func Resolve(artist, title string) (*Result, error) {
    query := fmt.Sprintf("%s - %s", artist, title)
    // ... exec yt-dlp, parse JSON, return first result
}
```

### Phase 1 — Clean up TIDAL client

Remove dead streaming code, keep search/info/mix. The proxy list stays for search failover.

#### Modify: `internal/tidal/client.go`

Keep:
- `SearchTracks()` — works via monochrome.tf proxy
- `GetTrackInfo()` — works via monochrome.tf proxy
- `GetMixTracks()` — works via monochrome.tf proxy
- `FetchRecommendations()` — uses SearchTracks as fallback
- `HealthCheck()` — useful for proxy validation
- `get()` helper with proxy fallback — useful for search failover

Remove:
- `GetTrackURL()` — dead endpoint, all proxies return "Upstream API error"
- `FetchProxiesList()` — no longer needed (static list in KnownProxyURLs suffices for search)
- `SetProxyList()` — no longer needed
- `ProxiesListURL` constant — no longer needed
- `ProxiesFetchedMsg` type — no longer needed

Simplify:
- Remove `proxyIndex` field (no longer tracking index for streaming fallback)
- `get()` method can still try multiple proxies for search failover, but with simpler logic

#### Modify: `internal/tidal/types.go`

Keep all types. Can remove `TrackURLResponse` and `Manifest` structs if they're unused (used only by `GetTrackURL`).

#### Remove: `proxies.json`

No longer fetched at startup. Static `KnownProxyURLs` list in `types.go` is sufficient for search proxy failover.

### Phase 2 — Rewrite Downloader

#### Rewrite: `internal/downloader/downloader.go`

Replace HTTP-based TIDAL download with yt-dlp subprocess:

```go
func (d *Downloader) runJob(job *Job, outDir string) {
    // 1. Search YouTube for "Artist - Title" to find matching video
    // 2. Download via yt-dlp -x --audio-format m4a
    // 3. Emit progress (parse yt-dlp stderr for percentage)
    // 4. Mark as done
}
```

Constructor no longer needs `*tidal.Client`:
```go
func New(outDir string) *Downloader { ... }
```

**Progress tracking**: Parse yt-dlp's stderr output for `[download] X.X%` lines, similar to the original approach.

### Phase 3 — Rewrite `resolvePlayURL()`

#### Modify: `internal/tui/model.go`

```go
// resolvePlayURL returns the URL for mpv to play.
// If the track is downloaded locally, returns the file path.
// Otherwise, searches YouTube via yt-dlp for "{artist} - {title}".
func (m *Model) resolvePlayURL(t queue.Track) string {
    if t.Downloaded && t.FilePath != "" {
        if _, err := os.Stat(t.FilePath); err == nil {
            return t.FilePath
        }
    }
    // Resolve via yt-dlp search using TIDAL metadata
    result, err := ytresolve.Resolve(t.Artist, t.Title)
    if err != nil {
        m.err = fmt.Errorf("failed to resolve stream URL: %w", err)
        return ""
    }
    return result.WebpageURL
}
```

Also:
- Remove `reinitTidalClient()` — no longer needed (TIDAL client only used for search)
- Remove `ProxiesFetchedMsg` message type
- Remove `tidalClient` field from Model (or keep it for search — see below)

Actually, keep `tidalClient` since search still uses it. Remove only dead code.

### Phase 4 — Update TUI integration

#### Modify: `internal/tui/commands.go`

- Remove `fetchProxiesCmd()` — no longer needed
- `searchCmd()` and `fetchRecommendationsCmd()` — unchanged (still use tidal.Client)
- `downloadCmd()` — unchanged (still reads from downloader channel)

#### Modify: `internal/tui/update.go`

- Remove `ProxiesFetchedMsg` case handler
- Remove `fetchProxiesCmd()` from `Init()` tea.Batch
- Everything else unchanged

#### Modify: `internal/tui/keyboard.go`

- Line 413, 417: `m.downloader.Enqueue(t.ID, t.Title, r.Uploader, r.URL, m.downloadDir())` — `r.URL` is still empty (it was empty before too), downloader now resolves via yt-dlp internally using the title+uploader metadata. No change needed to the signature, but the `URL` field is now used as a fallback if yt-dlp search fails.
- Line 608: `if t.URL == ""` — this check may need updating since URL will always be empty now (it was set to empty from TIDAL search). Instead, the downloader should verify it can find a video.

Actually, let me reconsider. The downloader `Enqueue()` signature takes a URL. In the old TIDAL code, the URL was empty and the downloader used `GetTrackURL()`. In the new code, the URL is also empty and the downloader needs to search via yt-dlp.

So we should change `Enqueue()` to not take a URL at all, or change the semantics:
```go
func (d *Downloader) Enqueue(trackID, title, uploader, outDir string)
```

Or keep the signature and just ignore the URL, using title+uploader for the search.

### Phase 5 — Dependencies & Config

#### Modify: `install.sh`

- Add `yt-dlp` back to dependency list
- Add `ffmpeg` back (yt-dlp needs it for `--audio-format m4a`)
- Keep `mpv`

#### Modify: `internal/settings/settings.go`

- Keep `TidalProxyURL` — still needed for search proxy configuration
- Keep `StreamQuality` — may still be useful for display preference, or could be removed if no longer needed. For now keep but mark as legacy.

#### Modify: `internal/db/db.go`

- Keep `tidal_proxy_url` and `stream_quality` columns — proxy URL still useful for search configuration
- No schema changes needed

### Phase 6 — Remove dead code

- Remove `proxies.json` file from repo root
- Remove `internal/ytdlp/` — already deleted
- Clean up any remaining references to dead TIDAL streaming endpoints

---

## Dependency Changes

### Added back
| Dependency | Reason |
|-----------|--------|
| `yt-dlp` | YouTube streaming URL resolution + download |
| `ffmpeg` | Audio extraction for downloads (yt-dlp dependency) |

### Kept
| Dependency | Reason |
|-----------|--------|
| `mpv` | Audio playback backend |
| `internal/tidal/` | Search, recommendations, track metadata |
| `internal/search/search.go` | TIDAL-backed search (unchanged) |
| `SQLite`, `Bubble Tea`, etc. | Unchanged |

### Removed
| Code | Reason |
|------|--------|
| `tidal.Client.GetTrackURL()` | Dead endpoint (all proxies return "Upstream API error") |
| `tidal.FetchProxiesList()` | Unnecessary — static proxy list suffices for search failover |
| `proxies.json` | No longer fetched at startup |
| `ProxiesFetchedMsg` / `fetchProxiesCmd` | Remove from TUI |

---

## Execution Order

```
Phase 0: New internal/ytresolve/ package
  └── ytresolve/resolver.go          ← new

Phase 1: Clean up TIDAL client
  └── internal/tidal/client.go        ← remove GetTrackURL, FetchProxiesList, SetProxyList
  └── internal/tidal/types.go         ← maybe remove TrackURLResponse, Manifest
  └── proxies.json                    ← delete file

Phase 2: Rewrite Downloader
  └── internal/downloader/downloader.go  ← rewrite to use yt-dlp subprocess

Phase 3: Rewrite resolvePlayURL
  └── internal/tui/model.go              ← update resolvePlayURL, remove dead code

Phase 4: Update TUI integration
  └── internal/tui/commands.go           ← remove fetchProxiesCmd
  └── internal/tui/update.go             ← remove ProxiesFetchedMsg handler, update Init
  └── internal/tui/keyboard.go           ← update download enqueue calls

Phase 5: Dependencies
  └── install.sh                         ← add yt-dlp + ffmpeg back
  └── README.md                          ← update dependency list

Phase 6: Cleanup & build
  └── Verify build: go build ./... && go vet ./...
  └── Verify: go test ./...
  └── Verify: yt-dlp is in PATH
```

---

## Open Questions

1. **Download progress**: yt-dlp outputs progress to stderr. We'll need to parse it (regex on `[download] X.X%`). This is the same approach used in the original yt-dlp code before the TIDAL migration.

2. **Search result quality**: yt-dlp's `ytsearch1:Artist - Title` generally returns the correct music video as the first result. If it returns a wrong video, the user can still skip to the next track. Duration validation was explicitly de-prioritized by the user.

3. **Download naming**: Files should be saved as `{Artist} - {Title}.m4a` using the TIDAL metadata (which is cleaner than YouTube titles that may include " (Official Video)" etc.).

4. **Downloaded file format**: yt-dlp `-x --audio-format m4a` produces M4A (AAC) which mpv plays natively. This matches the previous TIDAL downloader's `.m4a` extension.

5. **TIDAL proxy for search**: `monochrome.tf` instances are currently working for search/info. If they also die, search falls back to other proxies in the static list. If all are dead, search will fail with a clear error message.
