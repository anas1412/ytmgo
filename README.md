# ytmgo

A terminal-based YouTube Music client written in Go. Search YouTube, download audio, manage a play queue, and play music — all from the keyboard, inside your terminal.

![Screenshot placeholder](https://img.shields.io/badge/status-active-brightgreen)
![Go Version](https://img.shields.io/badge/go-1.22+-blue)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Features

- **YouTube Search** — Search YouTube directly from the terminal via `yt-dlp`
- **Audio Download** — Download tracks as MP3s with real-time progress
- **Play Queue** — Full queue management: reorder, shuffle, repeat (one / all)
- **Audio Playback** — Plays through `mpv` with seek, volume, and progress tracking
- **Slick TUI** — 5-panel layout with keyboard-driven navigation (Bubble Tea)
- **Concurrency-safe** — Mutex-guarded queue, single-playback lock, serial download pipeline

---

## Demo

```
┌─────────────────────────────────────────────────────────────┐
│  ♪ YTMUSIC          [Search: _______________]         [/]  │
├──────────────────────────┬──────────────────────────────────┤
│  SEARCH RESULTS          │  QUEUE                           │
│                          │  ► 1. Song name                  │
│  1. Artist - Title       │    2. Song name                  │
│  2. Artist - Title       │    3. Song name                  │
│  ...                     │                                  │
├──────────────────────────┴──────────────────────────────────┤
│  ⬇ Downloading: "Song"  [████░░░░░░]  47%                   │
├─────────────────────────────────────────────────────────────┤
│  Now Playing: Song — Artist                                 │
│  ══════════════════════░░░░░  2:14 / 4:32                   │
│  [prev]  [play/pause]  [next]   VOL: ████░  SHUFFLE REPEAT │
└─────────────────────────────────────────────────────────────┘
```

---

## Prerequisites

- **Go** 1.22+
- **mpv** — audio playback backend
- **yt-dlp** — YouTube search and audio downloading
- **Brave Browser** *(optional)* — for cookie extraction to access age-restricted content

### Install system dependencies

```bash
# Debian / Ubuntu
sudo apt install mpv yt-dlp

# macOS
brew install mpv yt-dlp

# Arch Linux
sudo pacman -S mpv yt-dlp
```

---

## Build & Run

```bash
# Clone or navigate to the project
cd ytmgo

# Build
go build -o ytmgo .

# Run
./ytmgo
```

Or use the pre-built binary included in the repository.

---

## Usage

| Step | Action |
|------|--------|
| 1 | Press `Tab` to focus the search input |
| 2 | Type a query and press `Enter` |
| 3 | Browse results in the left panel (`↑↓` / `jk`) |
| 4 | Press `Enter` on a result to add to queue + download |
| 5 | `Tab` to the queue panel, select a track, press `Enter` to play |
| 6 | Control playback with keys (see below) |

### Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Cycle focus: search → results → queue → search |
| `↑↓` / `jk` | Navigate lists |
| `Enter` | Search: add to queue / Queue: play track |
| `Space` | Play / Pause |
| `n` / `→` | Next track |
| `p` / `←` | Previous track |
| `h` / `Ctrl+B` | Seek backward 5s |
| `l` / `Ctrl+F` | Seek forward 5s |
| `+` / `=` | Volume up |
| `-` / `_` | Volume down |
| `d` / `Delete` | Remove from queue |
| `D` | Clear entire queue |
| `s` | Toggle shuffle |
| `r` | Cycle repeat: OFF → ONE → ALL |
| `Ctrl+↑` / `Ctrl+↓` | Move item up/down in queue |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

---

## Project Structure

```
ytmgo/
├── main.go                      # Entry point, Bubble Tea program setup
├── internal/
│   ├── tui/                     # Terminal UI (Bubble Tea)
│   │   ├── model.go             # Application model and commands
│   │   ├── update.go            # Message handling and state updates
│   │   ├── view.go              # Rendering / layout
│   │   ├── styles.go            # Color palette and styles
│   │   └── keys.go              # Key bindings
│   ├── player/                  # mpv audio playback control
│   │   └── player.go            # Subprocess lifecycle, IPC polling
│   ├── queue/                   # Thread-safe play queue
│   │   └── queue.go             # Queue with shuffle, repeat, reorder
│   ├── search/                  # YouTube search via yt-dlp
│   │   └── search.go            # Search + result parsing
│   └── downloader/              # Audio download via yt-dlp
│       └── downloader.go        # Serial download with progress
├── downloads/                   # Downloaded MP3 files
├── go.mod / go.sum              # Go module dependencies
└── plan.md                      # Architecture design notes
```

### Internal dependencies

```
main
  └── internal/tui
        ├── internal/player      (mpv playback)
        ├── internal/queue       (track queue)
        ├── internal/search      (yt-dlp search)
        └── internal/downloader  (yt-dlp download)
```

---

## Architecture Highlights

- **Single Playback Lock** — Only one `mpv` process runs at a time; old process is killed before starting new playback
- **Serial Download Pipeline** — One `yt-dlp` download at a time with a job queue behind it
- **Concurrency-safe Queue** — Mutex-guarded queue with shuffle, repeat-one, and repeat-all modes
- **mpv IPC Polling** — Real-time progress updates via Unix socket every 500ms
- **State Machine** — Player cycles through `Stopped → Playing → Paused → Playing → Stopped`
- **5-Panel Layout** — Header, search results, queue, download bar, player/controls bar

---

## Built With

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) — TUI components
- [Lipgloss](https://github.com/charmbracelet/lipgloss) — Terminal styling
- [mpv](https://mpv.io/) — Media player backend
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) — YouTube downloader

---

## License

MIT
