package tui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"ytmgo/internal/coverart"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/visualizer"
	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// worstCaseModel builds a model stuffed with the content that used to
// break the layout: long Japanese titles everywhere, a long status
// line, and a big queue (which lengthens the queue panel title).
func worstCaseModel(t *testing.T, w, h int) Model {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	m := InitialModel()

	nm, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	m = nm.(Model)

	long := "ラブ・ストーリーは突然に - Love Story Wa Totsuzenni (Extended Album Version)"
	for i := 0; i < 30; i++ {
		m.queue.Add(queue.Track{
			ID:          "sZxzPcT1Meg",
			Title:       long,
			Artist:      "小田和正 Kazumasa Oda and Friends Orchestra",
			Duration:    "4:48",
			DurationSec: 288,
		})
	}
	for i := 0; i < 20; i++ {
		m.results = append(m.results, search.Result{
			ID: "m9SMT5ipbxk", Title: long, Uploader: "YOASOBI feat. somebody with a long name", Duration: 213,
		})
	}
	m.setStatus("Now playing: " + long + " — " + long)
	return m
}

// TestLayoutGeometry pins the physical layout to the mouse handler's
// expectations: the controls row must be exactly where handleClick
// looks for it, and no rendered line may exceed the terminal width
// (an overlong line wraps in the terminal and shifts every hit zone
// below it — the "mouse dead in the player bar" bug).
func TestLayoutGeometry(t *testing.T) {
	sizes := [][2]int{{200, 50}, {150, 40}, {120, 35}, {110, 30}, {100, 28}, {90, 26}, {80, 24}}
	for _, size := range sizes {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)

		lines := strings.Split(m.View(), "\n")

		if len(lines) != h {
			t.Errorf("%dx%d: rendered %d lines, want exactly %d", w, h, len(lines), h)
		}
		for i, line := range lines {
			if lw := lipgloss.Width(line); lw > w {
				t.Errorf("%dx%d: line %d is %d cells wide (max %d): %.60q", w, h, i, lw, w, line)
			}
		}

		controlsRow := -1
		for i, line := range lines {
			if strings.Contains(line, "⏮ Prev") {
				controlsRow = i
				break
			}
		}
		panelsEnd := clickPanelStartY + m.panelHeight()
		if want := panelsEnd + 4; controlsRow != want {
			t.Errorf("%dx%d: controls rendered on row %d, mouse handler expects %d", w, h, controlsRow, want)
		}
	}
}

// TestLayoutGeometryAlbums holds the album list and album tracklist to
// the same contract as every other view: exact height, nothing wider
// than the terminal, controls row where the mouse handler expects it.
func TestLayoutGeometryAlbums(t *testing.T) {
	long := "Take Care (Deluxe Edition, Remastered, Anniversary Reissue)"
	for _, size := range [][2]int{{200, 50}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]

		// Album list.
		m := worstCaseModel(t, w, h)
		m.albumMode = true
		for i := 0; i < 25; i++ {
			m.albums = append(m.albums, ytmusic.Album{
				BrowseID: "MPREb_x", Title: long,
				Artist: "小田和正 Kazumasa Oda and Friends", Year: "2015",
			})
		}
		checkPanelGeometry(t, m, w, h, "album list")

		// Album tracklist.
		alb := ytmusic.Album{Title: long, Artist: "Mild High Club"}
		m.openAlbum = &alb
		for i := 0; i < 25; i++ {
			m.albumTracks = append(m.albumTracks, search.Result{
				ID: "sZxzPcT1Meg", Title: "ラブ・ストーリーは突然に - " + long,
				Uploader: "Mild High Club", Duration: 214,
			})
		}
		checkPanelGeometry(t, m, w, h, "album tracks")
	}
}

// TestLayoutGeometryVisualizer holds the visualizer to the same layout
// contract. It takes over the left panel rather than adding rows, so
// the geometry must be identical to every other view.
func TestLayoutGeometryVisualizer(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.vizOn = true

		// No frame yet — the "Listening…" placeholder.
		checkPanelGeometry(t, m, w, h, "visualizer (no frame)")

		// A full-scale frame: every bar pinned at 100 is the widest and
		// tallest the renderer can produce.
		bars := m.vizBars()
		frame := make(visualizer.Frame, bars)
		for i := range frame {
			frame[i] = 100
		}
		m.vizFrame = frame
		checkPanelGeometry(t, m, w, h, "visualizer (full scale)")

		// More bars than the panel was sized for must still not overflow.
		wide := make(visualizer.Frame, bars*3)
		for i := range wide {
			wide[i] = 100
		}
		m.vizFrame = wide
		checkPanelGeometry(t, m, w, h, "visualizer (oversized frame)")
	}
}

