# Backlog

> What is built, what is not, and what is next. Statuses here are checked
> against the code, not against memory — the previous version of this file
> had shipped features listed as not started.

---

## Shipped

Everything below is in main and working.

| Feature | Where |
|---|---|
| **Search** — songs and albums, straight against YouTube Music's own API, no key or login | `internal/ytmusic` |
| **Playback** — mpv under the hood, queue with shuffle and repeat OFF / ONE / ALL | `internal/player`, `internal/queue` |
| **Autoplay radio** — when the queue runs dry, enqueues what YouTube Music would play next | `internal/ytmusic`, `[c]` on the player bar |
| **Downloads** — one key per track, whole albums into numbered folders, on their own page | `internal/downloader` |
| **Library** — everything already on disk, filterable | `internal/library` |
| **Favourites and play history** — both persisted | `favorites`, `play_history` tables |
| **Queue recovery** — the queue comes back on restart | `db.LoadQueue` |
| **Synced lyrics** — LRCLIB, following the song, cached locally; `y` remembers whether the pane was open | `internal/lyrics` |
| **Album previews** — tracklist, cover art, queue or download the lot | `internal/tui` |
| **Artist pages** — top songs and full discography, from any track | `ytmusic.Artist`, `[i]` |
| **MPRIS** — media keys and desktop widgets; silently absent without a session bus | `internal/mpris` |
| **Discord Rich Presence** — now-playing, toggleable | `internal/discordrpc` |
| **Spectrum visualiser** — via cava, optional; `v` remembers whether the pane was open | `internal/visualizer` |
| **Eleven themes** — the terminal's own colours, ytmgo's, and nine full schemes | `internal/tui/theme.go` |
| **Full mouse support** — tabs, lists, seek bar, volume, transport, mode toggles | `internal/tui/mouse.go` |
| **Album art in the terminal** — kitty graphics where available | `internal/coverart` |

---

## Before 1.0

Nothing outstanding. The list was a licence file, the database moving
out of the config directory, live API tests off the CI gate, downloader
test coverage, and this file being wrong — all done.

Search history was on this list and moved below. It is a convenience,
not a promise about the interface, and shipping 1.0 does not make it
harder to add.

---

## After 1.0

### Search history
**Status:** ❌ Not started · **Effort:** Small

Recent queries, for re-running one without retyping. The last piece of
the original persistence plan; the other four shipped.

### User authentication
**Status:** ❌ Not started · **Effort:** Medium

Google OAuth, to reach a personal library: liked songs, uploads,
subscriptions, history-based recommendations. Everything today is
anonymous, which is a feature — no login, no key, nothing to leak — so
this has to stay strictly optional, with the anonymous path unaffected.

Blocks playlists and mixes below.

### Playlist management
**Status:** ❌ Not started · **Effort:** Large · **Needs:** authentication

List, view, create, rename, reorder, delete. Import from M3U, CSV or a
share URL; export to M3U or JSON. "Queue whole playlist" as an action.

### Mixes
**Status:** ❌ Not started · **Effort:** Medium · **Needs:** authentication

YouTube Music's auto-updating genre, mood and artist mixes: list, play,
refresh, save.

### Search filters
**Status:** ❌ Not started · **Effort:** Small

`type:` (songs / albums / artists / playlists), sort order, upload-date
range. The albums toggle (`A`) is the only filter today.

### Custom theme files
**Status:** ❌ Not started · **Effort:** Medium

Eleven built-in themes ship. This is the next step: a TOML or YAML file
defining the same colour roles, live-reloaded on change. The roles are
already named in `scheme` in `internal/tui/theme.go`, so the shape of the
file is decided — it needs loading, validation, and a contrast check so a
hand-written theme cannot make its own selected rows unreadable.

### Keybinding customisation
**Status:** ❌ Not started · **Effort:** Small

Bindings in a user-editable config, with conflict warnings at startup.
Today's bindings become the shipped defaults.

### Last.fm scrobbling
**Status:** ❌ Not started · **Effort:** Small

Scrobble on track change, periodic "now playing", account linking in
settings.

### Queue export / import
**Status:** ❌ Not started · **Effort:** Small

Save and restore the current queue as M3U or JSON. Independent of
playlists — needs no account.

### Equalizer
**Status:** ❌ Not started · **Effort:** Large

Through mpv's audio filter chain: presets, per-band adjustment,
persistence.

### Podcasts
**Status:** ❌ Not started · **Effort:** Large · **Needs:** authentication

Browse, subscribe, episode lists with per-episode resume, batch download.

---

## Legend

| Status | Meaning |
|--------|---------|
| 🟢 Active | Being worked on now |
| 🔵 Ready | Specced, can be picked up |
| ❌ Not started | No work has begun |
| 🟡 Blocked | Waiting on a dependency |
| ✅ Done | In main — see the Shipped table |
