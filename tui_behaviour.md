# What the TUI does, and what the GUI copies

The Windows GUI (`win_migration.md`) must behave like the TUI. This is the
TUI's behaviour as the code has it at v1.4.0, read from the code, not the
docs. Paths are under `internal/` unless shown otherwise. Where docs and code
disagree, the code is what is written here.

Each section ends with **GUI:** what the GUI does with it. Things the GUI
deliberately does differently are said there; bugs it must not copy are
collected at the end.

---

## 1. Queue

- A list of tracks, a current index (-1 = nothing playing), shuffle, repeat
  one, repeat all (`queue/queue.go:38-50`). Restored at start with
  current = -1 (`queue.go:346-360`); `current_idx` is saved but never read.
  Position is never saved.
- **Adding** (`tui/actions.go:26-41`, `enqueueAndMaybePlay`): Enter on any
  track row appends it to the end. If the player is stopped it starts that
  track ("Now playing: T"); otherwise "Added to queue: T". Paused does not
  start it. **No duplicate check**: the same song can be queued many times.
  There is no "play next".
- **Queue everything listed** with `e` (Stream list only): "Queued N tracks",
  "Nothing to queue", "Open an album to queue its tracks". `a` inside an album
  queues the album: "Queued N tracks from <album>" (`tui/keyboard.go:400-432`).
- **Remove** `d` (queue focused): no message. **Clear** `D`: confirm
  "Clear queue?", then stop and "Queue cleared" (`tui/confirmation.go:46-57`).
- **Reorder** `ctrl+↑/↓` (queue focused, Stream page only); the playing track
  stays the playing track (`queue.go:94-120`).
- **Rows**: `▶` on the current track (even when stopped), `N. Title`, then
  `✓` if downloaded, artist, `♥` if favourite, duration. Header
  `QUEUE N · H:MM:SS` (`H:MM:SS` when over an hour, else `M:SS`). Empty:
  "Queue is empty" (`tui/view.go:580-589, 1686-1775`).
- Saved after every change (`tui/commands.go:688-700`), never on quit.

**GUI:** the queue lives in Go (`gui/queue.go`), saved after every change,
shared with the TUI on Linux. Copied: append on Enter, play if stopped;
`e` and `a` queue a whole list; remove; clear with a confirmation; markers
`▶ ✓ ♥`; header count and total time. Differs on purpose:

