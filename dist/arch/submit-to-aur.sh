#!/usr/bin/env bash
# submit-to-aur.sh — prepare and submit ytmgo to Arch User Repository
#
# Prerequisites:
#   1. Arch Linux account: https://aur.archlinux.org/register/
#   2. SSH key added to your AUR account: https://aur.archlinux.org/account/
#   3. base-devel installed: sudo pacman -S base-devel
#
# Usage:
#   bash dist/arch/submit-to-aur.sh
#
# This script will:
#   1. Clone the AUR repo (or pull if already cloned)
#   2. Copy the PKGBUILD
#   3. Generate .SRCINFO
#   4. Show you the diff and ask to commit + push

set -euo pipefail

REPO="ytmgo"
AUR_SSH="ssh://aur@aur.archlinux.org/$REPO.git"
WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

echo "==> Cloning AUR repository..."
if ! git clone "$AUR_SSH" "$WORKDIR" 2>/dev/null; then
  echo "✗ Failed to clone AUR repo."
  echo "  Make sure you have an Arch Linux account and your SSH key is set up."
  echo "  https://aur.archlinux.org/register/"
  exit 1
fi

echo "==> Copying PKGBUILD..."
cp "$(dirname "$0")/PKGBUILD" "$WORKDIR/PKGBUILD"
cd "$WORKDIR"

echo "==> Generating .SRCINFO..."
makepkg --printsrcinfo > .SRCINFO

echo ""
echo "=== Review changes ==="
git diff

echo ""
read -p "Commit and push to AUR? [y/N] " confirm
case "${confirm,,}" in
  y|yes)
    git add PKGBUILD .SRCINFO
    git commit -m "Update to $(grep -oP 'pkgver=\K.*' PKGBUILD)"
    git push
    echo "✓ Submitted! https://aur.archlinux.org/packages/$REPO"
    ;;
  *)
    echo "Cancelled. Changes are in $WORKDIR"
    echo "Commit manually: cd $WORKDIR && git commit -m 'update' && git push"
    ;;
esac
