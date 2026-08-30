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

// TestLayoutGeometryNowPlaying holds the now-playing sub-panel to the
// layout contract. It splits the left column for the first time, so a
// miscount would shift every mouse hit zone in that column.
func TestLayoutGeometryNowPlaying(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 544, 544))
	for y := 0; y < 544; y++ {
		for x := 0; x < 544; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	frame := make(visualizer.Frame, 64)
	for i := range frame {
		frame[i] = 100 // every bar at full height: the widest, tallest case
	}

	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {100, 30}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		for _, kitty := range []bool{false, true} {
			if kitty {
				t.Setenv("TMUX", "")
				t.Setenv("KITTY_WINDOW_ID", "1")
			} else {
				t.Setenv("KITTY_WINDOW_ID", "")
				t.Setenv("TERM", "xterm-256color")
			}
			label := "now-playing"
			if kitty {
				label += " (kitty)"
			}

			m := worstCaseModel(t, w, h)
			m.npOn = true
			m.coverImg = img
			m.coverURL = "https://example/cover.jpg"
			m.vizFrame = frame
			checkPanelGeometry(t, m, w, h, label)

			// Also correct before the art or the first frame arrive.
			m.coverImg = nil
			m.vizFrame = nil
			m.coverLoading = true
			checkPanelGeometry(t, m, w, h, label+" loading")
		}
	}
}

// TestNowPlayingRefusesShortTerminals: opening the panel must never
// crush the results list below a usable size.
func TestNowPlayingRefusesShortTerminals(t *testing.T) {
	for _, size := range [][2]int{{150, 40}, {120, 35}, {100, 30}, {90, 24}, {80, 20}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.npOn = true
		resultsH, npH := m.leftPanelSplit()
		if npH == 0 {
			if !m.npFits() {
				continue // correctly refused
			}
			t.Errorf("%dx%d: panel collapsed although npFits() says it fits", w, h)
			continue
		}
		if resultsH < resultsMinRows {
			t.Errorf("%dx%d: results left with %d rows, below the %d minimum", w, h, resultsH, resultsMinRows)
		}
		if npH < npMinRows {
			t.Errorf("%dx%d: sub-panel got %d rows, below the %d minimum", w, h, npH, npMinRows)
		}
		if got := resultsH + npH; got != m.panelHeight()-6 {
			t.Errorf("%dx%d: split sums to %d, want %d", w, h, got, m.panelHeight()-6)
		}
	}
}

// TestCoverEscapesSurviveDiscardedFrames guards the bug that left the
// artwork stuck on the first track and still on screen after closing:
// the transmit and delete used to be emitted from inside View, which
// Bubble Tea calls more often than it flushes, so whichever frame
// carried them could simply be thrown away. Update now owns the
// decision and each escape is repeated across several frames.
func TestCoverEscapesSurviveDiscardedFrames(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	img := image.NewRGBA(image.Rect(0, 0, 544, 544))
	for y := 0; y < 544; y++ {
		for x := 0; x < 544; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), 40, 90, 255})
		}
	}

	m := worstCaseModel(t, 150, 40)
	m.npOn = true
	m.coverImg = img
	m.coverURL = "https://example/a.jpg"
	m.coverSendN = coverSendFrames

	// Rendering the same state repeatedly must keep carrying the
	// transmit — View is pure, so a discarded frame changes nothing.
	for i := 0; i < 3; i++ {
		if !strings.Contains(m.View(), "\x1b_Ga=t") {
			t.Fatalf("render %d dropped the transmit even though Update still owes it", i)
		}
	}

	// Once Update has counted the frames down, only the cheap placement
	// is emitted — re-sending the pixels every frame is what caused the
	// lag.
	for i := 0; i < coverSendFrames; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
	}
	frame := m.View()
	if strings.Contains(frame, "\x1b_Ga=t") {
		t.Error("still re-transmitting the image after the countdown")
	}
	if !strings.Contains(frame, "\x1b_Ga=p") {
		t.Error("resident image is no longer being placed")
	}

	// Closing the panel must carry the delete, repeatedly, or the image
	// stays on screen over whatever is drawn next.
	m.npOn = false
	m.coverClearN = coverSendFrames
	for i := 0; i < 3; i++ {
		if !strings.Contains(m.View(), "\x1b_Ga=d") {
			t.Fatalf("render %d after closing dropped the delete", i)
		}
	}
	for i := 0; i < coverSendFrames; i++ {
		nm, _ := m.Update(tickMsg{})
		m = nm.(Model)
	}
	if strings.Contains(m.View(), "\x1b_Ga=d") {
		t.Error("still emitting the delete long after closing")
	}
}