- Double-click plays now (Spotify's convention), placed straight after the
  playing song; Enter keeps the TUI's meaning (append, play if stopped).
- Dropping a song already queued moves it instead of duplicating it. Adding
  one with Enter or `+` still appends, as the TUI does.
- Reordering by dragging, on any page, plus `ctrl+↑/↓`. Songs can be dragged
  from any list into the queue at a chosen position.
- Removing the playing song is not offered (the TUI leaves it playing while
  the marker moves on).

## 2. Playback

- **Start** (`tui/model.go:615-728`): a local file if the track is
  downloaded and the file exists; else the watch URL (mpv resolves it); else
  a cached or resolved URL ("Fetching URL…"). Then "Now playing: T", history
  is recorded, Last.fm now-playing is sent, cover and lyrics load, and the
  next track's URL is prefetched.
- **End**: a natural end moves on with the queue's `Next()`: repeat-one
  plays the same song again. **End of the queue**: autoplay (§3) or stop with
  "Queue empty"; the queue itself is kept (`tui/async.go:576-619`).
- **Failures**: a song mpv cannot open is skipped; three in a row stop with
  "Nothing will play — mpv could not open 3 tracks in a row. Update yt-dlp:
  yt-dlp -U" (`async.go:552-567`). A URL that cannot be resolved stops with
  "Cannot play 'T': <err>".
- **Next** `n`/`→`: skips even under repeat-one; at the end without repeat-all
  nothing happens. **Previous** `p`/`←`: past 3 s restarts the song
  ("Restarting"); otherwise the song before; at the first song it reloads it
  (`tui/actions.go:210-247`).
- **Seek** `l`/`h` ±5 s; the seek bar seeks on click. **Volume** `+`/`-` ±5,
  0–100, saved as the Default Volume setting on every change; the mouse shows
  "Volume: N%". **Space** play/pause.
- **Shuffle** `s` ("Shuffle: ON/OFF"): a new random order around the current
  song. **Repeat** `r` cycles OFF → ONE → ALL ("Repeat: ONE/ALL/OFF").
- **Player bar**: `▶ Title — Artist`, the next song right-aligned as
  `⏭ <title>` (none under repeat-one or at the end), `○ <album>` (hidden when
  the album field is really a play count: ends in " plays"/" views"),
  Prev / Play-Pause / Next, `∞ AUTO`, `⇄ SHFL`, `↻ OFF|ONE|ALL`, volume bar
  and `NN%`, times `MM:SS` or `--:--`. Idle: "Ready — search and add tracks",
  or "Playback finished" when the queue is not empty (`tui/view.go:2068-2181`).

**GUI:** all of it, through the Go queue. Streams come from yt-dlp per song
(they expire), played by `<audio>`. The next song is on the Next button
("Next: <title>") and marked in the queue panel; the album, with the same
play-count filter, is on hover over the title. Shuffle and repeat buttons
in the player (repeat-one carries a small 1), `s` and `r`. Volume is saved to the settings row like the TUI's,
not to browser storage. Failures: one unplayable song is skipped and named;
three in a row stop. Offline and timeouts stop and say to press play.

## 3. Autoplay radio

- Setting `AutoplayEnabled`, default on; `c` or clicking `∞ AUTO` toggles it
  ("Autoplay: ON/OFF"). No row on the Settings page.
- **Fetches one song** when the last song has 30 s or less left (not under
  repeat), or when the queue runs out (`async.go:426-435, 582-608`).
- **Seeds**: up to 4 newest unique songs from history; the radio of the
  first 2 is used, and all 4 are excluded (`tui/commands.go:146-201`,
  `search/search.go:67-110`). Never checked against the queue.
- Result: appended. If playing, "Autoplay: N suggestions added to queue";
  otherwise it starts playing ("Autoplay: playing suggestions").

**GUI:** same triggers, seeds and batch, in Go. Not copied: the stuck latch
(a failed fetch leaves "Autoplay loading suggestions…" forever), and a
result replacing a paused song. Also skips suggestions already in the queue.

## 4. History, favourites, Last.fm, Discord, media keys

- **History** is recorded when a song **starts**, not after a while; playing
  the same song twice in a row only updates its time (`db/db.go:343-363`).
  The History page shows the newest 100, reloaded on every visit, with
  relative times: "just now", "N min ago", "N hours ago", "N days ago", then
  "Jan 2". `C` confirms "Clear history?" then "History cleared".
- **Favourites** `f` on any track row: "Added to favorites: T" /
  "Removed from favorites: T"; newest first; `♥` on search, album, artist,
  library, queue and favourite rows (not history, not the player).
- **Last.fm**: now-playing at every start; scrobble once per play when the
  song is over 30 s long and has played half or 4 minutes, with the start time
  as the timestamp (`lastfm/lastfm.go:67-72`). No retry, no offline queue.
  Connect from Settings: a link opens in the browser, the app polls every 3 s
  for 10 minutes, "connected as <user>".
- **Discord**: title, artist, cover, elapsed time while playing; "Browsing
  Music" when idle. Default on.
- **MPRIS** (Linux media keys): play/pause, next, previous, stop; title,
  artist, art, shuffle, loop.

**GUI:** all of these, in Go. Windows media keys and the system media
overlay come from the Media Session API in the page (spike S3); Discord's
IPC works on Windows through its named pipe. Favourites keep their order
across restarts and keep album and artist ids (the TUI loses both).

## 5. Search and pasted links

- Enter searches; there is no search-as-you-type. Songs only (the albums
  toggle is gone). Limit = Search Limit setting, default 20, 5–100 in steps
  of 5. "Found N results", "No results found", "Search failed: <err>".
- Results match local library files by artist and title; a match plays the
  file (`tui/library.go:45-102`).
- Clearing the box brings recommendations back, from the cached batch.
- **Pasted links** on Enter (`search/playlist.go`): a YouTube or YouTube
  Music link with `list=`, or a Spotify **playlist** link (not album or
  track). Spotify tracks are matched on YouTube Music: 6 at a time, the first
  hit within ±5 s of the Spotify length, else the closest length. Status
  "<Title> — N tracks[, M not found on YouTube]". Nothing is queued until
  `e`. YouTube playlists read only the first page.

**GUI:** search as you type after a pause, plus Enter (asked for). Results
come with `♥` and `✓`. Pasted links the same, including the notice; the
home view keeps its cached recommendations. Spotify album and track links
are worth adding later; not in the TUI today.

## 6. Recommendations (home)

- Shown when the search is empty: at start, on clearing the box, and on `R`
  ("Loading recommendations…"). Seeds as in §3, count = Search Limit.
  "N recommendations", "No recommendations available". Empty after load:
  "Play something and recommendations will follow  ([R] retry)".

**GUI:** the home view of the main panel when the search is empty, with
**Refresh** and `R`.

## 7. Album and artist pages

- **Album** (`a` on a song, Enter on a release): art, title, `Artist · Year`,
  `N tracks · H:MM:SS`, numbered tracks with play counts and a second artist
  when it differs. `a` queues the whole album. Esc goes back (to the artist's
  releases if the album came from there). "Opening <album>…", "No album page
  for X", "Cannot open album: <err>"; Esc while loading cancels.
- **Artist** (`A` on a song): name, `<subs> subscribers · N releases`,
  **top songs** (up to 100) or **releases** (albums, then singles and EPs);
  `A` switches between them. The search box filters the page as you type.
  Esc goes back to the results.

**GUI:** both as pages in the main view with a header (cover or photo,
title, stats), and Back. Buttons for queue all, download album, and
songs ⇄ releases; the same keys. Back keeps a history, so album over artist
over results unwinds in order.

## 8. Library and downloads

- **Library**: the download folder plus the user's folders (Settings, a
  folder picker). Scanned at start and when folders change; files are only
  re-read when their modification time changes; tags read with ffprobe; files
  without tags use `Artist - Title` from the name in the download folder, the
  file name elsewhere. Rows show `✓`, artist (or "Unknown artist") and the
  parent folder. The search box filters by title or artist. `d` deletes the
  file from disk after "Delete "T"?". Empty: "Nothing here yet — download
  something, or add your music folders in Settings".
