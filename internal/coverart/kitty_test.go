package coverart

import (
	"image"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestKittySupportedFromEnv(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "")
	t.Setenv("TERM", "xterm-256color")
	if KittySupported() {
		t.Error("plain xterm should not be treated as kitty")
	}
	t.Setenv("TERM", "xterm-kitty")
	if !KittySupported() {
		t.Error("TERM=xterm-kitty should be detected")
	}
	t.Setenv("TERM", "dumb")
	t.Setenv("KITTY_WINDOW_ID", "3")
	if !KittySupported() {
		t.Error("KITTY_WINDOW_ID should be detected")
	}
}

// TestKittyNotUsedInMultiplexers: tmux and screen swallow the graphics
// escapes, and KITTY_WINDOW_ID leaks into them, so the kitty path there
// would render an empty panel instead of falling back to half-blocks.
func TestKittyNotUsedInMultiplexers(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	t.Setenv("TERM", "xterm-kitty")

	t.Setenv("TMUX", "/tmp/tmux-1000/default,123,0")
	if KittySupported() {
		t.Error("kitty path must be disabled inside tmux")
	}

	t.Setenv("TMUX", "")
	t.Setenv("TERM", "screen-256color")
	if KittySupported() {
		t.Error("kitty path must be disabled inside screen")
	}
}

// TestKittyPlaceIsZeroWidth is the property the whole integration rests
// on: the escapes must not count towards the rendered line's width, or
// every padding and truncation calculation in the TUI would be wrong.
func TestKittyPlaceIsZeroWidth(t *testing.T) {
	esc, err := KittyTransmit(solid(544, 544), 40, 20)
	if err != nil {
		t.Fatalf("KittyTransmit: %v", err)
	}
	esc += KittyDisplay(40, 20)
	if w := lipgloss.Width(esc); w != 0 {
		t.Errorf("placement escape measures %d cells, want 0", w)
	}
	if w := lipgloss.Width(KittyClear()); w != 0 {
		t.Errorf("clear escape measures %d cells, want 0", w)
	}
	// And still zero-width once padded into a real line.
	line := esc + strings.Repeat(" ", 40)
	if w := lipgloss.Width(line); w != 40 {
		t.Errorf("escape + 40 spaces measures %d, want 40", w)
	}
}

// TestKittyPlaceChunking checks the payload is split into valid APC
// sequences with the continuation flags the protocol requires.
func TestKittyPlaceChunking(t *testing.T) {
	esc, err := KittyTransmit(solid(544, 544), 60, 30)
	if err != nil {
		t.Fatalf("KittyTransmit: %v", err)
	}
	if !strings.HasPrefix(esc, "\x1b_Ga=t,f=100,") {
		t.Errorf("first chunk lacks the transmit-and-place header: %.40q", esc)
	}
	chunks := strings.Count(esc, "\x1b_G")
	if chunks < 1 {
		t.Fatal("no escape sequences emitted")
	}
	if terms := strings.Count(esc, "\x1b\\"); terms != chunks {
		t.Errorf("%d escapes but %d terminators", chunks, terms)
	}
	// Every chunk but the last must say "more follows".
	more := regexp.MustCompile(`m=1`)
	if got := len(more.FindAllString(esc, -1)); got != chunks-1 {
		t.Errorf("m=1 appears %d times, want %d (all but the final chunk)", got, chunks-1)
	}
	if !strings.Contains(esc, "m=0;") {
		t.Error("final chunk is not marked m=0")
	}
	// Payload chunks must respect the protocol's size limit.
	for _, part := range strings.Split(esc, "\x1b_G")[1:] {
		body := part
		if i := strings.Index(body, ";"); i >= 0 {
			body = body[i+1:]
		}
		body = strings.TrimSuffix(body, "\x1b\\")
		if len(body) > chunkSize {
			t.Errorf("chunk of %d bytes exceeds the %d limit", len(body), chunkSize)
		}
	}
}

func TestKittyTransmitRejectsBadInput(t *testing.T) {
	if _, err := KittyTransmit(nil, 10, 10); err == nil {
		t.Error("nil image should error")
	}
	if _, err := KittyTransmit(solid(10, 10), 0, 5); err == nil {
		t.Error("zero columns should error")
	}
}

// TestKittyDisplayIsCheap: the per-frame escape must stay tiny, since
// it is emitted at the spectrum's frame rate. Re-sending the image
// there is what made the UI lag.
func TestKittyDisplayIsCheap(t *testing.T) {
	d := KittyDisplay(24, 10)
	if len(d) > 80 {
		t.Errorf("per-frame placement is %d bytes, expected well under 80", len(d))
	}
	if lipgloss.Width(d) != 0 {
		t.Errorf("placement escape measures %d cells, want 0", lipgloss.Width(d))
	}
	if !strings.Contains(d, "a=p") || !strings.Contains(d, "p=1") {
		t.Errorf("placement should reuse one placement id: %q", d)
	}
	// The transmit carries the pixels and the placement does not — the
	// point of splitting them. (Absolute sizes depend on how well the
	// image compresses, so only the relationship is asserted.)
	big, err := KittyTransmit(solid(544, 544), 24, 10)
	if err != nil {
		t.Fatalf("KittyTransmit: %v", err)
	}
	if len(big) <= len(d) {
		t.Errorf("transmit (%d bytes) should be larger than the placement (%d)", len(big), len(d))
	}
	if strings.Contains(d, "f=100") {
		t.Error("per-frame placement must not carry image data")
	}
}

// TestFitPreservesAspect: downscaling for transmission must not distort.
func TestFitPreservesAspect(t *testing.T) {
	out := fit(solid(544, 544), 100, 220)
	b := out.Bounds()
	if b.Dx() != b.Dy() {
		t.Errorf("square source became %dx%d", b.Dx(), b.Dy())
	}
	if b.Dx() > 100 || b.Dy() > 220 {
		t.Errorf("result %dx%d exceeds the box", b.Dx(), b.Dy())
	}
	// Already-small images are passed through untouched.
	small := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if got := fit(small, 100, 100); got != image.Image(small) {
		t.Error("small image should be returned unchanged")
	}
}
