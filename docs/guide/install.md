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

## Uninstalling

```bash
curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
```

It asks before each step, so you can remove the binary and the desktop
entry while keeping your settings, favourites and play history. Pass
`--keep-user-data` to skip that question entirely.
