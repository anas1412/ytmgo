#!/usr/bin/env bash
# screenshot.sh — regenerate the screenshot used by the README, the docs
# hero and every link preview.
#
# Usage:
#   bash dist/screenshot.sh [output.png]
#
# Needs: freeze (go install github.com/charmbracelet/freeze@latest),
#        ImageMagick, and python3 with Pillow.
#
# Why it is built rather than photographed: a staged screenshot goes
# stale silently — the one this replaced still showed v0.13 long after
# the UI had moved on. Here the frame comes from the app itself, so
# refreshing it is one command.
#
# The album art is composited in at full resolution afterwards. A
# captured text frame can only draw art as half-blocks, roughly ten
# pixels across, because the kitty graphics protocol is a separate
# channel a text capture cannot carry. The covers are pasted into the
# exact cells kitty places them in, so the result matches what a kitty,
# Ghostty or WezTerm user actually sees rather than the fallback.

set -euo pipefail

OUT="${1:-ytmgo-v1.1.png}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

freeze_frame() {
  freeze --output "$2" --background "#1a1b26" --padding 24 --margin 0 \
    --border.radius 8 --font.size 13 --line-height 1.2 --window=false "$1"
}

echo "==> Rendering the frame"
YTMGO_CAPTURE="$WORK/art.ans" YTMGO_CAPTURE_URLS="$WORK/urls.txt" \
  go test ./internal/tui/ -run TestCaptureFrame -count=1 >/dev/null

# The same frame with the art replaced by a solid colour. Diffing the
# two would not work — with no image at all the text beside it shifts
# left — so the sentinel keeps the layout and marks only the art.
echo "==> Rendering a sentinel frame to locate the art"
YTMGO_CAPTURE="$WORK/sentinel.ans" YTMGO_CAPTURE_NOART=1 \
  go test ./internal/tui/ -run TestCaptureFrame -count=1 >/dev/null

echo "==> Converting to PNG (freeze is slow, ~1 min each)"
freeze_frame "$WORK/art.ans" "$WORK/art.png"
freeze_frame "$WORK/sentinel.ans" "$WORK/sentinel.png"

# Ask the image host for more pixels than the app does: the app wants
# 544px for a block of terminal cells, the composite wants a clean
# downscale into roughly 325.
sed 's/w544-h544/w800-h800/g' "$WORK/urls.txt" > "$WORK/urls_big.txt"
i=0
while read -r u; do
  [ -n "$u" ] || continue
  i=$((i + 1))
  curl -fsSL -o "$WORK/src$i.jpg" "$u"
done < "$WORK/urls_big.txt"

echo "==> Compositing the covers"
WORK="$WORK" OUT="$OUT" python3 - <<'PY'
from PIL import Image
import os, sys

work, out = os.environ['WORK'], os.environ['OUT']
im = Image.open(f'{work}/sentinel.png').convert('RGB')
w, h = im.size
px = im.load()


def sentinel(p):
    r, g, b = p
    return r > 150 and b > 150 and g < 90


rows = [y for y in range(h) if any(sentinel(px[x, y]) for x in range(0, w, 2))]
bands = []
for y in rows:
    if bands and y - bands[-1][1] <= 8:
        bands[-1][1] = y
    else:
        bands.append([y, y])

boxes = []
for y0, y1 in bands:
    xs = [x for x in range(w) if any(sentinel(px[x, y]) for y in range(y0, y1 + 1))]
    boxes.append((min(xs), y0, max(xs), y1))

if len(boxes) != 2:
    sys.exit(f'expected 2 art rectangles, found {len(boxes)}: {boxes}')

base = Image.open(f'{work}/art.png').convert('RGB')
for (x0, y0, x1, y1), n in zip(boxes, (1, 2)):
    bw, bh = x1 - x0 + 1, y1 - y0 + 1
    src = Image.open(f'{work}/src{n}.jpg').convert('RGB')
    sw, sh = src.size
    scale = max(bw / sw, bh / sh)
    src = src.resize((max(1, round(sw * scale)), max(1, round(sh * scale))), Image.LANCZOS)
    left, top = (src.width - bw) // 2, (src.height - bh) // 2
    base.paste(src.crop((left, top, left + bw, top + bh)), (x0, y0))
    print(f'    cover {n} at {x0},{y0} ({bw}x{bh})')

base.resize((1200, round(1200 * base.height / base.width)), Image.LANCZOS).save(out)
print(f'==> Wrote {out}')
PY
