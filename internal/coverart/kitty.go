package coverart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
)

// Kitty's graphics protocol draws the real image instead of
// approximating it with block characters, which lifts the cover from
// the panel's cell count (about 70x58 pixels) to the artwork's own
// resolution.
//
// The escapes are APC sequences (ESC _ G … ESC \). Terminal-width
// calculations treat those as zero-width, so they can sit inside the
// rendered frame without disturbing any layout maths — the image is
// placed with C=1 ("do not move the cursor"), and the cells it covers
// are still emitted as spaces so the text layout is unchanged.
//
// Everything here is best-effort: callers fall back to the half-block
// renderer whenever KittySupported is false or an error comes back.

// coverImageID is the fixed graphics id for the cover. Re-transmitting
// with the same id replaces the previous image, so only one is ever
// resident.
const coverImageID = 1337

// chunkSize is the protocol's maximum payload per escape sequence.
const chunkSize = 4096

// KittySupported reports whether the terminal is kitty, which is the
// only terminal this path targets.
func KittySupported() bool {
	// Multiplexers swallow the graphics escapes, and KITTY_WINDOW_ID is
	// inherited into them — so taking the kitty path inside tmux or
	// screen would draw nothing at all. Fall back to half-blocks there.
	if os.Getenv("TMUX") != "" || strings.HasPrefix(os.Getenv("TERM"), "screen") {
		return false
	}
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	return strings.Contains(strings.ToLower(os.Getenv("TERM")), "kitty")
}

// KittyPlace returns the escape sequence that transmits img and draws
// it across cols x rows cells at the cursor, without moving the cursor.
// The image is downscaled to roughly the cells' pixel area first, so
// the payload stays small; kitty does the final fit.
func KittyPlace(img image.Image, cols, rows int) (string, error) {
	if img == nil || cols < 1 || rows < 1 {
		return "", fmt.Errorf("nothing to place")
	}

	// Cells are about 8x19 device pixels in a typical kitty setup.
	// Encoding much beyond the covered area only inflates the payload.
	scaled := fit(img, cols*10, rows*22)

	var buf bytes.Buffer
	if err := png.Encode(&buf, scaled); err != nil {
		return "", fmt.Errorf("encode cover: %w", err)
	}
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())

	var out strings.Builder
	first := true
	for len(payload) > 0 {
		n := min(chunkSize, len(payload))
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		if first {
			// f=100: PNG. a=T: transmit and place. C=1: keep the cursor
			// where it is. q=2: no acknowledgements to parse.
			fmt.Fprintf(&out, "\x1b_Ga=T,f=100,i=%d,c=%d,r=%d,C=1,q=2,m=%d;%s\x1b\\",
				coverImageID, cols, rows, more, chunk)
			first = false
			continue
		}
		fmt.Fprintf(&out, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
	}
	return out.String(), nil
}

// KittyClear removes the cover image. Images persist until deleted, so
// this must be emitted whenever the panel shows something else.
func KittyClear() string {
	return fmt.Sprintf("\x1b_Ga=d,d=i,i=%d,q=2\x1b\\", coverImageID)
}

// fit downscales img to sit inside maxW x maxH, preserving its aspect
// ratio. Images already smaller are returned untouched.
func fit(img image.Image, maxW, maxH int) image.Image {
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW < 1 || srcH < 1 || (srcW <= maxW && srcH <= maxH) {
		return img
	}
	dstW, dstH := srcW, srcH
	if dstW > maxW {
		dstH = dstH * maxW / dstW
		dstW = maxW
	}
	if dstH > maxH {
		dstW = dstW * maxH / dstH
		dstH = maxH
	}
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		for x := 0; x < dstW; x++ {
			c := sample(img, b, x, y, dstW, dstH)
			i := dst.PixOffset(x, y)
			dst.Pix[i] = c.R
			dst.Pix[i+1] = c.G
			dst.Pix[i+2] = c.B
			dst.Pix[i+3] = 0xff
		}
	}
	return dst
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
