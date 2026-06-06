# Backlog

> Feature proposals and enhancements for ytmgo, ordered by priority tier.
> Items marked **MVP** are considered essential before a 1.0 release.

---

## Tier 1 — High Priority

### User Authentication & Account Linking
**Status:** ❌ Not started  
**Effort:** Medium  
**Depends on:** None  

Integrate Google OAuth to sign in with a YouTube Music / Google account. This unlocks access to the user's personal library — liked songs, uploaded tracks, subscriptions, and watch-history-based recommendations — rather than relying solely on anonymous search.

- OAuth token storage (encrypted, locally persisted)
- Authenticated API requests to YouTube Music endpoints
- Graceful degradation when offline or unauthenticated

---

### Playlist Management
**Status:** ❌ Not started  
**Effort:** Large  
**Depends on:** User Authentication & Account Linking  

Full CRUD lifecycle for playlists sourced from the user's YouTube Music account:

- **List** — browse owned and subscribed playlists in a dedicated panel
- **View** — drill into a playlist to see its tracks with metadata (duration, artist, album art fallback)
- **Create** — new empty playlist with a title and optional description
- **Edit** — rename, reorder tracks via drag or keyboard (Ctrl+↑/↓), add/remove items
- **Delete** — remove a playlist with confirmation
- **Import** — parse external playlist formats (M3U, CSV, YouTube Music share URLs) into a new playlist
- **Export** — serialize a playlist to M3U or JSON for backup or sharing

UI considerations:
- Dedicated "Playlists" view accessible from the Library page (tab `2`)
- Right-panel detail view when a playlist is selected
- Queue integration: "Add entire playlist to queue" action

---

### Radio / Autoplay Mode
**Status:** 🟢 Active  
**Effort:** Medium  
**Depends on:** Player module (existing)  

When the queue is exhausted, the application automatically enqueues suggested tracks based on recent listening history.

- Fetches TIDAL recommendations when queue runs dry
- Explicit opt-in: Settings page toggle (default: ON)
- Repeat modes (OFF / ONE / ALL) compose naturally: autoplay only kicks in when ALL is OFF and queue is truly exhausted
- Status bar shows "Autoplay fetching suggestions…" while loading
- Does not interrupt if user manually adds tracks before suggestions arrive

---

### Discord Rich Presence
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** Player module (existing), optional: Presence image assets  

Expose now-playing state to Discord via the Discord RPC protocol so friends can see what you're listening to:

- **Large image** — album art or ytmgo icon as the "cover"
- **First line** — track title
- **Second line** — artist name
- **Elapsed time** — live playback progress
- **Buttons** — "Listen Along" or project link
- **Disconnect** — clear presence when playback stops or ytmgo exits
- **Configurable** — toggle in Settings, with an option to hide track metadata for privacy

---

## Tier 2 — Medium Priority

### Mixes Management
**Status:** ❌ Not started  
**Effort:** Medium  
**Depends on:** User Authentication & Account Linking  

YouTube Music generates auto-updating "mixes" for genres, moods, and artists. Surface these in the TUI:

- **List** — fetch and display available mixes from the user's account
- **Play** — enqueue an entire mix as a starting point
- **Refresh** — regenerate a mix to get fresh suggestions (analogous to `R` for recommendations)
- **Save / unsave** — add a mix to a "Saved mixes" collection for quick access
- **Detail view** — show which tracks are in the current mix iteration
- **Visual grouping** — distinct icon or label to differentiate mixes from regular playlists

---

### Library Persistence & Local State
**Status:** ❌ Not started  
**Effort:** Medium  
**Depends on:** None  

Persist application state across sessions so restarting ytmgo feels seamless:

- **Play history** — last N played tracks with timestamps, stored in a local SQLite or JSON store
- **Queue recovery** — optionally restore the previous queue on startup
- **Favorites** — allow marking tracks / albums / artists as favorites locally (not synced to YouTube)
- **Search history** — recent queries for quick re-search

Storage choice: embed a lightweight store (BoltDB, SQLite via CGO-free driver, or a plain JSON file). JSON is simplest for a v1; structured DB for v2.