- **Downloads** `x` on a song: "Already in download queue: T", "Already
  downloaded: T", else queued, the Downloads page opens, "Download queued: T".
  `x` on a release downloads the album into `Artist - Album/` as
  `NN - Title`. One download at a time. yt-dlp extracts M4A (default) or MP3,
  embeds metadata and the 544 px cover. The page shows the active download
  with a bar, then Pending, Completed, Failed (with the error). `o` opens the
  download folder.
- **Playback mode** setting: stream only, download while playing, download
  first.

**GUI:** all of it. The Downloads page gets an **Open downloads folder**
button (Explorer). Folder adding uses the native dialog. Album downloads go
into the album folder (the TUI writes them to the root folder by mistake).

## 9. Lyrics and visualizer

- **Lyrics** from LRCLIB, synced when available, otherwise plain; YouTube
  Music's plain lyrics as the fallback. Cached, including "not found";
  cached lyrics work offline. The current line is the last one whose time has
  been reached, kept a third of the way down; scrolling stops following until
  the next song. "Loading lyrics…", "No lyrics found", "Lyrics unavailable:
  <err>", "Play a track to see its lyrics".
- **Visualizer**: cava, Linux only.

**GUI:** lyrics in the right panel's Lyrics tab, the same sources and
cache, with the current line followed smoothly; clicking a synced line seeks
there. The visualizer needs an audio source in the page (a Web Audio analyser
on the `<audio>` element) and comes last, if at all.

## 10. Settings and the rest of the interface

Settings rows, in order (`tui/settingsdef.go:44-235`): Playback Mode;
Default Volume (0–100, steps of 5); Library Folders; Download Dir; Download
Format (M4A ⇄ MP3); Search Limit (5–100, steps of 5); Theme (terminal, ytmgo,
gruvbox, nord, dracula, catppuccin, tokyo-night, rose-pine, everforest,
solarized-light, latte); Show Quotes; Copy Link As (youtube.com ⇄
music.youtube.com); Discord RPC; Last.fm Scrobbling. Without a row: Autoplay,
hints, visualizer, lyrics (keys `c`, `z`, `v`, `y`). Each change is saved at
once.

