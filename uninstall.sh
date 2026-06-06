#!/usr/bin/env bash
# uninstall.sh — remove ytmgo and its data
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
#
# To keep your downloaded files:
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash -s -- --keep-downloads
#
# What this removes:
#   1. The ytmgo binary (~/.local/bin/ytmgo or /usr/local/bin/ytmgo)
#   2. The config database (~/.config/ytmgo/ytmgo.db)
#   3. Downloaded tracks (~/.local/share/ytmgo/downloads/)
#
# System dependencies (mpv, yt-dlp, ffmpeg) are NOT removed — they
# may be used by other applications.

set -euo pipefail

# ─── Colors ───────────────────────────────────────────────────────────
if [ -t 1 ]; then
  BOLD=$'\033[1m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; BLUE=$'\033[34m'; RESET=$'\033[0m'
else
  BOLD=""; RED=""; GREEN=""; YELLOW=""; BLUE=""; RESET=""
fi

info()    { printf '%s==>%s %s\n' "$BLUE"   "$RESET" "$*"; }
success() { printf '%s ✓%s  %s\n' "$GREEN"  "$RESET" "$*"; }
warn()    { printf '%s !%s  %s\n' "$YELLOW" "$RESET" "$*"; }
err()     { printf '%s ✗%s  %s\n' "$RED"    "$RESET" "$*" >&2; }

KEEP_DOWNLOADS=false
for arg in "$@"; do
  [ "$arg" = "--keep-downloads" ] && KEEP_DOWNLOADS=true
done

# ─── 1. Remove the binary ────────────────────────────────────────────
BINARY="ytmgo"
BIN_PATH=""

if command -v "$BINARY" >/dev/null 2>&1; then
  BIN_PATH=$(command -v "$BINARY")
  info "Removing binary: $BIN_PATH"
  rm -f "$BIN_PATH"
  success "Removed binary"
else
  warn "Binary not found on PATH — checking default locations…"
  for p in "$HOME/.local/bin/$BINARY" "/usr/local/bin/$BINARY"; do
    if [ -f "$p" ]; then
      info "Removing binary: $p"
      rm -f "$p"
      success "Removed binary"
      BIN_PATH="$p"
    fi
  done
fi

# ─── 2. Remove config directory (DB with settings/favorites/history) ──
CONFIG_DIR="$HOME/.config/ytmgo"
if [ -d "$CONFIG_DIR" ]; then
  info "Removing config & data: $CONFIG_DIR"
  rm -rf "$CONFIG_DIR"
  success "Removed config (settings, favorites, play history)"
else
  warn "No config directory found at $CONFIG_DIR"
fi

# ─── 3. Remove downloads ─────────────────────────────────────────────
if [ "$KEEP_DOWNLOADS" = false ]; then
  # Default data location
  DOWNLOADS_DIR="$HOME/.local/share/ytmgo"
  # Also check XDG_DATA_HOME override
  if [ -n "${XDG_DATA_HOME:-}" ]; then
    XDG_DIR="$XDG_DATA_HOME/ytmgo"
  else
    XDG_DIR=""
  fi

  for d in "$DOWNLOADS_DIR" "$XDG_DIR"; do
    if [ -n "$d" ] && [ -d "$d" ]; then
      info "Removing downloads: $d"
      rm -rf "$d"
      success "Removed downloaded tracks"
    fi
  done

  # macOS default
  MACOS_DIR="$HOME/Library/Application Support/ytmgo"
  if [ -d "$MACOS_DIR" ]; then
    info "Removing downloads: $MACOS_DIR"
    rm -rf "$MACOS_DIR"
    success "Removed downloaded tracks"
  fi
else
  warn "Keeping downloads (--keep-downloads was passed)"
fi

# ─── Done ─────────────────────────────────────────────────────────────
echo ""
success "ytmgo has been uninstalled."

if [ -n "$BIN_PATH" ]; then
  # Suggest removing from PATH if it was in a non-standard location
  case ":$PATH:" in
    *":${BIN_PATH%/*}:"*) ;;
    *)
      echo ""
      warn "The install directory '${BIN_PATH%/*}' is still on your PATH."
      warn "Remove it from ~/.bashrc / ~/.zshrc if it was only added for ytmgo."
      ;;
  esac
fi

echo ""
info "System dependencies (mpv, yt-dlp, ffmpeg) were left untouched."
info "Remove them manually if not needed:"
echo "  apt:   sudo apt remove mpv yt-dlp ffmpeg"
echo "  dnf:   sudo dnf remove mpv yt-dlp ffmpeg"
echo "  brew:  brew uninstall mpv yt-dlp ffmpeg"
