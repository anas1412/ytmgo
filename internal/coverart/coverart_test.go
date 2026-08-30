package coverart

import (
	"image"
	"image/color"
	"testing"
)

// solid builds a test image split down the middle: red left, blue right.
func solid(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{255, 0, 0, 255}
			if x >= w/2 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

// TestGridFitsBounds: the grid must never exceed the cells it was given,
// or it would overflow the panel and shift the mouse hit zones below it.
func TestGridFitsBounds(t *testing.T) {
	img := solid(544, 544)
	for _, size := range [][2]int{{40, 20}, {70, 30}, {10, 4}, {1, 1}} {
		cw, ch := size[0], size[1]
		rows := Grid(img, cw, ch)
		w, h := Describe(rows)
		if w > cw || h > ch {
			t.Errorf("%dx%d cells: grid is %dx%d, exceeds bounds", cw, ch, w, h)
		}
		for i, r := range rows {
			if len(r) != w {
				t.Errorf("%dx%d: row %d has %d cells, want %d", cw, ch, i, len(r), w)
			}
		}
	}
}

// TestGridAspect: a square image should look square on screen — twice as
// many columns as rows, since one cell stacks two pixels.
func TestGridAspect(t *testing.T) {
	rows := Grid(solid(544, 544), 60, 40)
	w, h := Describe(rows)
	if h == 0 {
		t.Fatal("empty grid")
	}
	// On screen the grid is w wide and h*CellAspect tall in cell-width
	// units; a square source should make those match.
	ratio := float64(w) / (float64(h) * CellAspect)
	if ratio < 0.88 || ratio > 1.12 {
		t.Errorf("square image rendered at %dx%d cells (on-screen ratio %.2f), want ~1.0", w, h, ratio)
	}
}

// TestGridWideImage checks a non-square source is fitted, not stretched.
func TestGridWideImage(t *testing.T) {
	rows := Grid(solid(800, 200), 60, 40)
	w, h := Describe(rows)
	if w == 0 || h == 0 {
		t.Fatal("empty grid")
	}
	ratio := float64(w) / (float64(h) * CellAspect)
	if ratio < 3.2 || ratio > 4.8 {
		t.Errorf("4:1 image rendered at %dx%d cells (ratio %.2f), want ~4.0", w, h, ratio)
	}
}

func TestGridSamplesColour(t *testing.T) {
	rows := Grid(solid(100, 100), 20, 10)
	if len(rows) == 0 || len(rows[0]) < 4 {
		t.Fatal("grid too small to check")
	}
	left := rows[0][0].Top
	right := rows[0][len(rows[0])-1].Top
	if left.R < 200 || left.B > 60 {
		t.Errorf("left edge = %v, want red", left)
	}
	if right.B < 200 || right.R > 60 {
		t.Errorf("right edge = %v, want blue", right)
	}
}

func TestGridDegenerateInput(t *testing.T) {
	if Grid(nil, 10, 10) != nil {
		t.Error("nil image should give no grid")
	}
	if Grid(solid(10, 10), 0, 0) != nil {
		t.Error("zero size should give no grid")
	}
	if Grid(solid(10, 10), -5, -5) != nil {
		t.Error("negative size should give no grid")
	}
}

func TestHex(t *testing.T) {
	if got := (RGB{255, 0, 128}).Hex(); got != "#ff0080" {
		t.Errorf("Hex() = %q", got)
	}
	if got := (RGB{0, 0, 0}).Hex(); got != "#000000" {
		t.Errorf("Hex() = %q", got)
	}
}

func TestLoadRejectsEmptyURL(t *testing.T) {
	if _, err := Load(""); err == nil {
		t.Error("empty URL should error")
	}
}
