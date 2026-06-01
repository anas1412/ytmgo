# 🎵 ytmgo — 3-Page Architecture Plan

## Overview

Split the monolithic TUI into three focused pages:
1. **Stream** (default) — search, recommendations, queue, playback. No download bar.
2. **Library** — downloaded tracks + download queue management.
3. **Settings** — config toggles persisted to disk.

The player, queue, and downloader are shared across all pages — switching pages never interrupts
playback or downloads.

---

## Page 1 — Stream (Main Page)

```
┌─────────────────────────────────────────────────────────┐
│ ♫ ytmgo v0.1   [Search YouTube…]               [/]     │
├───────────────────────────┬─────────────────────────────┤
│ SEARCH RESULTS / RECS     │ QUEUE  [5]                  │
│                           │                             │
│ 1. Song Title             │ ▶ 1. Now Playing            │
│    Artist       3:45      │      Artist       1:23    ✓ │
│ 2. Another Song           │   2. Next Track             │
│    Artist       5:12      │      Artist       4:00      │
│ 3. Third                  │                             │
│    Artist       2:30      │   ↓ 2 more                  │
│                           │                             │
│ ↓ 3 more                  │                             │
├───────────────────────────┴─────────────────────────────┤
│ ⏹ Stopped  [━━━━━━━━━━━━━━━━━━]  0:00/0:00  ◀⏸▶ ⏭    │
│ Now playing: Song Title                                 │
├─────────────────────────────────────────────────────────┤
│ [1] Stream  [2] Library  [3] Settings  ? help  q       │
└─────────────────────────────────────────────────────────┘
```

### Behavior Changes

| Action | Current | New |
|--------|---------|-----|
| `Enter` on search result | Enqueue + start download, play when done | Enqueue + play URL immediately via mpv |
| `Enter` on library track (old `L` view) | Play local file | Moved to Library page |
| Download bar | Always visible, shows active download | **Removed from Stream page** |
| `x` on a queued track | Not handled (key exists in keys.go but no case in update.go) | Enqueue download for offline (only on Library page) |

### Enter on search result (Stream page)

```go
// psuedocode
case PanelSearch && !showingLibrary && !searchFocused:
    if cursor >= len(results) { return m, nil }
    r := results[cursor]
    t := searchResultToTrack(r)
    m.queue.Add(t)
    m.queue.SetCurrentIndex(m.queue.Len() - 1)
    m.queueCursor = m.queue.CurrentIndex()
    m.playerState = player.StatePlaying
    m.duration = float64(t.DurationSec)
    m.position = 0
    m.statusMessage = "Now playing: " + t.Title
    if err := m.player.Play(t.URL); err != nil {  // ← Play URL, not FilePath
        m.err = err
        m.playerState = player.StateStopped
        return m, nil
    }
    return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
```

### Auto-advance (SongEndedMsg)

```go
// Changed to skip UNDOWNLOADED tracks that have NO URL.
// Streamed tracks (URL != "" && !Downloaded) ARE playable.
case SongEndedMsg:
    for {
        t, ok := m.queue.Next()
        if !ok {
            m.playerState = player.StateStopped
            m.position = 0
            return m, nil
        }
        m.queueCursor = m.queue.CurrentIndex()

        if t.Downloaded && t.FilePath != "" {
            // Play local file
            m.playerState = player.StatePlaying
            m.duration = float64(t.DurationSec)
            m.position = 0
            m.statusMessage = "Now playing: " + t.Title
            if err := m.player.Play(t.FilePath); err != nil {
                m.err = err
                m.playerState = player.StateStopped
                return m, nil
            }
            return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
        }

        if t.URL != "" {
            // Stream via URL (not downloaded, but has URL)
            m.playerState = player.StatePlaying
            m.duration = float64(t.DurationSec)
            m.position = 0
            m.statusMessage = "Now playing: " + t.Title
            if err := m.player.Play(t.URL); err != nil {
                m.err = err
                m.playerState = player.StateStopped
                return m, nil
            }
            return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
        }

        // No URL and not downloaded — skip, keep looking
    }
```

