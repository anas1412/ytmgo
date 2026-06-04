# Implementation Plan — Persistence & Favorites

Three phases, ordered by value/dependency.

---

## Phase 1 — Queue Persistence (JSON)

**Goal:** Save/restore queue state across sessions so a crash or restart doesn't lose the playlist.

### Data

Single JSON file at `~/.config/ytmgo/queue-state.json`:

```json
{
  "tracks": [ ... ],
  "current_index": -1,
  "shuffle": false,
  "repeat": false,
  "repeat_all": false
}
```

### Files to change

| File | Change |
|------|--------|
| `internal/queue/queue.go` | Add `SaveState(path string) error` and `LoadState(path string) error` — marshal/unmarshal tracks + metadata fields. Tracks() already returns a copy; LoadState replaces the internal slice + index + flags. |
| `internal/tui/commands.go` | Add `saveQueueCmd(q *queue.Queue) tea.Cmd` and `loadQueueCmd(q *queue.Queue) tea.Cmd`. Path resolved via `settings.configPath()` dir + `"queue-state.json"`. |
| `internal/tui/model.go` | Add `QueueLoadedMsg` struct (carries `error`). |
| `internal/tui/update.go` | Add `QueueLoadedMsg` case — report error or show "Queue restored (N tracks)" via setStatus. Fire `loadQueueCmd` in `Init()`. |
| `internal/tui/keyboard.go` | tea.Batch `saveQueueCmd` after every queue mutation: add track, remove, clear, move up/down, shuffle, repeat. |

### Behaviors

- `Init()` fires `loadQueueCmd` alongside existing startup commands
- Every queue mutation in `handleKey` appends `saveQueueCmd` to the batch
- First run (no file) → silent no-op, start with empty queue
- If JSON is corrupt → silently start empty, overwrite on next mutation

### Done when

- Restart ytmgo → queue looks exactly as it was before quitting
- Adding/removing/moving/reordering/shuffling tracks persists
- First run with no file starts with empty queue (no error)

---

## Phase 2 — Favorites Tab & Management

**Goal:** Users can favorite tracks from the queue or library, browse them in a dedicated tab, and unfavorite them.

### UI Layout

- New page `PageFavorites` (key `4`)
- Layout mirrors Library: left panel = favorited tracks list, right panel (PanelQueue) = queue
- Tab `4` in header: "Favs"

### Data

Another JSON file `~/.config/ytmgo/favorites.json`:

```json
[
  { "id": "...", "title": "...", "artist": "...", "duration": "...", "duration_sec": 123, "added_at": "..." }
]
```

### Files to change

| File | Change |
|------|--------|
| `internal/tui/model.go` | Add `favorites []queue.Track`, `favCursor int`, `favOffset int`, `PageFavorites` constant, `FavoritesMsg`/`FavoriteAddedMsg`/`FavoriteRemovedMsg` message types |
| `internal/tui/commands.go` | Add `loadFavoritesCmd`, `saveFavoritesCmd` — same JSON pattern as queue |
| `internal/tui/update.go` | Wire `FavoritesMsg`, `FavoriteAddedMsg`, `FavoriteRemovedMsg` cases. Add `loadFavoritesCmd` to `Init()`. |
| `internal/tui/keys.go` | Add `PageFavorites` key.Binding with `"4"` |
| `internal/tui/keyboard.go` | Add `case "4":` to Globals. Add keybinding in main switch for favoriting (`F`?) — toggles favorite on highlighted track. Favorite from: queue panel, library panel, search results. |
| `internal/tui/view.go` | Add `renderFavorites()` — same pattern as `renderLibrary()`. Add `PageFavorites` case in main view switch. Add "Favs" to header tabs. |
| `internal/tui/mouse.go` | Handle click on favorites tab. Handle mouse wheel on favorites list. |
| `internal/tui/layout.go` | Update layout constants, tab highlight logic |

### Behaviors

- `F` key on any track (queue, library, search result) toggles favorite on/off
- Favorite icon (e.g. `♥`) shown next to title in queue/library when favorited
- Tab `4` shows all favorited tracks, same interaction as library (Enter to add to queue, `d` to unfavorite)
- Sorting: most recently favorited first
- Click to unfavorite from the favorites list itself

### Done when

- `4` switches to favorites view showing all favorited tracks
- `F` toggles favorite on any track from queue/library/search
- Favorite state shows up as `♥` icon next to title
- Unfavorite from favorites list with `d` or `F`
- Favorites survive restart

---

## Phase 3 — SQLite Migration

**Goal:** Replace all JSON files with a single `ytmgo.db` file using `modernc.org/sqlite` (CGO-free). Add play history as a bonus.

### New package

`internal/db/db.go` — all SQLite access behind a clean API.

### Schema

```sql
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS queue_state (
    id          INTEGER PRIMARY KEY CHECK (id = 1),
    tracks      TEXT NOT NULL,       -- JSON array
    current_idx INTEGER NOT NULL DEFAULT -1,
    shuffle     INTEGER NOT NULL DEFAULT 0,
    repeat      INTEGER NOT NULL DEFAULT 0,
    repeat_all  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS favorites (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id  TEXT NOT NULL UNIQUE,
    title     TEXT NOT NULL,
    artist    TEXT NOT NULL DEFAULT '',
    duration  TEXT NOT NULL DEFAULT '',
    duration_sec INTEGER NOT NULL DEFAULT 0,
    added_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE TABLE IF NOT EXISTS play_history (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id  TEXT NOT NULL,
    title     TEXT NOT NULL,
    artist    TEXT NOT NULL DEFAULT '',
    played_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);
```

### Migration

- On first open of `ytmgo.db`, check if `queue-state.json` or `favorites.json` exist
- If yes, read them and write to the SQLite tables, then rename JSON files to `.json.migrated`
- Clean break — no need to support both formats going forward

### Files to change

| File | Change |
|------|--------|
| `internal/db/db.go` | **(new)** Open/Close + all table methods |
| `internal/settings/settings.go` | Optionally migrate from `settings.json` → `settings` table (or keep JSON for human-editable settings) |
| `internal/tui/model.go` | Replace `db *db.DB` with single connection |
| `internal/tui/commands.go` | Replace `saveQueueCmd`/`loadQueueCmd`/`saveFavoritesCmd` with SQLite versions. Add `recordPlayCmd`. |
| `internal/tui/update.go` | Wire play history recording. Migration logic in Init. |
| `internal/tui/keyboard.go` | Add `recordPlayCmd` when a track starts playing |
| `go.mod` | Add `modernc.org/sqlite` |

### Done when

- All data in single `ytmgo.db`
- Queue, favorites, settings all work as before
- Play history is recording silently
- JSON files from Phase 1-2 are migrated automatically

---

## Key Principles (all phases)

- **No data loss** — migrations read old format before writing new one
- **Atomic writes** — write to temp file then rename (`os.WriteFile` → already fine for small JSON, same pattern for SQLite)
- **Graceful degradation** — corrupted data starts empty, not crashing
- **Silent background** — no "Saving..." status messages on routine mutations (only errors)
- **Same pattern as settings** — config dir, JSON marshal/indent, os.WriteFile, tea.Cmd pattern