// TestLayoutGeometryCover holds the cover panel to the same contract.
// Half-block cells carry ANSI colour on every character, so an
// off-by-one here would overflow the panel invisibly.
func TestLayoutGeometryCover(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 544, 544))
	for y := 0; y < 544; y++ {
		for x := 0; x < 544; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	for _, size := range [][2]int{{200, 50}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]

		m := worstCaseModel(t, w, h)
		m.coverOn = true
		checkPanelGeometry(t, m, w, h, "cover (no image)")

		m.coverLoading = true
		checkPanelGeometry(t, m, w, h, "cover (loading)")

		m.coverLoading = false
		m.coverImg = img
		m.coverURL = "https://example/cover.jpg"
		checkPanelGeometry(t, m, w, h, "cover (image)")

		m.coverImg = nil
		m.coverErr = "fetch cover: HTTP 404 and a very long trailing message that must be truncated"
		checkPanelGeometry(t, m, w, h, "cover (error)")
	}
}

// TestLayoutGeometryCoverKitty runs the cover panel through the kitty
// path. Its escapes must measure zero cells, so the rendered frame has
// to obey exactly the same height and width contract as every other
// view — a miscount here would wrap lines and shift the mouse zones.
func TestLayoutGeometryCoverKitty(t *testing.T) {
	t.Setenv("KITTY_WINDOW_ID", "1")
	if !coverart.KittySupported() {
		t.Fatal("kitty detection did not take effect")
	}
	img := image.NewRGBA(image.Rect(0, 0, 544, 544))
	for y := 0; y < 544; y++ {
		for x := 0; x < 544; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	for _, size := range [][2]int{{200, 50}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.coverOn = true
		m.coverImg = img
		m.coverURL = "https://example/cover.jpg"
		checkPanelGeometry(t, m, w, h, "cover via kitty")

		// The frame must actually carry the graphics escape.
		if !strings.Contains(m.View(), "\x1b_Ga=T") {
			t.Errorf("%dx%d: kitty placement escape missing from the frame", w, h)
		}

		// Switching away must emit the delete, or the image would linger
		// over whatever the panel shows next.
		m.coverOn = false
		if !strings.Contains(m.View(), "\x1b_Ga=d") {
			t.Errorf("%dx%d: cover not cleared when the panel shows something else", w, h)
		}
	}
}

// checkPanelGeometry asserts the shared layout invariants for a model.
func checkPanelGeometry(t *testing.T, m Model, w, h int, what string) {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != h {
		t.Errorf("%s %dx%d: %d lines, want %d", what, w, h, len(lines), h)
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("%s %dx%d: line %d is %d cells (max %d)", what, w, h, i, lw, w)
		}
	}
	controls := -1
	for i, line := range lines {
		if strings.Contains(line, "⏮ Prev") {
			controls = i
			break
		}
	}
	if want := clickPanelStartY + m.panelHeight() + 4; controls != want {
		t.Errorf("%s %dx%d: controls on row %d, mouse expects %d", what, w, h, controls, want)
	}
}

// TestLayoutGeometrySettingsPage does the same check for the Settings
// page, which has its own click routing.
func TestLayoutGeometrySettingsPage(t *testing.T) {
	m := worstCaseModel(t, 120, 35)
	m.switchPage(PageSettings)

	lines := strings.Split(m.View(), "\n")
	if len(lines) != 35 {
		t.Fatalf("settings page rendered %d lines, want 35", len(lines))
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > 120 {
			t.Fatalf("settings page line %d is %d cells wide (max 120)", i, lw)
		}
	}
	controlsRow := -1
	for i, line := range lines {
		if strings.Contains(line, "⏮ Prev") {
			controlsRow = i
			break
		}
	}
	if want := clickPanelStartY + m.panelHeight() + 4; controlsRow != want {
		t.Fatalf("settings page controls on row %d, mouse handler expects %d", controlsRow, want)
	}
}