### Edge Cases — Stream Page

| Edge Case | Handling |
|-----------|----------|
| Network down when pressing Enter | `m.player.Play(url)` returns error → set `m.err`, `m.playerState = StateStopped`, queue still has the track |
| URL is empty on a result | Defensive: check `r.URL != ""` before playing; if empty, show status message |
| Queue empty, user presses Enter on result | Play starts immediately (only track in queue, currentIndex = 0) |
| User presses Enter on the same result twice | Track gets added twice to queue (no dedup — same as current behavior). Both entries play fine since they have independent queue positions |
| mpv fails to play URL (e.g., age-restricted) | `m.player.Play()` returns error → `m.err` set, status shows failure, queue advances when `SongEndedMsg` fires |
| No recommendations loaded on startup | `m.results` is empty → shows "Loading recommendations…" (same as current behavior) |
| Search results vs recommendations | Same as current behavior — search overwrites results, `R` re-fetches recs |
| `L` key on Stream page | Currently toggles library overlay in search panel. **New:** disabled on Stream page (library is its own page). Pressing `L` on Stream page does nothing or jumps to Library page |
| Queue cursor out of bounds after auto-advance | Already handled by `Queue.Next()` and `clampQueueOffset()` |

---

## Page 2 — Library / Downloads

```
┌─────────────────────────────────────────────────────────┐
│ ♫ ytmgo v0.1   [Filter library…]              [/]      │
├───────────────────────────┬─────────────────────────────┤
│ LIBRARY  [42]             │ DOWNLOADS                   │
│                           │                             │
│ 1. Downloaded Song        │ ⬇ Actively downloading:    │
│    Artist       3:45  ✓  │   Song 1  ████░░  60%  [x]  │
│ 2. Offline Track          │                             │
│    Artist       5:12  ✓  │ ⏳ Pending:                 │
│ 3. Another Song           │   Song 3                    │
│    Artist       2:30  ✓  │   Song 4                    │
│                           │   Song 5                    │
│ ↓ 3 more                  │                             │
│                           │ ✓ Completed:                │
│ [d] delete  [Enter] play  │   Song 2  ✓                │
├───────────────────────────┴─────────────────────────────┤
│ ⏹ Stopped  [━━━━━━━━━━━━━━━━━━]  0:00/0:00  ◀⏸▶ ⏭    │
│ Now playing: Song Title                                 │
├─────────────────────────────────────────────────────────┤
│ [1] Stream  [2] Library  [3] Settings  ? help  q       │
└─────────────────────────────────────────────────────────┘
```

### Left Panel: Downloaded Tracks (`m.library`)
- Same as current library view but lives here permanently
- Shows all tracks from `library.ScanDir()` with checkmarks
- Search/filter via search input (same as current behavior)
- `Enter` on a track → plays local file (same as current `L` + Enter flow)
- `d` on a track → **deletes** the file from disk AND removes from library list
- Shows count like `LIBRARY  [42]`

### Right Panel: Download Queue
- Shows all jobs from `m.downloader.Jobs()` — a new method returning `[]*downloader.Job`
- Sections:
  - **Active** (status `StatusDownloading`) with progress bar
  - **Pending** (status `StatusPending`) 
  - **Completed** (status `StatusDone`/`StatusSkipped`) with checkmark
  - **Failed** (status `StatusFailed`) with error indicator
- Cursor navigation within the list
- `x` on a pending job → starts download (actually pending jobs auto-start via downloader worker)
- `d` on a completed job → delete downloaded file
- `D` to clear completed/failed jobs from the list

### Model Changes for Library Page

```go
type Model struct {
    // ...existing fields...
    activePage        Page       // NEW: which page is shown
    downloadJobs      []*downloader.Job // NEW: cached list for display
    libraryFilter     string     // NEW: separate filter text for library page
}
```

### Library page key handlers

