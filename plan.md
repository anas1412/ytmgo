## 📋 Detailed Plan

### Architecture & Edge Cases

**1. Single Playback Lock — preventing multiple songs playing**
A single `*mpv.Instance` pointer guarded by a mutex. Before starting any new song, we `Kill()` the existing process, wait for it to exit, then start fresh. There's no "play alongside" — only one process lives at a time.

**2. Download Pipeline — no race conditions**
- A `Downloader` struct owns a single `yt-dlp` subprocess at a time
- Downloads go into `downloads/` as `.mp3` via `yt-dlp --extract-audio --audio-format mp3`
- Progress is parsed from yt-dlp's `--progress` stdout (`[download] 47.3%...`) via regex and sent over a channel
- **Only one download runs at a time** — subsequent downloads are queued in a `[]DownloadJob` slice
- If a song is already downloaded (file exists), it skips yt-dlp entirely

**3. Queue Management**
- `Queue` struct: `[]Track`, `currentIndex int`, mutex-protected
- Operations: `Add`, `Remove`, `MoveUp`, `MoveDown`, `Clear`, `Next`, `Prev`, `PlayAt(i)`
- When a song finishes (mpv exits), queue auto-advances → sends a `NextTrackMsg` into bubbletea
- Removing the currently-playing track stops playback and advances

**4. State Machine for Player**
```
Stopped → Playing → Paused → Playing
                 ↓
              Stopped (song ends / next)
```
States stored as `PlayerState` enum, all transitions go through one `func (p *Player) transition(cmd PlayerCmd)`

**5. mpv IPC**
- mpv launched with `--input-ipc-server=/tmp/ytmgo_mpv.sock`
- A goroutine polls the socket every 500ms: sends `{"command":["get_property","time-pos"]}` and `{"command":["get_property","duration"]}`
- Responses update `Model.Position` and `Model.Duration`
- On mpv process death, goroutine exits and sends `SongEndedMsg`

**6. TUI Layout — 5 panels**
```
┌─────────────────────────────────────────────────────┐
│  ♪ YTMUSIC          [Search: _______________] [/]   │  ← Header
├──────────────────────────┬──────────────────────────┤
│                          │                          │
│   SEARCH RESULTS         │   QUEUE                  │
│   (scrollable list)      │   (scrollable list)      │
│                          │   ► 1. Song name         │
│   1. Result title        │     2. Song name         │
│   2. Result title        │     3. Song name         │
│   ...                    │                          │
├──────────────────────────┴──────────────────────────┤
│  DOWNLOADS                                          │  ← Download bar
│  ⬇ Downloading: "Song Title"  [████░░░░░░] 47%      │
├─────────────────────────────────────────────────────┤
│  NOW PLAYING: Song Title — Artist                   │  ← Player
│  ══════════════════════░░░░░░░░░  2:14 / 4:32       │
│  [◄◄]  [⏸]  [►►]   VOL: ████░  SHUFFLE  REPEAT    │
└─────────────────────────────────────────────────────┘
```

**7. Keybindings**
- `Tab` — switch focus between panels
- `/` — jump to search input
- `Enter` — on result: add to queue + auto-download if needed; on queue: play immediately
- `Space` — play/pause
- `n`/`p` — next/prev
- `d` — remove from queue
- `↑↓` — navigate lists
- `←→` — seek ±5s
- `+`/`-` — volume
- `s` — toggle shuffle
- `r` — toggle repeat
- `q` — quit

**8. File Structure**
```
ytmgo/
├── main.go
├── go.mod
├── internal/
│   ├── player/     player.go, mpv.go
│   ├── downloader/ downloader.go
│   ├── queue/      queue.go
│   ├── search/     search.go (yt-dlp search)
│   └── tui/        model.go, view.go, update.go, styles.go, keys.go
└── downloads/
```
