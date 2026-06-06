#!/usr/bin/env bash
# push-to-aur.sh — push ytmgo PKGBUILD update to Arch User Repository
#
# Prerequisites:
#   1. Arch Linux account: https://aur.archlinux.org/register/
#   2. SSH key at ~/.ssh/aur, registered at https://aur.archlinux.org/account/
#      (run `ssh-keygen -t ed25519 -f ~/.ssh/aur -N "" -C "aur@archlinux.org"` if needed)
#   3. base-devel installed: sudo pacman -S base-devel
#
# Usage:
#   bash dist/arch/push-to-aur.sh <version>
#
# Example:
#   bash dist/arch/push-to-aur.sh v0.4.0
#
# What it does:
#   1. Generates PKGBUILD + .SRCINFO for the given version
#   2. Clones or pulls the AUR repo to /tmp/ytmgo-aur
#   3. Copies the generated files
#   4. Shows you the diff and asks before committing + pushing
#
# For first-time submission (package doesn't exist on AUR yet):
#   The script will auto-create it on first push.

set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>"
  echo "  version — e.g. v0.4.0"
  echo ""
  echo "Latest git tag: $(git describe --tags --abbrev=0 2>/dev/null || echo 'none')"
  exit 1
fi

REPO="ytmgo"
AUR_SSH="ssh://aur@aur.archlinux.org/$REPO.git"
CLONE_DIR="/tmp/ytmgo-aur"

echo "==> Generating PKGBUILD + .SRCINFO for $VERSION ..."
GEN_DIR=$(mktemp -d)
bash "$(dirname "$0")/generate-pkgbuild.sh" "$VERSION" --out "$GEN_DIR"

echo "==> Syncing AUR repository..."
if [ -d "$CLONE_DIR" ]; then
  echo "  Pulling latest from AUR..."
  cd "$CLONE_DIR"
  git pull --ff-only origin master
else
  echo "  Cloning AUR repository..."
  git clone "$AUR_SSH" "$CLONE_DIR"
  cd "$CLONE_DIR"
fi

echo "==> Updating files..."
cp "$GEN_DIR/PKGBUILD" "$CLONE_DIR/PKGBUILD"
cp "$GEN_DIR/.SRCINFO" "$CLONE_DIR/.SRCINFO"
rm -rf "$GEN_DIR"

# Check if anything actually changed
cd "$CLONE_DIR"
if git diff --quiet && git diff --cached --quiet && [[ -z "$(git status --porcelain)" ]]; then
  echo ""
  echo "✓ No changes — AUR is already at $VERSION."
  exit 0
fi

echo ""
echo "=== Changes to be pushed ==="
git diff --stat
echo ""

read -p "Commit and push to AUR? [y/N] " confirm
case "${confirm,,}" in
  y|yes)
    git add PKGBUILD .SRCINFO
    git commit -m "Update to $VERSION"
    git push origin master
    echo ""
    echo "✓ Pushed! https://aur.archlinux.org/packages/$REPO"
    ;;
  *)
    echo "Cancelled. Changes are in $CLONE_DIR"
    echo "Review: cd $CLONE_DIR && git diff"
    echo "Commit manually: cd $CLONE_DIR && git add PKGBUILD .SRCINFO && git commit -m 'update' && git push"
    ;;
esac