Also: `u` copies the song's link ("Copied link: <url>"); `?` opens Settings,
which lists every shortcut; the footer shows the version and "up to date" or
"Update <cur> → <new> — press U"; a quote or tip when idle; tabs `1`–`6`
(Stream, Favs, Library, History, Downloads, Settings).

**GUI:** the same settings, saved to the same row, as a Settings page.
Themes become the GUI's own palettes later; the TUI's names do not all suit
a desktop app. Copy link uses the Wails clipboard. Updates come from the
installer (Phase 5), not `U`. A `?` overlay lists the keys. No quote line.

---

## TUI bugs found, not to copy

Found while reading. They are bugs in the TUI, to be fixed there separately;
the GUI avoids each one.

**Can lose data**

1. A pending confirmation survives a page change: `1`–`6`, `R` and `o` run
   before the confirmation check (`tui/keyboard.go:118` before `:127`), so
   pressing `D` (clear queue) or `C` (clear history), then `1`, then Enter
   later on another page, clears it. The "1/2/3 cancel" branch
   (`keyboard.go:137-139`) can never run.
2. Album downloads land in the root download folder, not `Artist - Album/`:
   the worker writes to the folder it was created with and ignores the job's
   own (`downloader/downloader.go:92, 230-237, 297-437`). Changing Download
   Dir mid-session keeps writing to the old one. The CLI is not affected.
3. Favourites reverse their order on every restart (`db/db.go:274, 298-325`)
   and lose album and artist ids, so `a`/`A` on a favourite fails after a
   restart.

**Playback**

4. The "3 in a row" stop cannot trigger for songs with a URL:
   `failedInARow` is reset on every successful `loadfile` (`tui/model.go:629`),
   before mpv reports the failure.
5. Under repeat, a song that keeps failing to start loops forever
   (`async.go:625-630`).
6. Removing the playing song leaves mpv playing it while the marker and title
   move to the next one, which is then skipped.
7. Space before anything has played this session flips Playing/Paused with no
   audio, and stops Enter from starting playback.
8. A failed autoplay fetch leaves the latch set: "Autoplay loading
   suggestions…" forever; "Autoplay: no suggestions available" is unreachable
   (`tui/commands.go:192-194`).
9. An autoplay result that arrives while paused replaces the paused song.
10. A saved Default Volume of 0 comes back as 80 (`tui/model.go:571-573`).

**Interface**

11. An error stays on the status line until the next search, hiding every
    later message and confirmation (`tui/view.go:2237`, `ticks.go:35`).
12. Two searches in flight: the last reply wins, not the newest search; there
    is no sequence number (`tui/model.go:99-106`).
13. `f` on History or Downloads toggles a hidden search result
    (`keyboard.go:554-568`); a query and Enter on Favs, History or Downloads
    runs a search nobody sees.
14. `x` inside an album downloads only the highlighted song, though the panel
    title says `[x] download`.
15. Loading flags for album and artist are cleared before the stale-reply
    check, so a stale reply ends a newer load (`async.go:80, 686`).
16. "Download first, then play" streams at once, like the hybrid mode.
17. Last.fm gets the raw album field, play counts included.
18. A failed ffprobe read is cached and never retried; the library cache never
    forgets deleted files; changing Download Dir does not rescan.
19. A recommendations error leaves the panel saying "No results".
20. Lyrics stay on screen after stopping or clearing the queue.
21. `ctrl+↑/↓` works only on the Stream page, and saves the queue even when
    nothing moved.
22. `nextTrack`/`prevTrack` can leave the queue cursor off screen.

**Docs that disagree with the code**

23. "press F on any track" (the key is `f`); the idle tip says `1`-`5` for
    six tabs; `docs/guide/cli.md` shows search output with `·` where the code
    prints `—`, and says `-a` works on `play`, which rejects it; the guide says
    Spotify matches within "a couple of seconds", the code allows 5.
