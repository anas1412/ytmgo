# Install

## One line

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
```

The script works out what your system needs:

- **Arch Linux** installs from the AUR through `paru` or `yay`, falling
  back to the release binary if neither is present.
- **Everything else (Linux, macOS)** downloads the release binary for
  your architecture, verifies its checksum, and installs it.

On Linux it also installs a desktop entry and icon, so ytmgo appears in
your applications menu, and offers to install the dependencies below
with your package manager.

::: tip Where it installs
`/usr/local/bin` when `sudo` is available, the same place a package
would put it, so the command works in the terminal you ran the installer
from. Without `sudo` it falls back to `~/.local/bin` and adds that to
your shell's `PATH`.

Override it with `YTMGO_INSTALL_DIR=/opt/bin curl ... | bash`.
:::

## Arch Linux

The one-liner handles this, but the package is on the AUR directly:

```bash
paru -S ytmgo
# or
yay -S ytmgo
```

## From source

Go 1.22 or newer:

```bash
git clone https://github.com/anas1412/ytmgo
cd ytmgo
go build -o ytmgo .
./ytmgo
```

## Supported systems

| | |
|---|---|
| **Linux**, any distribution | x86_64 and arm64 |
| **macOS** | Intel and Apple Silicon |
| Windows | Not supported |

*Any distribution* is meant literally: the binary is built with cgo off,
so it is statically linked and depends on no system C library. It runs
the same on glibc and on musl — Alpine included — and needs nothing
backported.

Two things are Linux-only inside the app. **Media keys and the desktop
media widget** work over MPRIS, which is D-Bus, so on macOS they simply
do nothing — everything else behaves identically. And the **spectrum**
needs cava to find a monitor of your audio output, which PipeWire and
PulseAudio both provide automatically; on a bare ALSA setup cava needs a
loopback configured by hand. Nothing else cares which audio stack you
run: playback goes through mpv, which picks its own output — PipeWire,
PulseAudio, ALSA, JACK, sndio or CoreAudio.

Windows is not supported and is not planned. ytmgo runs mpv, yt-dlp and
ffmpeg as child processes and has to kill a whole tree when a download
is cancelled — yt-dlp spawns ffmpeg, and killing only the parent orphans
it. That relies on POSIX process groups, which Windows has no equivalent
of.

## Requirements

ytmgo drives a few external programs rather than reimplementing them.
The installer offers to fetch these for you.

| Program | Used for | Required |
|---------|----------|:--------:|
| `mpv` | Playback | Yes |
| `yt-dlp` | Downloading | Yes |
| `ffmpeg` | Converting audio and embedding album art | Yes |
| `cava` | The audio visualizer | Yes |

Debian, Ubuntu and Mint:

```bash
sudo apt install mpv yt-dlp ffmpeg cava
```

Fedora:

```bash
sudo dnf install mpv yt-dlp ffmpeg cava
```

macOS:

```bash
brew install mpv yt-dlp ffmpeg cava
```

## Updating

Press `U` inside the app to check for a new version and install it. On
Arch that runs your AUR helper; elsewhere it re-runs the installer.

Or just run the one-liner again; it replaces the existing install.

## Where your data lives

Everything ytmgo keeps between runs sits in one directory:

| Platform | Directory |
|---|---|
| Linux | `$XDG_DATA_HOME/ytmgo`, or `~/.local/share/ytmgo` |
| macOS | `~/Library/Application Support/ytmgo` |

It holds `ytmgo.db` — settings, favourites, play history and the saved
queue — plus `downloads/` and a log.

Before v1 the database was in `~/.config/ytmgo`. It moves itself the
first time a v1 build starts; nothing to do, and nothing is lost.

## Uninstalling

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
```

It asks before each step, so you can remove the binary and the desktop
entry while keeping your settings, favourites and play history. Pass
`--keep-user-data` to skip that question entirely.
