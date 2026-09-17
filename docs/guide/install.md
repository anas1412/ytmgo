# Install

## One line

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
```

The script works out what your system needs:

- **Arch Linux** installs `ytmgo-bin` from the AUR through `paru` or
  `yay` — the same prebuilt binary, so nothing is compiled. If either
  package is already installed it updates that one instead, so an
  update never swaps out the package you chose. With no AUR helper at
  all it just fetches the binary.
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

The one-liner handles this, but both packages are on the AUR directly:

```bash
paru -S ytmgo      # builds from source
paru -S ytmgo-bin  # the released binary
```

`ytmgo` compiles from source, which pulls in the Go toolchain — 226 MB
installed, if you do not already have it. `ytmgo-bin` installs the same
static binary the GitHub release publishes, so there is nothing to
build. They provide the same program and conflict with each other, so
install whichever you prefer.

Already have `ytmgo` and want the prebuilt one? Updates leave your
choice alone, so the swap is yours to make:

```bash
paru -R ytmgo && paru -S ytmgo-bin
```

## Rolling build

Every push to `main` publishes a prerelease on the `latest` tag. It is
the newest code rather than a release, so expect it to break
occasionally:

```bash
YTMGO_VERSION=latest curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
```

It names itself after the last release and the commit it was built
from — `v1.2.2-8-g3c10e7b` — so a bug report says exactly what ran.

::: tip This cannot reach you by accident
The installer resolves versions through the newest *release*, and a
prerelease is not one. Only asking for `YTMGO_VERSION=latest` gets you
this build; running the installer again without it puts you back on a
released version.
:::

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

The installer does this for you. To do it by hand, or to give a
system-wide yt-dlp the same treatment:

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

::: tip Backing it up
Copy `ytmgo.db` while ytmgo is closed. A clean exit leaves no `-wal`
file beside it; if you see one, the app is still running or did not exit
cleanly, and copying the database alone would miss recent writes.
:::

## When something does not work

Start with [Troubleshooting](/guide/troubleshooting) — nothing plays, no
sound, no visualizer, no album art, media keys, lyrics.

## Uninstalling

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
```

It asks before each step, so you can remove the binary and the desktop
entry while keeping your settings, favourites and play history. Pass
`--keep-user-data` to skip that question entirely.
