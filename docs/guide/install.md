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

::: tip yt-dlp comes from upstream, not your package manager
`yt-dlp` is the one dependency a distro package actively breaks. YouTube
changes, yt-dlp patches within days, and a frozen archive does not
follow — Debian and Ubuntu still carry builds from 2023 and 2025
alongside the current one. An out-of-date yt-dlp cannot open a single
track, which looks like ytmgo skipping through the whole queue in
silence.

So the installer fetches yt-dlp from its own releases, next to the
ytmgo binary. That copy self-updates with `yt-dlp -U`; a packaged one
refuses to.

You do not have to do anything about a yt-dlp you already have. ytmgo
runs its own copy by absolute path and tells mpv's stream hook to use
the same one, so whatever your PATH order is, the current yt-dlp wins.
:::

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
| `yt-dlp` | Downloading, and resolving streams for playback | Yes, installed from upstream |
| `ffmpeg` | Converting audio and embedding album art | Yes |
| `cava` | The audio visualizer | Yes |

Debian, Ubuntu and Mint:

```bash
sudo apt install mpv ffmpeg cava
```

Fedora:

```bash
sudo dnf install mpv ffmpeg cava
```

macOS:

```bash
brew install mpv ffmpeg cava
```

yt-dlp is missing from those lists on purpose. Install it from upstream,
where it can keep itself current:

```bash
curl -fsSL https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux \
  -o ~/.local/bin/yt-dlp && chmod +x ~/.local/bin/yt-dlp
```

Use `yt-dlp_macos` on macOS, or `yt-dlp_linux_aarch64` on arm64. From
then on `yt-dlp -U` updates it in place.

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

## If nothing plays

Almost always an out-of-date yt-dlp. YouTube changes, yt-dlp patches
within days, and a distro package does not follow — so every track fails
to open at once.

ytmgo stops after three tracks in a row fail and says so:

> Nothing will play — mpv could not open 3 tracks in a row. Update
> yt-dlp: `yt-dlp -U`

Run that, and if the copy on your machine came from a package manager
and refuses to self-update, install the upstream one from the section
above. You do not need to remove the packaged copy or change your
`PATH`; ytmgo prefers its own.

To check what ytmgo is actually running:

```bash
ls -l "$(dirname "$(command -v ytmgo)")/yt-dlp" && yt-dlp --version
```

If a track still will not play, take ytmgo out of it and try mpv
directly — if this fails too, the problem is below ytmgo:

```bash
mpv --no-video "https://music.youtube.com/watch?v=dQw4w9WgXcQ"
```

## Uninstalling

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
```

It asks before each step, so you can remove the binary and the desktop
entry while keeping your settings, favourites and play history. Pass
`--keep-user-data` to skip that question entirely.