---

### MPRIS Integration
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** Player module (existing)  

Implement the MPRIS D-Bus interface (media-player2 specification) so ytmgo appears as a media player in the Linux desktop ecosystem:

- **Controls** — play/pause, next, previous, seek, volume from any MPRIS client (media keys, GNOME lock screen, KDE plasma, etc.)
- **Metadata** — expose track title, artist, album art URL, duration
- **Playback status** — report Playing / Paused / Stopped accurately
- **Loop / shuffle** — reflect current repeat and shuffle state
- **Desktop notification** — optional song-change notification via MPRIS events

---

### Search Filters & Advanced Querying
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** None  

Enhance the search panel with scoped and filtered queries:

- **Type filter** — restrict search to Songs / Albums / Artists / Playlists / Videos
- **Sort** — relevance, date, view count, rating
- **Date range** — filter by upload date (last hour, day, week, year)
- **Inline hints** — show available filter syntax in the search input placeholder (like `type:album`)

---

## Tier 3 — Backlog / Future Consideration

### Last.fm Scrobbling
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** Library Persistence & Local State  

Authenticate with Last.fm and automatically scrobble played tracks:

- Handshake via Last.fm API (API key + session)
- Scrobble on track change (or after 50% play duration per Last.fm rules)
- "Now Playing" periodic update
- Configurable: enable/disable, account linking from Settings UI

---

### Custom Theming System
**Status:** ❌ Not started  
**Effort:** Medium  
**Depends on:** None  

Allow users to define custom color schemes and layout preferences:

- **Theme file** — load a TOML or YAML config specifying Lipgloss color values for each component (header, panels, borders, progress bar, etc.)
- **Built-in presets** — ship 2–3 curated themes (catppuccin, dracula, gruvbox)
- **Live reload** — re-apply theme on file change without restart
- **Settings UI** — theme selector dropdown in the Settings page

---

### Podcast Support
**Status:** ❌ Not started  
**Effort:** Large  
**Depends on:** User Authentication & Account Linking, possibly a dedicated download pipeline  

YouTube Music hosts podcasts natively. Surface them in the TUI:

- **Browse** — discover podcasts via search with a `type:podcast` filter
- **Subscribe** — manage podcast subscriptions synced with the user's account
- **Episodes** — list episodes with publish date, duration, and play-progress tracking
- **Download management** — batch download episodes for offline listening
- **Playback** — resume from last position per episode
- **UI** — distinct iconography and a "Podcasts" section on the Library page
---

### Equalizer & Audio Pipeline
**Status:** ❌ Not started  
**Effort:** Large  
**Depends on:** Player module (existing), mpv configuration  

Leverage mpv's built-in audio filter chain to expose an equalizer:

- **Equalizer presets** — Flat, Bass boost, Vocal, Classical, Rock, Jazz, Custom
- **Real-time adjustment** — keybindings or slider in the TUI to adjust gain per band
- **Persistence** — selected preset survives restart
- **Settings UI** — a dedicated equalizer view with per-band +/- controls

---

### Export / Import Queue
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** None  

Allow the user to save and restore the current queue as a portable file:

- **Export** — serialize current queue (tracks + ordering) to M3U or JSON
- **Import** — load a queue from file, appending or replacing the current queue
- **Share format** — M3U is widely supported by other media players

---

### Keyboard Shortcut Customization
**Status:** ❌ Not started  
**Effort:** Small  
**Depends on:** None  

Move all keybindings into a user-editable config so power users can remap to their preference:

- **Config file** — TOML map of action → key(s)
- **Validation** — warn on startup if a binding conflicts with an existing one
- **Defaults** — ship the current bindings as the default config
- **Settings UI** — read-only listing that points to the config file path

---

## Legend

| Status | Meaning |
|--------|---------|
| 🟢 Active | Being worked on this cycle |
| 🔵 Ready | Spec is complete, can be picked up |
| ❌ Not started | No work has begun |
| 🟡 Blocked | Waiting on a dependency |
| ✅ Done | Delivered, in main |

---

*Last updated: 2026-06-06*
