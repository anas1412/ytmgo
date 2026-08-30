// Package coverart fetches album art and renders it as terminal text.
//
// Images are drawn with the upper-half-block character: the glyph's
// foreground paints the top pixel and its background the bottom one, so
// each cell carries two pixels. Because a terminal cell is roughly
// twice as tall as it is wide, those half-cells come out approximately
// square, and the result is ordinary styled text — it composes with the
// TUI's renderer instead of fighting it the way a terminal graphics
// protocol would.
package coverart

import (
	"fmt"
	"image"
	_ "image/jpeg" // cover art is JPEG in practice
	_ "image/png"  // …but decode PNG too, just in case
	"net/http"
	"strings"
	"sync"
	"time"
)

// CellAspect is how many times taller a terminal cell is than it is
// wide. It cannot be queried portably, and getting it wrong stretches
// the image: 2.0 is the textbook value, but most modern terminals and
// fonts land nearer 2.4, which is what this is tuned to.
var CellAspect = 2.4

const (
	fetchTimeout = 15 * time.Second
	maxCached    = 24 // a session's worth of covers; each is small once decoded
)

var client = &http.Client{Timeout: fetchTimeout}

var (
	mu     sync.Mutex
	cache  = map[string]image.Image{}
	recent []string // insertion order, for trimming the oldest
)

// Load fetches and decodes the image at url, caching the result so
// re-displaying a cover costs nothing.
func Load(url string) (image.Image, error) {
	if url == "" {
		return nil, fmt.Errorf("no cover art for this track")
	}

	mu.Lock()
	if img, ok := cache[url]; ok {
		mu.Unlock()
		return img, nil
	}
	mu.Unlock()

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch cover: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch cover: HTTP %d", resp.StatusCode)
	}
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decode cover: %w", err)
	}

	mu.Lock()
	cache[url] = img
	recent = append(recent, url)
	for len(recent) > maxCached {
		delete(cache, recent[0])
		recent = recent[1:]
	}
	mu.Unlock()
	return img, nil
}

// RGB is one pixel's colour, ready for a terminal style.
type RGB struct{ R, G, B uint8 }

// Hex renders the colour as "#rrggbb".
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// Cell is one character position: two vertically stacked pixels.
type Cell struct{ Top, Bottom RGB }

// Grid renders img as rows of cells that fit inside cellW x cellH,
// preserving the image's aspect ratio and centring the result.
// The caller styles each cell — this package stays free of any
// particular terminal-styling library.
func Grid(img image.Image, cellW, cellH int) [][]Cell {
	if img == nil || cellW < 1 || cellH < 1 {
		return nil
	}
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW < 1 || srcH < 1 {
		return nil
	}

	// A cell is CellAspect times taller than it is wide, and holds two
	// stacked pixels, so one pixel is CellAspect/2 cell-heights tall.
	// A pxW x pxH grid therefore appears pxW wide and pxH*CellAspect/2
	// tall in cell-width units; squaring that is what keeps the cover
	// from being stretched.
	maxPxW, maxPxH := cellW, cellH*2
	pxW := maxPxW
	pxH := int(float64(pxW) * float64(srcH) / float64(srcW) * 2 / CellAspect)
	if pxH > maxPxH {
		pxH = maxPxH
		pxW = int(float64(pxH) * float64(srcW) / float64(srcH) * CellAspect / 2)
		if pxW > maxPxW {
			pxW = maxPxW
		}
	}
	if pxW < 1 || pxH < 1 {
		return nil
	}
	pxH -= pxH % 2 // whole cells only
	if pxH < 2 {
		pxH = 2
	}

	rows := make([][]Cell, 0, pxH/2)
	for y := 0; y < pxH; y += 2 {
		row := make([]Cell, 0, pxW)
		for x := 0; x < pxW; x++ {
			row = append(row, Cell{
				Top:    sample(img, b, x, y, pxW, pxH),
				Bottom: sample(img, b, x, y+1, pxW, pxH),
			})
		}
		rows = append(rows, row)
	}
	return rows
}

// sample box-averages the source pixels mapping to one output pixel,
// which reads far better than nearest-neighbour at these sizes.
func sample(img image.Image, b image.Rectangle, x, y, outW, outH int) RGB {
	x0 := b.Min.X + x*b.Dx()/outW
	x1 := b.Min.X + (x+1)*b.Dx()/outW
	y0 := b.Min.Y + y*b.Dy()/outH
	y1 := b.Min.Y + (y+1)*b.Dy()/outH
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}

	var rs, gs, bs, n uint64
	for yy := y0; yy < y1 && yy < b.Max.Y; yy++ {
		for xx := x0; xx < x1 && xx < b.Max.X; xx++ {
			r, g, bl, _ := img.At(xx, yy).RGBA()
			rs += uint64(r >> 8)
			gs += uint64(g >> 8)
			bs += uint64(bl >> 8)
			n++
		}
	}
	if n == 0 {
		return RGB{}
	}
	return RGB{uint8(rs / n), uint8(gs / n), uint8(bs / n)}
}

// HalfBlock is the glyph whose foreground is the upper pixel and
// background the lower one.
const HalfBlock = "▀"

// Describe reports the grid's size in cells, for callers that need to
// centre it.
func Describe(rows [][]Cell) (w, h int) {
	if len(rows) == 0 {
		return 0, 0
	}
	return len(rows[0]), len(rows)
}

// Blank returns a run of spaces, used to centre a rendered cover.
func Blank(n int) string {
	if n < 1 {
		return ""
	}
	return strings.Repeat(" ", n)
}
