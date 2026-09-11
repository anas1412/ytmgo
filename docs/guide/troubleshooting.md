# Troubleshooting

Most problems here are a missing external program rather than ytmgo
itself: it drives `mpv`, `yt-dlp`, `ffmpeg` and `cava` rather than
reimplementing them. Each section says which one.

## Nothing plays

Almost always an out-of-date yt-dlp. YouTube changes, yt-dlp patches
within days, and a distro package does not follow — so every track fails
to open at once.

ytmgo stops after three tracks in a row fail and says so:

> Nothing will play — mpv could not open 3 tracks in a row. Update
> yt-dlp: `yt-dlp -U`

Run that. If the copy on your machine came from a package manager and
refuses to self-update, install the upstream one:

```bash
curl -fsSL https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp_linux \
  -o ~/.local/bin/yt-dlp && chmod +x ~/.local/bin/yt-dlp
```

Use `yt-dlp_macos` on macOS, or `yt-dlp_linux_aarch64` on arm64.

You do **not** need to remove the packaged copy or change your `PATH`.
ytmgo runs the yt-dlp sitting beside its own binary by absolute path, so
whichever one comes first in `PATH` makes no difference.

To see what it is actually running:

```bash
ls -l "$(dirname "$(command -v ytmgo)")/yt-dlp" && yt-dlp --version
```

If a track still will not play, take ytmgo out of it and try mpv
directly. If this fails too, the problem is below ytmgo:

```bash
mpv --no-video "https://music.youtube.com/watch?v=dQw4w9WgXcQ"
```

## The queue skips through every track

The same cause as above, seen from the other side: mpv reports a failure
for each track, ytmgo advances, and the queue empties in seconds.

Older versions treated that failure as a track ending normally and raced
through the whole queue silently. Since v1.0.1 it stops after three and
tells you why. Update yt-dlp.

## No sound, but the progress bar moves

ytmgo is playing; your audio setup is not. It has no volume control of
its own beyond mpv's, so test mpv directly with the command above.

If mpv is silent too, it is picking the wrong output device. PipeWire,
PulseAudio and ALSA all work — ytmgo does not care which — so this is an
mpv or system-audio question:

```bash
mpv --audio-device=help
```

## No visualizer

The spectrum needs [`cava`](https://github.com/karlstav/cava). Without
it, `v` says so and names the command for your package manager:

```bash
sudo apt install cava      # Debian, Ubuntu, Mint
sudo dnf install cava      # Fedora
sudo pacman -S cava        # Arch
brew install cava          # macOS
```

If cava is installed and the pane is still empty, cava itself is not
hearing anything — it needs a monitor source to read. Run `cava` on its
own in a terminal; if it is flat there, the fix is in cava's config, not
in ytmgo.

## Copying a link says there is no clipboard tool

`u` shells out to whatever your session provides — `wl-copy` on Wayland,
`xclip` or `xsel` on X11, `pbcopy` on macOS. With none of them present
it tells you:

```bash
sudo apt install wl-clipboard    # Wayland
sudo apt install xclip           # X11
```

macOS always has `pbcopy`.

## No album art

Full-colour artwork uses the kitty graphics protocol. kitty defined it,
but three terminals implement it and ytmgo draws real images in all
three:

| Terminal | Artwork |
|---|---|
| kitty | Full image |
| Ghostty | Full image |
| WezTerm | Full image |
| Alacritty, xterm, GNOME Terminal, Konsole… | Coloured half-blocks |

The fallback is a coarser picture, not a missing one. Alacritty in
particular implements no inline-image protocol at all, by design, so
half-blocks is the best any program can do there.

Inside `tmux` or `screen` the fallback is used even in kitty: the
multiplexer swallows the graphics escapes, so drawing the real image
would produce nothing at all.

## "/usr is configured to be read-only"

An atomic Fedora — Silverblue, Kinoite, Bazzite, Bluefin. `dnf` is
present but cannot write to `/usr`, so installing mpv, ffmpeg and cava
the usual way fails.

ytmgo itself installs fine: it goes to `/usr/local/bin`, which is
writable on these systems. Only the dependencies need another route.

Layer them onto the image, the native way — this needs a reboot:

```bash
sudo rpm-ostree install mpv ffmpeg cava
```

Or put them in your home directory instead, with no reboot and nothing
layered:

```bash
brew install mpv ffmpeg cava
```

A `distrobox` container works too, if you already run one.

## Media keys do nothing

Media keys and the desktop's now-playing widget work over
[MPRIS](https://specifications.freedesktop.org/mpris-spec/latest/), which
needs a D-Bus session bus. Without one — a bare TTY, most SSH sessions, a
container — ytmgo starts normally and simply does not register.

This is also why it is Linux-only: macOS has no MPRIS.

## No lyrics for a track

Lyrics come from [LRCLIB](https://lrclib.net) first, which is the only
one of the two sources with **timestamps** — that is what makes the pane
follow the song. If LRCLIB has nothing, is slow, or is rate-limiting,
ytmgo falls back to YouTube Music's own plain text, so you lose the
timing rather than the lyrics.

`No lyrics found` means both were asked and neither had any. It is
common for instrumentals, remixes, and tracks whose title carries extra
decoration that stops it matching.

## ytmgo is not in my applications menu

The installer writes a desktop entry and an icon. Where depends on where
the binary went: `/usr/local/share/applications` for a system install,
`~/.local/share/applications` for a user one.

It refreshes the menu cache itself, but Mint and GNOME sometimes need a
re-login. To force it:

```bash
update-desktop-database ~/.local/share/applications
# or, for a system install
sudo update-desktop-database /usr/local/share/applications
```

If you built from source rather than running the installer, no entry was
created.

## Where the log is

In the data directory, beside `ytmgo.db` and `downloads/` —
[Where your data lives](/guide/install#where-your-data-lives) says where
that is on each platform.

## Windows

Not supported, and not planned —
[Supported systems](/guide/install#supported-systems) explains why. WSL
works, since that is Linux.

## Something else

Check the log in the data directory above, then
[open an issue](https://github.com/anas1412/ytmgo/issues) with what you
were doing, your distribution, and the output of:

```bash
ytmgo --version && mpv --version | head -1 && yt-dlp --version
```
