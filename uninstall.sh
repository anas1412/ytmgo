#!/usr/bin/env bash
# uninstall.sh — remove ytmgo and its data
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/anas1412/ytmgo/main/uninstall.sh | bash
#
# Flags:
#   -y, --yes           skip all confirmations, remove everything
#   --keep-downloads    keep downloaded audio files
#   --keep-user-data    keep config database (settings, favorites, history)
#
# Examples:
#   curl -fsSL ... | bash -s -- -y                          # silent full removal
#   curl -fsSL ... | bash -s -- --keep-downloads            # keep files, prompt for rest
#   curl -fsSL ... | bash -s -- -y --keep-user-data         # keep config, remove rest silently
#
# What this removes:
#   1. The ytmgo binary (~/.local/bin/ytmgo or /usr/local/bin/ytmgo)
#   2. The config database (~/.config/ytmgo/ytmgo.db)       ← skipped with --keep-user-data
#   3. Downloaded tracks (~/.local/share/ytmgo/downloads/)  ← skipped with --keep-downloads
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

# ─── Parse flags ──────────────────────────────────────────────────────
YES=false
KEEP_DOWNLOADS=false
KEEP_USER_DATA=false

for arg in "$@"; do
  case "$arg" in
    -y|--yes)      YES=true ;;
    --keep-downloads) KEEP_DOWNLOADS=true ;;
    --keep-user-data) KEEP_USER_DATA=true ;;
  esac
done

# ─── Prompt helper ────────────────────────────────────────────────────
# ask "Question?" default_is_yes (true/false)
# Returns 0 (true) for yes, 1 (false) for no.
ask() {
  local question=$1 default_yes=$2 answer
  if [ "$YES" = true ]; then
    return 0  # always yes in silent mode
  fi
  local prompt
  if [ "$default_yes" = true ]; then
    prompt="$question [Y/n] "
  else
    prompt="$question [y/N] "
  fi
  read -r -p "$prompt" answer < /dev/tty || true
  case "${answer,,}" in
    n|no)  return 1 ;;
    "")    [ "$default_yes" = true ] && return 0 || return 1 ;;
    *)     return 0 ;;
  esac
}

BINARY="ytmgo"
BIN_PATH=""

# ─── 1. Remove the binary ────────────────────────────────────────────
if ask "Remove the ytmgo binary?" true; then
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
    if [ -z "$BIN_PATH" ]; then
      warn "No ytmgo binary found — nothing to remove."
    fi
  fi
else
  warn "Skipping binary removal."
fi

# ─── 2. Remove config directory (DB with settings/favorites/history) ──
CONFIG_DIR="$HOME/.config/ytmgo"
if [ "$KEEP_USER_DATA" = true ]; then
  warn "Skipping user data removal (--keep-user-data was passed)."
elif [ -d "$CONFIG_DIR" ]; then
  if ask "Remove user data — settings, favorites, play history?" false; then
    info "Removing config & data: $CONFIG_DIR"
    rm -rf "$CONFIG_DIR"
    success "Removed config (settings, favorites, play history)"
  else
    warn "Skipping user data removal."
  fi
else
  warn "No config directory found at $CONFIG_DIR"
fi

# ─── 3. Remove downloads ─────────────────────────────────────────────
REMOVE_DOWNLOADS=false

if [ "$KEEP_DOWNLOADS" = true ]; then
  warn "Skipping download removal (--keep-downloads was passed)."
elif [ -d "$HOME/.local/share/ytmgo" ] || [ -d "$HOME/Library/Application Support/ytmgo" ] || { [ -n "${XDG_DATA_HOME:-}" ] && [ -d "$XDG_DATA_HOME/ytmgo" ]; }; then
  if ask "Remove downloaded audio files?" false; then
    REMOVE_DOWNLOADS=true
  else
    warn "Skipping download removal."
  fi
else
  warn "No downloads directory found."
fi

if [ "$REMOVE_DOWNLOADS" = true ]; then
  # Default Linux data location
  if [ -d "$HOME/.local/share/ytmgo" ]; then
    info "Removing downloads: $HOME/.local/share/ytmgo"
    rm -rf "$HOME/.local/share/ytmgo"
    success "Removed downloaded tracks"
  fi
  # XDG_DATA_HOME override
  if [ -n "${XDG_DATA_HOME:-}" ] && [ -d "$XDG_DATA_HOME/ytmgo" ]; then
    info "Removing downloads: $XDG_DATA_HOME/ytmgo"
    rm -rf "$XDG_DATA_HOME/ytmgo"
    success "Removed downloaded tracks"
  fi
  # macOS default
  MACOS_DIR="$HOME/Library/Application Support/ytmgo"
  if [ -d "$MACOS_DIR" ]; then
    info "Removing downloads: $MACOS_DIR"
    rm -rf "$MACOS_DIR"
    success "Removed downloaded tracks"
  fi
fi

# ─── Done ─────────────────────────────────────────────────────────────
echo ""
success "ytmgo has been uninstalled."

if [ -n "$BIN_PATH" ]; then
  # Suggest removing from PATH if the bin dir is only for ytmgo
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