| Key | Action |
|-----|--------|
| `up`/`down` | Navigate active panel (left: library, right: download queue) |
| `tab` | Cycle focus between panels (library ↔ downloads) |
| `enter` | On library track: play local file. On download completed: play |
| `d` | On library track: delete file. On download completed: delete file |
| `x` | On pending/downloading: (no-op, auto-started). On done/failed: re-download |
| `L` | Jump back to Library page (no-op if already on it) |
| `1` | Switch to Stream page |
| `/` | Focus search input to filter library |

### Library → Download Flow

When user wants to download a queued track for offline:
1. Switch to Library page (or use `x` key from Stream page)
2. View download progress on right panel
3. The downloader's `Enqueue()` method is called when:
   - User presses `x` on a search result (from Stream page)
   - User presses `x` on a queued track (from Library page)

**New:** Add `x` handler in update.go for both Stream and Library pages:

```go
// On Stream page: download a queued track
case PageStream && activePanel == PanelQueue:
    i := queueOffset + queueCursor
    tracks := m.queue.Tracks()
    if i >= len(tracks) { return m, nil }
    t := tracks[i]
    if t.Downloaded { 
        m.statusMessage = "Already downloaded: " + t.Title
        return m, nil 
    }
    m.downloader.Enqueue(t.ID, t.Title, t.URL, downloadDir())
    m.statusMessage = "Download queued: " + t.Title

// On Library page: download a queued track from queue panel
case PageLibrary && activePanel == rightPanel:
    // similar
```

### Edge Cases — Library Page

| Edge Case | Handling |
|-----------|----------|
| Library scan finds 0 files | Show empty state: "No downloaded tracks yet. Download from Stream page with `x`." |
| File deleted externally between scan and `d` key | `os.Remove()` returns error → show in status, remove from list anyway |
| Delete file that is currently playing | Stop playback first (`m.player.Stop()`), then delete. Show status: "Deleted currently playing file — playback stopped" |
| Filter returns 0 results | Show "No tracks match '<query>'" (same as current empty-filter behavior) |
| Library cursor out of bounds after filter | `clampLibraryOffset` already handles this |
| Download queue empty | Show empty state: "No active downloads. Press `x` in Stream to download for offline." |
| Download job list grows long | Show scroll indicators (`↓ N more`) if list exceeds panel height |
| Library file has embedded metadata via ffprobe | Already handled by `probeDuration()` — gets actual duration from file |
| Library file has no artist in filename | `parseFilename` returns empty artist → show "Unknown Artist" |
| Duplicate library entries (same file scanned twice) | `ScanDir` reads directory entries; if file appears once in dir, it's once in list. No dedup needed |
| `d` on a track that's also in the queue | Queue still has reference to old FilePath. After delete, if auto-advance tries to play that FilePath, `m.player.Play()` will fail → error caught, skip to next |
| Download job already exists for same track ID | `Enqueue` currently always creates new job (no dedup). Could add check: if pending job exists with same ID, skip |
| `downloader.Jobs()` — need a new method | Add `func (d *Downloader) Jobs() []*Job` that returns copy of `d.jobs` slice for UI rendering |
| Download progress updates while on Library page | `DownloadProgressMsg` handler runs regardless of active page — progress always updates in model. UI re-renders on every msg. |

---

## Page 3 — Settings

```
┌─────────────────────────────────────────────────────────┐
│ ♫ ytmgo  v0.1                          Settings         │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  Stream Mode                          ● ON   ○ OFF     │
│    Stream from YouTube directly instead of downloading  │
│                                                         │
│  Auto-Download on Add                  ○ ON   ● OFF     │
│    Automatically download queued tracks for offline     │
│                                                         │
│  Default Volume                         [80]  ━━━━━░   │
│                                                         │
│  Search Results Limit                   [20]            │
│                                                         │
│  Download Directory                 ./downloads/        │
│                                                         │
│  Cookie Browser                  Brave-Nightly          │
│                                                         │
│  Library Rescan                     [ rescan now ]      │
│                                                         │
├─────────────────────────────────────────────────────────┤
│ [1] Stream  [2] Library  [3] Settings  ? help  q       │
└─────────────────────────────────────────────────────────┘
```

