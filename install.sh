#!/usr/bin/env bash
# install.sh — one-line installer for ytmgo
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash -s -- --force
#
# To uninstall: curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
#
# Environment overrides (set before the pipe):
#   YTMGO_VERSION=v0.2.0     # pin a specific version (default: latest)
#   YTMGO_INSTALL_DIR=...   # override install dir (default: ~/.local/bin or /usr/local/bin if root)
#   YTMGO_FORCE=true        # reinstall even if already up to date
#
# What this does:
#   1. Detects your OS and CPU architecture
#   2. Downloads the matching static binary from the GitHub Release
#   3. Installs it to a directory on PATH (or prints the export command)
#   4. Installs yt-dlp from upstream, and any missing mpv/ffmpeg/cava via
#      your package manager — uses sudo for system PMs, no sudo for brew.
#      You'll see the exact command before it runs.

set -euo pipefail

REPO="anas1412/ytmgo"
BINARY="ytmgo"
BINARIES_ARE_SCRATCH=false

# ─── Colors (only if stdout is a TTY) ────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; BLUE=$'\033[34m'; RESET=$'\033[0m'
else
  BOLD=""; DIM=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; RESET=""
fi

info()    { printf '%s==>%s %s\n' "$BLUE"   "$RESET" "$*"; }
success() { printf '%s ✓%s  %s\n' "$GREEN"  "$RESET" "$*"; }
warn()    { printf '%s !%s  %s\n' "$YELLOW" "$RESET" "$*"; }
err()     { printf '%s ✗%s  %s\n' "$RED"    "$RESET" "$*" >&2; }

# ─── Detect OS / arch ───────────────────────────────────────────────
uname_os=$(uname -s)
uname_arch=$(uname -m)

case "$uname_os" in
  Linux)  os="Linux" ;;
  Darwin) os="Darwin" ;;
  *) err "Unsupported OS: $uname_os (only Linux and macOS)"; exit 1 ;;
esac

case "$uname_arch" in
  x86_64|amd64)           arch="x86_64" ;;
  aarch64|arm64)          arch="arm64" ;;
  *) err "Unsupported architecture: $uname_arch (only x86_64 and arm64)"; exit 1 ;;
esac

# ─── Arch Linux: try AUR package first ──────────────────────────────
# If we're on Arch (or any Arch derivative), prefer installing via AUR
# helper (paru > yay) for better system integration (desktop file, man
# page, etc.). Fall back to the static binary if no AUR helper is found.
is_arch=false
if [ -f /etc/arch-release ]; then
  is_arch=true
elif [ -f /etc/os-release ]; then
  grep -qi '^ID=arch' /etc/os-release  && is_arch=true
  grep -qi 'cachyos'  /etc/os-release  && is_arch=true
fi
if [ "$os" = "Linux" ] && [ "$is_arch" = true ] && [ -z "${YTMGO_VERSION:-}" ] && [ -z "${YTMGO_INSTALL_DIR:-}" ]; then
  helper=""
  command -v paru >/dev/null 2>&1 && helper=paru
  [ -z "$helper" ] && command -v yay >/dev/null 2>&1 && helper=yay
  if [ -n "$helper" ]; then
    # ytmgo-bin installs the same static binary this script would fetch
    # by hand, so the AUR route costs no more than the manual one. The
    # plain ytmgo package compiles instead, which pulls the whole Go
    # toolchain — a fine thing to choose, a poor thing to be given.
    #
    # Whichever is already installed is the one that gets updated. The
    # two are the same program and conflict, and asking for ytmgo-bin
    # over an installed ytmgo cannot work unattended: pacman takes the
    # default answer under --noconfirm and the default is no, so every
    # update printed "unresolvable package conflicts" and then quietly
    # fell back. Switching means removing a package the user chose, so
    # it is offered rather than done.
    switch_hint=false
    if pacman -Qq ytmgo-bin >/dev/null 2>&1; then
      pkg=ytmgo-bin
    elif pacman -Qq ytmgo >/dev/null 2>&1; then
      pkg=ytmgo
      switch_hint=true
    else
      pkg=ytmgo-bin
    fi
    info "Detected Arch Linux + $helper — installing $pkg via AUR…"
    if ! $helper -S --noconfirm "$pkg"; then
      # -bin is the newer package and the only one that might be missing
      # from the AUR; ytmgo has been there all along, so a failure there
      # is a real failure.
      if [ "$pkg" != "ytmgo-bin" ]; then
        err "$helper could not install $pkg."
        exit 1
      fi
      warn "$pkg unavailable — falling back to ytmgo (builds from source)."
      pkg=ytmgo
      $helper -S --noconfirm "$pkg"
    fi
    success "Installed $pkg via $helper"
    if [ "$switch_hint" = true ]; then
      echo ""
      info "ytmgo-bin is the same build without compiling it. To switch:"
      echo "  $helper -R ytmgo && $helper -S ytmgo-bin"
    fi
    echo ""
    info "To uninstall later:"
    echo "  $helper -R $pkg"
    exit 0
  else
    warn "Arch Linux detected but no AUR helper found (paru/yay)."
    warn "Falling back to static binary. Install paru or yay for AUR support."
    echo ""
  fi