// TestNewArtworkReplacesOld: a track change must delete the resident
// image and send the new one, or the first song's cover stays forever.
func TestNewArtworkReplacesOld(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	m := worstCaseModel(t, 150, 40)
	m.npOn = true

	nm, _ := m.Update(CoverLoadedMsg{URL: "https://example/second.jpg", Img: img})
	m = nm.(Model)
	if m.coverSendN <= 0 {
		t.Fatal("new artwork did not schedule a transmit")
	}
	if m.coverClearN <= 0 {
		t.Fatal("new artwork did not schedule a delete of the old image")
	}
	frame := m.View()
	if !strings.Contains(frame, "\x1b_Ga=d") || !strings.Contains(frame, "\x1b_Ga=t") {
		t.Error("frame should both drop the old image and send the new one")
	}
}

// TestLayoutGeometryDownloadsHidden: collapsing the downloads panel
// hands the whole right column to the queue, so the frame must still
// come out the exact height and the click boundary must follow.
func TestLayoutGeometryDownloadsHidden(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		for _, hidden := range []bool{false, true} {
			m := worstCaseModel(t, w, h)
			m.downloadsHidden = hidden
			label := "downloads shown"
			if hidden {
				label = "downloads hidden"
			}
			checkPanelGeometry(t, m, w, h, label)

			qh, dh := m.rightPanelSplit()
			if hidden {
				if dh != 0 {
					t.Errorf("%dx%d: hidden downloads still claim %d rows", w, h, dh)
				}
				if qh != m.panelHeight()-3 {
					t.Errorf("%dx%d: queue got %d content rows, want the full column (%d)", w, h, qh, m.panelHeight()-3)
				}
			} else if qh+dh != m.panelHeight()-6 {
				t.Errorf("%dx%d: split sums to %d, want %d", w, h, qh+dh, m.panelHeight()-6)
			}
		}
	}
}

// TestDownloadsPanelStartsHiddenAndReveals: the panel is hidden until
// there is something to show, and queueing a job brings it back — a
// download running behind a closed panel is invisible.
func TestDownloadsPanelStartsHiddenAndReveals(t *testing.T) {
	m := InitialModel()
	if !m.downloadsHidden {
		t.Error("downloads panel should start hidden")
	}
	m.revealDownloads()
	if m.downloadsHidden {
		t.Error("queueing a download should reveal the panel")
	}
	// Revealing is idempotent: a second job must not toggle it shut.
	m.revealDownloads()
	if m.downloadsHidden {
		t.Error("a second download must not hide the panel again")
	}
}

// TestNowPlayingOnEveryPage: the panel used to be wired to the stream
// page only. It belongs on every page that has a results list, and must
// never appear on settings, which draws its own layout.
func TestNowPlayingOnEveryPage(t *testing.T) {
	pages := []struct {
		page Page
		name string
		want bool
	}{
		{PageStream, "stream", true},
		{PageFavorites, "favorites", true},
		{PageLibrary, "library", true},
		{PageHistory, "history", true},
		{PageSettings, "settings", false},
	}
	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {90, 26}} {
		w, h := size[0], size[1]
		for _, p := range pages {
			m := worstCaseModel(t, w, h)
			m.switchPage(p.page)
			m.npOn = true // the user's toggle is on everywhere

			want := p.want && m.npFits() // a short terminal refuses it everywhere
			if got := m.npVisible(); got != want {
				t.Errorf("%dx%d %s: npVisible()=%v, want %v", w, h, p.name, got, want)
			}
			_, npH := m.leftPanelSplit()
			if want && npH == 0 {
				t.Errorf("%dx%d %s: panel is visible but got no rows", w, h, p.name)
			}
			if !want && npH != 0 {
				t.Errorf("%dx%d %s: panel claimed %d rows on a page that never shows it", w, h, p.name, npH)
			}
			checkPanelGeometry(t, m, w, h, "now-playing on "+p.name)
		}
	}
}

// TestSettingsPageClearsCoverImage: a kitty image outlives the frame
// that drew it, so stepping onto the page that cannot show the panel
// must carry the delete — otherwise the artwork stays painted over it.
func TestSettingsPageClearsCoverImage(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	m := worstCaseModel(t, 150, 40)
	m.npOn = true
	if !m.npVisible() {
		t.Fatal("panel should be visible before the page switch")
	}

	wasVisible := m.npVisible()
	m.switchPage(PageSettings)
	m.syncNowPlaying(wasVisible)

	if m.npVisible() {
		t.Error("panel must not be visible on the settings page")
	}
	if m.coverClearN <= 0 {
		t.Fatal("leaving the panel did not schedule the image delete")
	}
	if got := m.View(); !strings.Contains(got, coverart.KittyClear()) {
		t.Error("settings page does not carry the delete escape")
	}
}