### Settings Struct

New file: `internal/settings/settings.go`

```go
package settings

import (
    "encoding/json"
    "os"
    "path/filepath"
)

type Settings struct {
    StreamMode      bool   `json:"stream_mode"`       // default: true
    AutoDownload    bool   `json:"auto_download"`     // default: false  
    DefaultVolume   int    `json:"default_volume"`    // default: 80
    SearchLimit     int    `json:"search_limit"`      // default: 20
    DownloadDir     string `json:"download_dir"`      // default: "./downloads"
    CookieBrowser   string `json:"cookie_browser"`    // default: "brave" (detected)
}

const configPath = "~/.config/ytmgo/settings.json"

func Load() (*Settings, error)
func (s *Settings) Save() error
func Defaults() *Settings
```

### Settings UI

- Simple list of settings, cursor navigates between them
- Each setting type determines interaction:
  - **Booleans** (Stream Mode, Auto-Download) → toggle with `Enter` or `space`
  - **Numbers** (Volume, Search Limit) → `+`/`-` or `←`/`→` to adjust
  - **Strings** (Download Dir, Cookie Browser) → `Enter` to edit inline
  - **Actions** (Library Rescan) → `Enter` to trigger
- Changes saved to disk immediately on modification
- No "save" button needed — auto-save

### Model Changes for Settings

```go
type SettingsField int
const (
    FieldStreamMode SettingsField = iota
    FieldAutoDownload
    FieldDefaultVolume
    FieldSearchLimit
    FieldDownloadDir
    FieldCookieBrowser
    FieldRescanLibrary
)

type Model struct {
    // ...existing fields...
    settings          *settings.Settings  // NEW
    settingsCursor    int                 // NEW: which setting is selected
    settingsEditField bool               // NEW: editing a string field inline
    settingsEditInput textinput.Model     // NEW: input widget for string editing
}
```

### Settings page key handlers

| Key | Action |
|-----|--------|
| `up`/`down` | Navigate settings list |
| `enter`/`space` | Toggle boolean / start editing string / trigger action |
| `+`/`=` | Increase number value |
| `-`/`_` | Decrease number value |
| `esc` | Cancel editing / exit settings (switch to Stream) |
| `1` | Switch to Stream page |

### Edge Cases — Settings Page

| Edge Case | Handling |
|-----------|----------|
| Config file doesn't exist on first run | `Load()` returns `os.ErrNotExist` → use `Defaults()`, create file on first `Save()` |
| Config file is corrupted/invalid JSON | `Load()` returns error → log warning, use `Defaults()`, overwrite on next `Save()` |
| Download dir changed mid-session | Update `m.downloadDir()`, rescan library. Old downloads remain at old path |
| Volume changed while mpv is running | Immediately call `m.player.SetVolume(settings.DefaultVolume)` |
| Search limit changed | Affects next search only (already searched results are in memory) |
| Cookie browser path invalid | Existing code in search.go/downloader.go handles missing Brave config gracefully |
| Network location for config dir (NFS) | Standard `os.ReadFile`/`os.WriteFile` — may be slow but functional |
| Permissions error writing config | `Save()` returns error → show in status message but don't crash |
| Two instances modifying config | Last write wins (no locking). Acceptable for personal project |
| String edit field is too long | `textinput.Model` handles this natively |
| Volume set below 0 or above 100 | Clamp: `max(0, min(100, newVol))` |

---

## Shared State Model

All pages share:

```
┌─────────────────────────────────────────────────────────────┐
│  Model                                                      │
│  ├── Page state: activePage, panels, cursors, offsets       │
│  ├── Queue (shared) ─── queue, queueCursor, queueOffset    │
│  ├── Player (shared) ── player, playerState, pos, dur, vol │
│  ├── Downloader (shared) ── downloader, downloadJobs()     │
│  ├── Library (shared) ──── library, libraryCursor         │
│  └── Settings (shared) ─── settings                        │
└─────────────────────────────────────────────────────────────┘
```