fi

goarch="$arch"
asset="${BINARY}_${os}_${goarch}.tar.gz"

# ─── Pick install dir ────────────────────────────────────────────────
# /usr/local/bin whenever we can write there, because it is on every
# shell's PATH already — the same reason the Arch package's /usr/bin
# just works. ~/.local/bin needs PATH wiring that only helps the *next*
# shell, which read as "installed, but command not found". The deps
# step already uses sudo on Linux, so using it for one binary is not a
# new ask.
SUDO=""
if [ -n "${YTMGO_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$YTMGO_INSTALL_DIR"
elif [ "$(id -u)" -eq 0 ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ "$os" = "Linux" ] && command -v sudo >/dev/null 2>&1; then
  INSTALL_DIR="/usr/local/bin"
  SUDO="sudo"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

# ─── Determine version ──────────────────────────────────────────────
VERSION="${YTMGO_VERSION:-}"
if [ -z "$VERSION" ]; then
  info "Looking up latest release…"
  latest_json=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null || true)
  if [ -z "$latest_json" ]; then
    err "Could not reach GitHub API. Set YTMGO_VERSION=vX.Y.Z and retry."
    exit 1
  fi
  VERSION=$(printf '%s' "$latest_json" \
    | grep -oE '"tag_name":[[:space:]]*"v[^"]+"' \
    | head -1 \
    | sed -E 's/.*"v([^"]+)".*/\1/')
  if [ -z "$VERSION" ]; then
    err "Could not parse latest version from GitHub API response."
    exit 1
  fi
fi
tag="v$VERSION"

