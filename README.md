# ytmgo

A terminal-based YouTube Music client written in Go. Search YouTube, download audio, manage a play queue, and play music — all from the keyboard, inside your terminal.

![Go Version](https://img.shields.io/badge/go-1.22+-blue)
![License](https://img.shields.io/badge/license-MIT-green)

---

## Install

One line does everything — grabs the static binary for your OS/arch, installs it, and auto-installs `mpv`/`yt-dlp`/`ffmpeg` if they're missing (using `sudo` for system package managers):

```bash
curl -fsSL https://anas1412.github.io/ytmgo/install.sh | bash
```

> Override the version: `YTMGO_VERSION=v0.2.0 curl ... | bash`
> Override the install dir: `YTMGO_INSTALL_DIR=/opt/bin curl ... | bash`

Or build from source (after installing `mpv`/`yt-dlp`/`ffmpeg` yourself):

```bash
go build -o ytmgo .
./ytmgo
```

---

## Features

- **YouTube Search** — Search YouTube directly from the terminal via `yt-dlp`
- **Audio Download** — Download tracks as MP3s with real-time progress
- **Play Queue** — Full queue management: reorder, shuffle, repeat (one / all)
- **Audio Playback** — Plays through `mpv` with seek, volume, and progress tracking
- **Keyboard-driven TUI** — Tab-focused layout with vim navigation, no mouse needed
- **Concurrency-safe** — Mutex-guarded queue, single-playback lock, serial download pipeline

---

## Demo

![ytmgo TUI screenshot](ytmgo.png)

---

## Prerequisites

- **Go** 1.22+
- **mpv** — audio playback backend
- **yt-dlp** — YouTube search and audio downloading
- **Brave** / **Firefox** / **Chrome** *(optional)* — for cookie extraction to access age-restricted content; configurable in Settings

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
| 4 | Press `Enter` on a result to add to queue + start download |
| 5 | `Tab` to the queue panel, select a track, press `Enter` to play |
| 6 | Control playback with keys (see below) |

Tab cycles focus through: search input → result list → queue panel → settings — and the focused panel's border glows violet.

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
| `x` | Download selected track immediately |
| `R` | Refresh recommendations |
| `1` / `2` / `3` | Switch page: Stream / Library / Settings |
| `Ctrl+↑` / `Ctrl+↓` | Move item up/down in queue |
| `?` | Toggle help overlay |
| `q` / `Ctrl+C` | Quit |

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