Model fields after changes:

```go
type Model struct {
    // ── Terminal ──
    width, height int
    ready         bool

    // ── Page Navigation ──
    activePage    Page       // PageStream (0), PageLibrary (1), PageSettings (2)
    activePanel   Panel      // Per-page: which sub-panel has focus
    showHelp      bool
    quitting      bool

    // ── Search ──
    searchInput           textinput.Model
    searchFocused         bool
    searchCursor          int
    searchOffset          int
    results               []search.Result
    isSearching           bool
    showingRecommendations bool
    recStreamCh           chan search.Result
    recStreamCancel       context.CancelFunc

    // ── Library ──
    library       []queue.Track
    libraryCursor int
    libraryOffset int

    // ── Queue ──
    queue       *queue.Queue
    queueCursor int
    queueOffset int
    queuePanel  Panel  // NEW: which panel in the queue (QueuePanelList or QueuePanelDownloads)

    // ── Player ──
    player      *player.Player
    playerState player.State
    position    float64
    duration    float64
    volume      int

    // ── Downloader ──
    downloader    *downloader.Downloader
    downloading   bool
    downloadTitle string
    downloadPct   float64
    downloadDone  bool
    downloadErr   error

    // ── Settings ──
    settings          *settings.Settings  // NEW
    settingsCursor    int                 // NEW
    settingsEditField bool                // NEW
    settingsEditInput textinput.Model     // NEW

    // ── Status ──
    statusMessage string
    err           error
}
```

**REMOVED fields:**
- `showingLibrary` — no longer needed; Library is its own page
- `showSettings` — if it existed; Settings is its own page

---

## Update Function Architecture

```
Update(msg tea.Msg) → (Model, tea.Cmd)

  ├── WindowSizeMsg → resize (always)
  ├── tea.MouseMsg  → route to page handler
  │
  ├── Model-level messages (always handled):
  │   ├── SearchResultsMsg
  │   ├── RecStreamMsg
  │   ├── RecommendationsMsg
  │   ├── LibraryScanMsg
  │   ├── DownloadProgressMsg (runs regardless of page)
  │   ├── PositionMsg
  │   ├── SongEndedMsg
  │   ├── tickMsg
  │   └── SettingsSavedMsg (new)
  │
  ├── tea.KeyMsg → dispatch by activePage:
  │   ├── PageStream  → streamPageKeyHandler(msg)
  │   ├── PageLibrary → libraryPageKeyHandler(msg)
  │   └── PageSettings → settingsPageKeyHandler(msg)
  │
  └── ErrorMsg → set err
```

### Global Keys (always work regardless of page)

| Key | Action |
|-----|--------|
| `q`, `ctrl+c` | Quit + Shutdown |
| `?` | Toggle help |
| `1` | Switch to Stream page (unless searchFocused) |
| `2` | Switch to Library page (unless searchFocused) |
| `3` | Switch to Settings page (unless searchFocused) |
| `esc` | Close help / unfocus search / back |

**IMPORTANT:** `1`/`2`/`3` should NOT trigger when `searchFocused` is true, otherwise typing "123" in search would switch pages.

### Page Switch Logic

```go
func (m Model) switchPage(page Page) (Model, tea.Cmd) {
    m.activePage = page
    m.searchFocused = false
    m.showHelp = false
    
    // Reset search input text when switching TO library page (use as filter)
    // Keep search text when switching TO stream page (user might want to continue searching)
    
    switch page {
    case PageStream:
        m.searchInput.SetValue("")  // Clear filter; stream page uses search for new queries
        m.searchInput.Placeholder = "Search YouTube…"
        m.activePanel = PanelSearch  // Focus search results
    case PageLibrary:
        m.searchInput.SetValue("")  // Clear previous filter
        m.searchInput.Placeholder = "Filter library…"
        m.activePanel = PanelSearch  // Focus library list
    case PageSettings:
        m.activePanel = PanelSearch  // Focus settings list (reuse Panel type or new SettingsPanel)
        m.settingsCursor = 0
    }
    
    return m, nil
}
```

