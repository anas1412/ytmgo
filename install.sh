#!/usr/bin/env bash
# install.sh — one-line installer for ytmgo
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/install.sh | bash -s -- --force
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
#   4. Auto-installs any missing system deps (mpv, ffmpeg) via
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

goarch="$arch"
asset="${BINARY}_${os}_${goarch}.tar.gz"

# ─── Pick install dir ────────────────────────────────────────────────
if [ -n "${YTMGO_INSTALL_DIR:-}" ]; then
  INSTALL_DIR="$YTMGO_INSTALL_DIR"
elif [ "$(id -u)" -eq 0 ]; then
  INSTALL_DIR="/usr/local/bin"
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

mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
success "Installed ${BINARY} ${tag} → ${INSTALL_DIR}/${BINARY}"

# ─── PATH nudge ─────────────────────────────────────────────────────
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    warn "$INSTALL_DIR is not on your PATH."
    printf '   Add this to your ~/.bashrc (or ~/.zshrc):\n'
    printf '   %sexport PATH="%s:$PATH"%s\n' "$BOLD" "$INSTALL_DIR" "$RESET"
    ;;
esac

# ─── System deps check + install ───────────────────────────────────
# Auto-install any missing mpv/yt-dlp/ffmpeg via the user's package
# manager. Uses sudo for system PMs (apt/dnf/pacman/apk) — not for brew.
missing=()                        # init for `set -u` (line 154 reads ${#missing[@]})
deps=("mpv" "yt-dlp")
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
  case "$os" in
    Linux)
      if   command -v apt    >/dev/null 2>&1; then pm_cmd="sudo apt install -y mpv yt-dlp ffmpeg"
      elif command -v dnf    >/dev/null 2>&1; then pm_cmd="sudo dnf install -y mpv yt-dlp ffmpeg"
      elif command -v pacman >/dev/null 2>&1; then pm_cmd="sudo pacman -S --noconfirm mpv yt-dlp ffmpeg"
      elif command -v apk    >/dev/null 2>&1; then pm_cmd="sudo apk add mpv yt-dlp ffmpeg"
      fi ;;
    Darwin)
      if command -v brew >/dev/null 2>&1; then
        pm_cmd="brew install mpv yt-dlp ffmpeg"
      fi ;;
  esac

  if [ -z "$pm_cmd" ]; then
    err "No supported package manager found."
    err "Please install these manually: ${missing[*]}"
    exit 1
  fi

  info "Running: $pm_cmd"
  if ! $pm_cmd; then
    err "Package install failed (maybe sudo password was wrong?)."
    err "Try running this yourself: $pm_cmd"
    exit 1
  fi
  success "Installed system dependencies"
fi

# ─── Done ───────────────────────────────────────────────────────────
echo ""
success "Run: ytmgo"