# ─── Version check (skip if already up to date) ──────────────────────
FORCE="${YTMGO_FORCE:-}"
if [ $# -gt 0 ]; then
  for arg in "$@"; do
    [ "$arg" = "--force" ] && FORCE="true"
  done
fi
if [ -z "$FORCE" ] && command -v "$BINARY" >/dev/null 2>&1; then
  installed_ver=$("$BINARY" --version 2>/dev/null | awk '{print $2}')
  if [ "$installed_ver" = "$tag" ]; then
    success "${BINARY} ${tag} is already installed — nothing to do."
    echo ""
    info "Run with YTMGO_FORCE=true (or pass --force) to reinstall."
    exit 0
  fi
fi

# ─── Download + verify ──────────────────────────────────────────────
base_url="https://github.com/$REPO/releases/download/$tag"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading ${BINARY} ${tag} (${os}/${arch})…"
if ! curl -fSL --progress-bar -o "$tmp/$asset" "$base_url/$asset"; then
  err "Download failed. Check that $tag exists and has a $asset."
  err "  (open https://github.com/$REPO/releases/tags/$tag to verify)"
  exit 1
fi

# Verify SHA256 if a sidecar is published alongside the asset
if curl -fsSL -o "$tmp/$asset.sha256" "$base_url/$asset.sha256" 2>/dev/null; then
  info "Verifying checksum…"
  expected=$(awk '{print $1}' "$tmp/$asset.sha256")
  if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
  elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
  else
    warn "No sha256sum/shasum found — skipping checksum verification."
    actual="$expected"
  fi
  if [ "$expected" != "$actual" ]; then
    err "Checksum mismatch!"
    err "  expected: $expected"
    err "  actual:   $actual"
    exit 1
  fi
  success "Checksum OK"
else
  warn "No .sha256 sidecar found — skipping checksum verification."
fi

# ─── Install ────────────────────────────────────────────────────────
info "Extracting…"
tar -xzf "$tmp/$asset" -C "$tmp" "$BINARY"
chmod +x "$tmp/$BINARY"

$SUDO mkdir -p "$INSTALL_DIR"
$SUDO install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
success "Installed ${BINARY} ${tag} → ${INSTALL_DIR}/${BINARY}"

# ─── Desktop entry ──────────────────────────────────────────────────
# The AUR package ships one; everyone else got a bare binary and no
# launcher entry. Installed to the same prefix as the binary, so a
# --user install lands in ~/.local/share and a root install system-wide.
if [ "$os" = "Linux" ]; then
  if [ "$INSTALL_DIR" = "/usr/local/bin" ] || [ "$INSTALL_DIR" = "/usr/bin" ]; then
    DATA_DIR="/usr/local/share"
  else
    DATA_DIR="${XDG_DATA_HOME:-$HOME/.local/share}"
  fi
  app_dir="$DATA_DIR/applications"
  icon_dir="$DATA_DIR/icons/hicolor/256x256/apps"
  if $SUDO mkdir -p "$app_dir" "$icon_dir" 2>/dev/null; then
    icon_url="https://raw.githubusercontent.com/${REPO}/${tag}/ytmgo-icon.png"
    if curl -fsSL -o "$tmp/ytmgo-icon.png" "$icon_url" 2>/dev/null; then
      $SUDO install -m 0644 "$tmp/ytmgo-icon.png" "$icon_dir/ytmgo.png"
      icon_line="Icon=ytmgo"
    else
      # No icon is not a reason to skip the launcher entry.
      icon_line="Icon=multimedia-audio-player"
    fi
    cat > "$tmp/ytmgo.desktop" <<DESKTOP_EOF
[Desktop Entry]
Type=Application
Name=ytmgo
Comment=YouTube Music from the Terminal
Exec=$INSTALL_DIR/$BINARY
$icon_line
Terminal=true
Categories=AudioVideo;Audio;Music;Player;
Keywords=music;youtube;player;terminal;tui;audio;
StartupNotify=false
DESKTOP_EOF
    $SUDO install -m 0644 "$tmp/ytmgo.desktop" "$app_dir/ytmgo.desktop"
    # Mint/GNOME cache launcher entries; without this the item can take
    # a re-login to appear.
    command -v update-desktop-database >/dev/null 2>&1 &&
      $SUDO update-desktop-database "$app_dir" >/dev/null 2>&1 || true
    success "Desktop entry → ${app_dir}/ytmgo.desktop"
  fi
fi

# ─── PATH ───────────────────────────────────────────────────────────
# ~/.local/bin is the usual case on Debian/Ubuntu/Mint, where ~/.profile
# adds it to PATH *only if it already existed at login*. Installing
# creates it, so a first install leaves the command missing until the
# next login — which reads as "installed, but command not found". Wire
# it into the shell rc files so a new terminal just works, and tell the
# user how to fix the one they are standing in.
if [ "$INSTALL_DIR" = "/usr/local/bin" ] || [ "$INSTALL_DIR" = "/usr/bin" ]; then
  # System locations are on every default PATH; a shell missing them is
  # exotic enough that editing rc files would be a guess.
  :
else
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    line="export PATH=\"$INSTALL_DIR:\$PATH\""
    added=""
    for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
      [ -f "$rc" ] || continue
      grep -qF "$INSTALL_DIR" "$rc" 2>/dev/null && continue
      printf '\n# added by ytmgo installer\n%s\n' "$line" >> "$rc"
      added="$added $(basename "$rc")"
    done
    if [ -n "$added" ]; then
      success "Added $INSTALL_DIR to PATH in:$added"
      printf '   %sNew terminals will find it. For this one:%s\n' "$BOLD" "$RESET"
    else
      warn "$INSTALL_DIR is not on your PATH."
      printf '   Add this to your shell rc file:\n'
    fi
    printf '   %s%s%s\n' "$BOLD" "$line" "$RESET"
    ;;
esac
fi

# ─── yt-dlp, from upstream ──────────────────────────────────────────
# yt-dlp is the one dependency a distro package actively breaks. YouTube
# changes, yt-dlp patches within days, and a frozen archive does not
# follow: Debian and Ubuntu still carry builds from 2023 and 2025
# alongside the current one. An out-of-date yt-dlp cannot open a single
# track, which ytmgo used to show as the queue skipping past everything
# in silence.
#
# So it comes from yt-dlp's own releases instead, into the same dir as
# ytmgo. That copy self-updates with `yt-dlp -U`, which a distro-managed
# one refuses to do.
install_ytdlp() {
  local dest="$INSTALL_DIR/yt-dlp" asset
  case "$os/$(uname -m)" in
    Linux/x86_64)          asset="yt-dlp_linux" ;;
    Linux/aarch64|Linux/arm64) asset="yt-dlp_linux_aarch64" ;;
    Darwin/*)              asset="yt-dlp_macos" ;;
    *)                     asset="yt-dlp" ;;   # the python zipapp, needs python3
  esac
  local url="https://github.com/yt-dlp/yt-dlp/releases/latest/download/$asset"

  # An existing upstream copy just updates itself; that is the whole
  # point of not using the package manager.
  if [ -x "$dest" ]; then
    info "Updating yt-dlp…"
    "$dest" -U >/dev/null 2>&1 && { success "yt-dlp is current"; return 0; }
  fi

  info "Installing yt-dlp from upstream…"
  local tmp
  tmp=$(mktemp) || return 1
  # Shown, not silent: this is ~30MB, and -s left the terminal blank
  # long enough to look like a hang on a slow connection.
  if ! curl -fL --progress-bar "$url" -o "$tmp"; then
    rm -f "$tmp"
    warn "Could not download yt-dlp from $url"
    return 1
  fi
  chmod +x "$tmp"
  if ! mv "$tmp" "$dest" 2>/dev/null; then
    if command -v sudo >/dev/null 2>&1 && sudo mv "$tmp" "$dest"; then
      sudo chmod +x "$dest"
    else
      rm -f "$tmp"
      warn "Could not write $dest"
      return 1
    fi
  fi
  success "Installed yt-dlp ($("$dest" --version 2>/dev/null || echo 'version unknown'))"
}

# A packaged yt-dlp earlier on PATH used to matter: it would win, and
# it is the stale one. ytmgo now runs the copy beside its own binary by
# absolute path, and tells mpv's ytdl_hook to do the same, so PATH order
# no longer decides. Worth a note, not a warning.
if command -v yt-dlp >/dev/null 2>&1; then
  existing=$(command -v yt-dlp)
  if [ "$existing" != "$INSTALL_DIR/yt-dlp" ]; then
    info "A packaged yt-dlp exists at $existing; ytmgo will use its own copy instead."
  fi
fi
install_ytdlp || warn "Continuing without yt-dlp — downloads and playback will not work until it is installed."

# ─── System deps check + install ───────────────────────────────────
# Auto-install any missing mpv/ffmpeg/cava via the user's package
# manager. Uses sudo for system PMs (apt/dnf/pacman/apk) — not for brew.
#
# yt-dlp is deliberately NOT in that list; see install_ytdlp below.
missing=()                        # init for `set -u` (line 154 reads ${#missing[@]})
deps=("mpv" "cava")
for dep in "${deps[@]}"; do
  if ! command -v "$dep" >/dev/null 2>&1; then
    missing+=("$dep")
  fi
done
# ffprobe ships with ffmpeg; check it last since fewer distros install it standalone
if ! command -v ffprobe >/dev/null 2>&1 && ! command -v ffmpeg >/dev/null 2>&1; then
  missing+=("ffmpeg")
fi

if [ ${#missing[@]} -gt 0 ]; then
  echo ""
  warn "Missing system dependencies: ${missing[*]}"
  warn "Installing them now via your package manager…"
  echo ""

  # Pick the right package manager. Note: order matters — brew on Linux
  # also installs to /usr/local so we check it first on macOS only.
  pm_cmd=""
  # Root has no use for sudo and often has no sudo: a container or a
  # root shell would fail on "sudo: command not found" rather than on
  # anything to do with packages.
  pm_sudo="sudo"
  [ "$(id -u)" -eq 0 ] && pm_sudo=""
  case "$os" in
    Linux)
      # Atomic Fedora first — Silverblue, Kinoite, Bazzite, Bluefin and
      # friends all carry dnf, but /usr is read-only, so dnf fails with
      # a message about an image-based system and the installer used to
      # blame the sudo password. Layering is the native fix, and it
      # needs a reboot, so this tells rather than does.
      if command -v rpm-ostree >/dev/null 2>&1 && [ ! -w /usr ]; then
        err "This system keeps /usr read-only, so packages cannot be installed into it."
        err "ytmgo itself is installed. For the rest, layer them onto the image:"
        err "    sudo rpm-ostree install ${missing[*]}"
        err "  (then reboot)"
        if command -v brew >/dev/null 2>&1; then
          err "Or into your home directory, with no reboot and no layering:"
          err "    brew install ${missing[*]}"
        else
          err "Or run them from a container: distrobox, or a Homebrew install."
        fi
        exit 1
      fi
      if   command -v apt    >/dev/null 2>&1; then pm_cmd="$pm_sudo apt install -y"
      elif command -v dnf    >/dev/null 2>&1; then pm_cmd="$pm_sudo dnf install -y"
      elif command -v pacman >/dev/null 2>&1; then pm_cmd="$pm_sudo pacman -S --noconfirm"
      elif command -v apk    >/dev/null 2>&1; then pm_cmd="$pm_sudo apk add"
      fi ;;
    Darwin)
      if command -v brew >/dev/null 2>&1; then
        pm_cmd="brew install"
      fi ;;
  esac

  if [ -z "$pm_cmd" ]; then
    err "No supported package manager found."
    err "Please install these manually: ${missing[*]}"
    exit 1
  fi

  info "Running: $pm_cmd ${missing[*]}"
  if ! $pm_cmd "${missing[@]}"; then
    # cava is the spectrum and nothing else — ytmgo says so and carries
    # on without it. It is also the one of the three that older Debian
    # and Ubuntu do not package, and a single unavailable name makes the
    # whole command fail, which used to leave mpv and ffmpeg uninstalled
    # too. Required first, then the nice-to-have on its own.
    required=()
    for dep in "${missing[@]}"; do
      [ "$dep" = "cava" ] || required+=("$dep")
    done
    if [ ${#required[@]} -gt 0 ]; then
      warn "That failed — retrying without the optional ones…"
      if ! $pm_cmd "${required[@]}"; then
        err "Package install failed."
        err "Try running this yourself: $pm_cmd ${required[*]}"
        exit 1
      fi
    fi
    warn "cava is not packaged here, so the visualizer (v) stays off."
    warn "Everything else works. Build it from https://github.com/karlstav/cava if you want it."
  fi
  success "Installed system dependencies"
fi

# ─── Done ───────────────────────────────────────────────────────────
echo ""
success "Run: ytmgo"
echo ""
info "To uninstall later:"
echo "  curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash"