---

## View Function Architecture

```
View() → string

  └── switch m.activePage:
      ├── PageStream   → renderStreamPage(m)
      ├── PageLibrary  → renderLibraryPage(m)
      └── PageSettings → renderSettingsPage(m)
```

### renderStreamPage
Same as current view but:
- No download bar (remove `m.renderDownloadBar()`)
- Nav bar shows `[1] Stream  [2] Library  [3] Settings`
- Header: search placeholder says "Search YouTube…"

### renderLibraryPage
- Header: search placeholder says "Filter library…"  
- Left panel: `renderLibrary()` (same as current but wider)
- Right panel: `renderDownloadQueue()` (new)
- Player bar at bottom (same as current)
- Nav bar shows `[1] Stream  [2] Library  [3] Settings`

### renderSettingsPage
- Header: shows "Settings" on right
- Body: `renderSettingsList()` (new)
- Player bar at bottom
- Nav bar shows `[1] Stream  [2] Library  [3] Settings`

### Page Navigation Bar

```go
func (m Model) renderNavBar() string {
    tabs := []string{"1 Stream", "2 Library", "3 Settings"}
    var rendered []string
    for i, tab := range tabs {
        if int(m.activePage) == i {
            rendered = append(rendered, styleActiveTab.Render(tab))
        } else {
            rendered = append(rendered, styleInactiveTab.Render(tab))
        }
    }
    return lipgloss.JoinHorizontal(lipgloss.Left, rendered...)
}
```

---

## Download Queue UI (new right panel on Library page)

```go
func (m Model) renderDownloadQueue(width, height int) string {
    jobs := m.downloader.Jobs()
    if len(jobs) == 0 {
        return styleEmpty.Render("No downloads")
    }
    
    var sections []string
    
    // Active downloads
    active := filterByStatus(jobs, StatusDownloading)
    if len(active) > 0 {
        sections = append(sections, styleSectionHeader.Render("⬇ Active"))
        for _, j := range active {
            sections = append(sections, renderDownloadJob(j, width))
        }
    }
    
    // Pending
    pending := filterByStatus(jobs, StatusPending)
    if len(pending) > 0 {
        sections = append(sections, styleSectionHeader.Render("⏳ Pending"))
        // Show limited number + "and N more"
        for i, j := range pending {
            if i >= maxVisible { break }
            sections = append(sections, renderDownloadJob(j, width))
        }
        if len(pending) > maxVisible {
            sections = append(sections, styleMore.Render(fmt.Sprintf("  + %d more", len(pending)-maxVisible)))
        }
    }
    
    // Completed
    done := filterByStatus(jobs, StatusDone, StatusSkipped)
    if len(done) > 0 {
        sections = append(sections, styleSectionHeader.Render("✓ Completed"))
        for i, j := range done {
            if i >= maxVisible { break }
            sections = append(sections, renderDownloadJob(j, width))
        }
    }
    
    // Failed
    failed := filterByStatus(jobs, StatusFailed)
    if len(failed) > 0 {
        sections = append(sections, styleSectionHeader.Render("✗ Failed"))
        for _, j := range failed {
            sections = append(sections, renderDownloadJob(j, width))
        }
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, sections...)
}
```

### downloader.Jobs() — new method needed

```go
// internal/downloader/downloader.go
func (d *Downloader) Jobs() []*Job {
    d.mu.Lock()
    defer d.mu.Unlock()
    jobs := make([]*Job, len(d.jobs))
    copy(jobs, d.jobs)
    return jobs
}
```

---

## `x` Key Handler Changes

The `x` key is defined in `keys.go` (`Download` binding) but **currently not handled in update.go**.
We need to add it:

### On Stream page — download a queued track for offline

```go
case PageStream && m.activePanel == PanelQueue:
    i := m.queueOffset + m.queueCursor
    tracks := m.queue.Tracks()
    if i < 0 || i >= len(tracks) { return m, nil }
    t := tracks[i]
    if t.Downloaded {
        m.statusMessage = "Already downloaded: " + t.Title
        return m, nil
    }
    if t.URL == "" {
        m.statusMessage = "Cannot download: no URL for " + t.Title
        return m, nil
    }
    m.downloader.Enqueue(t.ID, t.Title, t.URL, downloadDir())
    m.statusMessage = "Download queued: " + t.Title
```

### On Library page — same (download a queued track from download queue)
Also: re-download a failed/downloaded track.

```go
case PageLibrary && downloadQueuePanelActive:
    // Get selected job from download jobs list
    job := m.selectedDownloadJob()
    if job == nil { return m, nil }
    if job.Status == StatusDownloading || job.Status == StatusPending {
        return m, nil // already downloading/pending
    }
    m.downloader.Enqueue(job.TrackID, job.Title, job.URL, downloadDir())
```

---

## SongEndedMsg — Enhanced Auto-Advance

Current behavior: skips undownloaded tracks entirely.
New behavior: plays ANY track that has either `FilePath` (downloaded) or `URL` (streamable).

```go
case SongEndedMsg:
    for {
        t, ok := m.queue.Next()
        if !ok {
            m.playerState = player.StateStopped
            m.position = 0
            return m, nil
        }
        m.queueCursor = m.queue.CurrentIndex()

        var playPath string
        switch {
        case t.Downloaded && t.FilePath != "":
            playPath = t.FilePath
        case t.URL != "":
            playPath = t.URL
        default:
            continue // skip — can't play this track
        }

        m.playerState = player.StatePlaying
        m.duration = float64(t.DurationSec)
        m.position = 0
        m.statusMessage = "Now playing: " + t.Title
        if err := m.player.Play(playPath); err != nil {
            m.err = err
            m.playerState = player.StateStopped
            return m, nil
        }
        return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
    }
```

---

## Enter Key Handler Changes

### Stream page — search result (enter)
```go
case PageStream && m.activePanel == PanelSearch && !m.searchFocused && len(m.results) > 0:
    i := m.searchOffset + m.searchCursor
    if i >= len(m.results) { return m, nil }
    r := m.results[i]
    t := searchResultToTrack(r)
    m.queue.Add(t)
    m.queue.SetCurrentIndex(m.queue.Len() - 1)
    m.queueCursor = m.queue.CurrentIndex()
    m.playerState = player.StatePlaying
    m.duration = float64(t.DurationSec)
    m.position = 0
    m.statusMessage = "Now playing: " + t.Title
    if err := m.player.Play(t.URL); err != nil {
        m.err = err
        m.playerState = player.StateStopped
        return m, nil
    }
    return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
```

### Library page — library track (enter)
```go
case PageLibrary && m.activePanel == PanelSearch && !m.searchFocused:
    tracks := m.filteredLibrary()
    i := m.libraryOffset + m.libraryCursor
    if i >= len(tracks) { return m, nil }
    t := tracks[i]
    m.queue.Add(t)
    m.queue.SetCurrentIndex(m.queue.Len() - 1)
    m.queueCursor = m.queue.CurrentIndex()
    m.playerState = player.StatePlaying
    m.duration = float64(t.DurationSec)
    m.position = 0
    m.statusMessage = "Now playing: " + t.Title
    if err := m.player.Play(t.FilePath); err != nil {
        m.err = err
        m.playerState = player.StateStopped
        return m, nil
    }
    return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
```

### Queue panel — existing track (enter)
Same on both pages — play the selected queued track.

