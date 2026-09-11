package tui

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"testing"

	"ytmgo/internal/player"
	"ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// TestCaptureFrame renders one realistic frame to a file, for the
// screenshot on the README and the site. Skipped unless YTMGO_CAPTURE
// names an output path.
func TestCaptureFrame(t *testing.T) {
	out := os.Getenv("YTMGO_CAPTURE")
	if out == "" {
		t.Skip("set YTMGO_CAPTURE=<path> to write a frame")
	}
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	ApplyTheme("tokyo-night")

	m := worstCaseModel(t, 150, 42)
	m.queue.Clear()
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.searchFocused = false      // the cursor row is only highlighted when the list has focus
	m.settings.ShowHints = false // a clean frame: the hints are for using it, not for looking at it
	version.Version = "v1.1.1"

	msg := openArtistCmd("UCRr1xG_2WIDs18a6cIiCxeA", m.artistSeq+1)()
	m.artistSeq++
	nm, _ := m.handleArtistLoaded(msg.(ArtistLoadedMsg))
	m = nm.(Model)
	m.searchCursor = 0

	// A real queue, from the artist's own songs.
	for i, r := range m.artistSongs {
		if i >= 8 {
			break
		}
		m.queue.Add(m.resolveTrack(r))
	}
	m.queue.SetCurrentIndex(1)
	m.playerState = player.StatePlaying
	m.position = 112
	m.duration = 337
	m.volume = 80

	// Load the artist photo so the strip is not an empty box.
	if cmd := loadAlbumArtCmd(m.artistArtURL, m.albumSeq); cmd != nil {
		if art, ok := cmd().(AlbumArtLoadedMsg); ok && art.Err == nil {
			m.albumArtImg = art.Img
			m.albumArtURL = art.URL
		}
	}
	if cmd := m.refreshCoverCmd(); cmd != nil {
		if cv, ok := cmd().(CoverLoadedMsg); ok && cv.Err == nil {
			m.coverImg = cv.Img
			m.coverURL = cv.URL
		}
	}
	nm, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = nm.(Model)

	// Real lyrics, rather than a spinner.
	if cur, ok := m.queue.Current(); ok {
		m.lyricsSeq++
		if msg := fetchLyricsCmd(cur, m.lyricsSeq, nil)(); msg != nil {
			nm, _ = m.Update(msg)
			m = nm.(Model)
		}
	}

	// A solid sentinel rather than no image: dropping the art changes
	// the layout (the text beside it shifts left when there is nothing
	// to inset past), so a diff would flag half the strip. Same size,
	// one colour — the art rectangles are then exactly the magenta.
	if os.Getenv("YTMGO_CAPTURE_NOART") != "" {
		fill := func(src image.Image) image.Image {
			if src == nil {
				return nil
			}
			b := src.Bounds()
			out := image.NewRGBA(b)
			draw.Draw(out, b, &image.Uniform{color.RGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)
			return out
		}
		m.albumArtImg = fill(m.albumArtImg)
		m.coverImg = fill(m.coverImg)
	}

	// Set the profile last: the helpers above touch it, and a frame
	// rendered under the default (no TTY under `go test`) downsamples
	// the accent to basic ANSI, which is what made the cursor row come
	// out dark-on-dark.
	lipgloss.SetColorProfile(termenv.TrueColor)
	termenv.SetDefaultOutput(termenv.NewOutput(os.Stdout, termenv.WithProfile(termenv.TrueColor)))
	ApplyTheme("tokyo-night") // rebuild the package styles under that profile

	// The art rectangles get the real covers composited in afterwards
	// at full resolution — half-blocks are ten pixels wide, which is
	// what a terminal without the kitty graphics protocol is stuck
	// with, not what the app looks like in one that has it.
	if u := os.Getenv("YTMGO_CAPTURE_URLS"); u != "" {
		_ = os.WriteFile(u, []byte(m.albumArtURL+"\n"+m.coverURL+"\n"), 0o644)
	}

	if err := os.WriteFile(out, []byte(m.View()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %s", out)
}
