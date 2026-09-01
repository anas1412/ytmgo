package coverart

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
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

// CoverImageID is the fixed graphics id for the player bar's cover.
// Re-transmitting with the same id replaces the previous image, so only
// one is ever resident per id. AlbumImageID is the second slot, for the
// open album's art in the browse panel — the two are on screen at once.
const CoverImageID = 1337

// AlbumImageID is the kitty image id for the open album's cover.
const AlbumImageID = 1338

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

// transmitCache memoises the encoded transmit, so emitting it across a
// few frames costs one encode rather than several.
var (
	transmitMu    sync.Mutex
	transmitKey   string
	transmitValue string
)

// KittyTransmitCached is KittyTransmitID memoised on the artwork, size
// and image id.
func KittyTransmitCached(img image.Image, key string, cols, rows, id int) (string, error) {
	full := fmt.Sprintf("%s|%d|%d|%d", key, cols, rows, id)
	transmitMu.Lock()
	if transmitKey == full {
		v := transmitValue
		transmitMu.Unlock()
		return v, nil
	}
	transmitMu.Unlock()

	esc, err := KittyTransmitID(img, cols, rows, id)
	if err != nil {
		return "", err
	}
	transmitMu.Lock()
	transmitKey, transmitValue = full, esc
	transmitMu.Unlock()
	return esc, nil
}

// KittyTransmit sends the image under the player-cover id.
func KittyTransmit(img image.Image, cols, rows int) (string, error) {
	return KittyTransmitID(img, cols, rows, CoverImageID)
}

// KittyTransmitID sends the image to the terminal under the given id,
// without displaying it. This is the expensive half — PNG encoding and
// base64 of the whole image — so it must run only when the artwork
// changes, never once per rendered frame.
func KittyTransmitID(img image.Image, cols, rows, id int) (string, error) {
	if img == nil || cols < 1 || rows < 1 {
		return "", fmt.Errorf("nothing to transmit")
	}

	// Cells are about 8x19 device pixels in a typical kitty setup;
	// encoding much beyond the covered area only inflates the payload.
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
			// a=t: transmit only. f=100: PNG. q=2: no acknowledgements.
			fmt.Fprintf(&out, "\x1b_Ga=t,f=100,i=%d,q=2,m=%d;%s\x1b\\",
				id, more, chunk)
			first = false
			continue
		}
		fmt.Fprintf(&out, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
	}
	return out.String(), nil
}

// KittyDisplay places the player-cover image.
func KittyDisplay(cols, rows int) string {
	return KittyDisplayID(cols, rows, CoverImageID)
}

// KittyDisplayID draws the already-transmitted image across cols x rows
// cells at the cursor, without moving it. This is the cheap half, safe
// to emit on every frame: a fixed placement id per image means repeated
// calls update one placement instead of stacking new ones.
func KittyDisplayID(cols, rows, id int) string {
	return fmt.Sprintf("\x1b_Ga=p,i=%d,p=1,c=%d,r=%d,C=1,q=2\x1b\\",
		id, cols, rows)
}

// KittyClear removes the player-cover image.
func KittyClear() string {
	return KittyClearID(CoverImageID)
}

// KittyClearID removes one image and its placement. Images persist
// until deleted, so this must be emitted whenever the UI stops showing
// one — otherwise the artwork stays on screen over whatever is drawn
// next.
func KittyClearID(id int) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d,q=2\x1b\\", id)
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