```go
case m.activePanel == PanelQueue:
    i := m.queueOffset + m.queueCursor
    tracks := m.queue.Tracks()
    if i >= len(tracks) { return m, nil }
    t := tracks[i]
    
    // Set as current
    m.queue.SetCurrentIndex(i)
    m.queueCursor = i
    
    var playPath string
    switch {
    case t.Downloaded && t.FilePath != "":
        playPath = t.FilePath
    case t.URL != "":
        playPath = t.URL
    default:
        m.statusMessage = "Cannot play: " + t.Title + " (no URL or file)"
        return m, nil
    }
    
    m.playerState = player.StatePlaying
    m.duration = float64(t.DurationSec)
    m.position = 0
    m.statusMessage = "Now playing: " + t.Title
    if err := m.player.Play(playPath); err != nil {
        m.err = err
        m.playerState = player.StateStopped
        return m, nil
    }
    return m, tea.Batch(positionCmd(m.player), endedCmd(m.player))
```

---

## Download Bar Removal

The `renderDownloadBar()` is removed from `renderStreamPage()` but kept in `renderLibraryPage()`.

Actually, the download bar is no longer needed as a floating bar. Instead:
- Download progress shows in the **Downloads panel** on Library page
- Brief status updates appear in the **status message** line (below player bar)
- When a download completes, a transient status message says "Download complete: Title"

---

## File Changes Summary

| File | Action | Lines Changed |
|------|--------|------|
| `internal/tui/model.go` | Add Page type, settings fields, `activePage`, `downloadJobs`; remove `showingLibrary` | ~50 |
| `internal/tui/update.go` | Route by page, change Enter/SongEnded/`x` handlers, add page-specific key handlers | ~100 |
| `internal/tui/view.go` | Split into page renderers, add settings renderer, remove download bar, add nav bar | ~200 |
| `internal/tui/keys.go` | Add page-switch bindings, update help | ~15 |
| `internal/tui/styles.go` | Add nav tab styles, settings styles | ~30 |
| `internal/settings/settings.go` | **NEW** — load/save JSON config | ~80 |
| `internal/downloader/downloader.go` | Add `Jobs()` method | ~10 |
| `internal/queue/queue.go` | No changes needed | 0 |
| `internal/player/player.go` | No changes needed (Play() already handles URLs) | 0 |
| `internal/search/search.go` | No changes needed | 0 |
| `internal/library/library.go` | No changes needed | 0 |
| **TOTAL** | | **~485** |

---

## Implementation Order

### Phase 1: Foundation (model + page navigation)
1. Add `Page` type constants to model.go
2. Add `activePage`, `settings`, `settingsCursor`, `settingsEditField`, `settingsEditInput` to Model
3. Add `queuePanel` to Model (for library page dual-panel focus)
4. Remove `showingLibrary` from Model
5. Create `internal/settings/settings.go` with `Load()`, `Save()`, `Defaults()`
6. Update `InitialModel()` to load settings
7. Add global `1`/`2`/`3` key handling in update.go
8. Add `switchPage()` helper

### Phase 2: Stream Page (refactor existing view)
1. Remove `renderDownloadBar()` from Stream page
2. Add `renderNavBar()` to styles.go + view.go
3. Change Enter handler to play URL (not download)
4. Change SongEndedMsg to stream by URL when not downloaded
5. Add `x` handler to enqueue download for offline
6. Remove `showingLibrary` toggle from `L` key (L becomes a no-op or page-switch on Stream page)

### Phase 3: Library Page
1. Add `renderLibraryPage()` wrapping existing library renderer
2. Add `renderDownloadQueue()` for right panel
3. Add `Jobs()` method to downloader
4. Wire tab focus between library list and download queue
5. Add `d` delete handler for library tracks
6. Add download status display

### Phase 4: Settings Page
1. Add `renderSettingsPage()` and `renderSettingsList()`
2. Add settings editing key handlers
3. Wire settings changes to live model updates (volume, search limit)

### Phase 5: Polish
1. Update help texts and key descriptions
2. Update ShortHelp/FullHelp for each page context
3. Handle mouse clicks on all pages
4. Test edge cases
5. `go build ./...` to verify compilation
