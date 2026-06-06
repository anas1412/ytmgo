#!/usr/bin/env bash
# push-to-aur.sh — push PKGBUILD updates to Arch User Repository
#
# Prerequisites:
#   1. Arch Linux account: https://aur.archlinux.org/register/
#   2. SSH key added to your AUR account: https://aur.archlinux.org/account/
#   3. base-devel installed: sudo pacman -S base-devel
#
# Usage:
#   bash dist/arch/push-to-aur.sh
#
# What it does:
#   1. Clones or pulls the AUR repo to /tmp/ytmgo-aur
#   2. Copies the PKGBUILD from this repo (dist/arch/PKGBUILD)
#   3. Generates .SRCINFO
#   4. Shows you the diff and asks before committing + pushing
#
# For first-time submission (package doesn't exist on AUR yet):
#   The script will auto-create it if git push succeeds.

set -euo pipefail

REPO="ytmgo"
AUR_SSH="ssh://aur@aur.archlinux.org/$REPO.git"
CLONE_DIR="/tmp/ytmgo-aur"

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

PKGBUILD_SRC="$(dirname "$0")/PKGBUILD"
echo "==> Copying PKGBUILD from $PKGBUILD_SRC ..."
cp "$PKGBUILD_SRC" "$CLONE_DIR/PKGBUILD"

echo "==> Generating .SRCINFO..."
cd "$CLONE_DIR"
makepkg --printsrcinfo > .SRCINFO

# Check if anything actually changed
if git diff --quiet && git diff --cached --quiet && [[ -z "$(git status --porcelain)" ]]; then
  echo ""
  echo "✓ No changes — PKGBUILD and .SRCINFO are up to date."
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
    pkgver=$(grep -oP 'pkgver=\K.*' PKGBUILD)
    pkgrel=$(grep -oP 'pkgrel=\K.*' PKGBUILD)
    git commit -m "Update to ${pkgver}-${pkgrel}"
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
