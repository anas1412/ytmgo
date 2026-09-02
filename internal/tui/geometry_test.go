package tui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"ytmgo/internal/coverart"
	"ytmgo/internal/lyrics"
	"ytmgo/internal/mpris"
	"ytmgo/internal/player"
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
	// The panel is on by default, and the resize below would make it
	// visible — which spawns a real cava process per model built. Tests
	// that want the panel set npOn themselves, after the resize.
	m.npOn = false

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
			if strings.Contains(line, "[space]") {
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

		// Album tracklist, with the strip's art in hand — the art path
		// must obey the same geometry as the text-only strip.
		alb := ytmusic.Album{Title: long, Artist: "Mild High Club"}
		m.openAlbum = &alb
		m.albumArtImg = image.NewRGBA(image.Rect(0, 0, 544, 544))
		m.albumArtURL = "https://example/album.jpg"
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
		// The prev icon is on the controls row in every mode — compact
		// or full, hints on or off — and nowhere else ("Playback
		// finished" on the title row is why matching "Play" failed).
		if strings.Contains(line, "⏮") {
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
		if strings.Contains(line, "[space]") {
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
		if got := resultsH + npH; got != m.panelHeight()-4 {
			t.Errorf("%dx%d: split sums to %d, want %d", w, h, got, m.panelHeight()-4)
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
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying
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

	// Stopping playback must carry the delete, repeatedly, or the image
	// stays on screen over whatever is drawn next.
	m.playerState = player.StateStopped
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
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying
	// The load must match the art the bar wants right now — stale
	// arrivals (an older fetch landing after a track change) are dropped.
	m.queue.UpdateTrack("sZxzPcT1Meg", func(tr *queue.Track) {
		tr.CoverURL = "https://example/second.jpg"
	})

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

// TestCoverFollowsTrackWithAlbumOpen: an open album preview must not pin
// the player bar's art. The bar follows the playing track — the open
// album's cover has its own slot in the album header strip — or a track
// change while browsing an album leaves the bar stuck on the album art.
func TestCoverFollowsTrackWithAlbumOpen(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.queue.SetCurrentIndex(0)
	m.queue.UpdateTrack("sZxzPcT1Meg", func(tr *queue.Track) {
		tr.CoverURL = "https://example/track.jpg"
	})
	m.albumCoverURL = "https://example/album.jpg"
	if got := m.desiredCoverURL(); got != "https://example/track.jpg" {
		t.Fatalf("desiredCoverURL = %q, want the playing track's art", got)
	}
}

// TestLayoutGeometryDownloadsPage: downloads render as a full page now,
// held to the same contract as every other page.
func TestLayoutGeometryDownloadsPage(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {120, 35}, {80, 24}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.switchPage(PageDownloads)
		checkPanelGeometry(t, m, w, h, "downloads page")
	}
}

// TestLayoutGeometryLyrics: the lyrics pane splits the right column the
// way the now-playing panel splits the left, and must obey the same
// contract — exact height, no over-wide line, controls row unmoved.
func TestLayoutGeometryLyrics(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {90, 26}, {80, 24}} {
		w, h := size[0], size[1]
		for _, on := range []bool{false, true} {
			m := worstCaseModel(t, w, h)
			m.lyricsOn = on
			m.lyricsTrackID = "sZxzPcT1Meg"
			m.lyricsSynced = true
			for i := 0; i < 40; i++ {
				m.lyricLines = append(m.lyricLines, lyrics.Line{
					Time: float64(i * 10),
					Text: "ラブ・ストーリーは突然に long lyric line to stress the width",
				})
			}
			label := "lyrics off"
			if on {
				label = "lyrics on"
			}
			checkPanelGeometry(t, m, w, h, label)

			qh, lh := m.rightPanelSplit()
			if !on || !m.lyricsFits() {
				if lh != 0 {
					t.Errorf("%dx%d %s: hidden lyrics still claim %d rows", w, h, label, lh)
				}
				if qh != m.panelHeight()-2 {
					t.Errorf("%dx%d %s: queue got %d rows, want the full column (%d)", w, h, label, qh, m.panelHeight()-2)
				}
			} else if qh+lh != m.panelHeight()-4 {
				t.Errorf("%dx%d %s: split sums to %d, want %d", w, h, label, qh+lh, m.panelHeight()-4)
			}
		}
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

// TestCoverFollowsPlayback: the art lives in the player bar, which is
// on every page — so the settings page shows it too, and what removes
// it is playback stopping, reconciled once per message in Update.
func TestCoverFollowsPlayback(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	m := worstCaseModel(t, 150, 40)
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying
	m.coverImg = image.NewRGBA(image.Rect(0, 0, 64, 64))
	m.coverURL = "https://example/a.jpg"
	m.coverSendN = coverSendFrames

	m.switchPage(PageSettings)
	if !strings.Contains(m.View(), "\x1b_Ga=p") {
		t.Error("the settings page should still place the cover — the player bar is there too")
	}

	// Stop playback through Update — the reconcile compares the model
	// either side of the dispatch, so the change must happen inside it.
	if !m.coverOnScreen() {
		t.Fatal("cover should be on screen before the stop")
	}
	nm, _ := m.Update(MprisCmdMsg{Cmd: mpris.CmdStop})
	m = nm.(Model)
	if m.coverClearN <= 0 {
		t.Fatal("stopping playback did not schedule the image delete")
	}
	if !strings.Contains(m.View(), coverart.KittyClear()) {
		t.Error("the frame after stopping does not carry the delete escape")
	}
}

// TestPlayerRowZonesInsideRow: every click zone the mouse reads must
// fall inside the row the view renders, in both density tiers. The
// controls row holds transport and modes; the seek bar has a line of
// its own.
func TestPlayerRowZonesInsideRow(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {100, 28}, {80, 24}} {
		w, h := size[0], size[1]
		m := worstCaseModel(t, w, h)
		m.playerState = player.StatePlaying
		m.duration = 200
		m.position = 60
		l := m.playerRowLayout()

		innerW := w - 6
		if m.playerCoverSlot() {
			innerW -= playerCoverCols + 2
		}
		if got := lipgloss.Width(l.controlsRow); got > innerW {
			t.Errorf("%dx%d: controls row is %d cells, inner width is %d", w, h, got, innerW)
		}
		if got := lipgloss.Width(l.progressRow); got > innerW {
			t.Errorf("%dx%d: progress row is %d cells, inner width is %d", w, h, got, innerW)
		}
		end := 3 + (w - 6)
		for _, z := range []struct {
			name string
			x    int
		}{
			{"prevEnd", l.prevEnd}, {"playEnd", l.playEnd}, {"transportEnd", l.transportEnd},
			{"barStart", l.barStart}, {"barEnd", l.barStart + l.barWidth},
			{"rightStart", l.rightStart}, {"volEnd", l.volEnd},
		} {
			if z.x < 3 || z.x > end {
				t.Errorf("%dx%d: zone %s at x=%d outside content [3,%d]", w, h, z.name, z.x, end)
			}
		}
		// Zones on the controls row must be ordered and non-overlapping;
		// the bar lives on its own row and only needs to fit.
		if !(l.prevEnd <= l.playEnd && l.playEnd <= l.transportEnd &&
			l.transportEnd <= l.rightStart &&
			l.rightStart <= l.shuffleEnd && l.shuffleEnd <= l.repeatStart &&
			l.repeatEnd <= l.volStart && l.volStart < l.volEnd) {
			t.Errorf("%dx%d: zones out of order: %+v", w, h, l)
		}
	}
}

// TestAlbumArtFollowsTheAlbum: the strip's kitty image is owed a delete
// when the album closes — otherwise it stays painted over the results
// list that replaces the tracklist.
func TestAlbumArtFollowsTheAlbum(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	m := worstCaseModel(t, 150, 40)
	alb := ytmusic.Album{Title: "Skiptracing", Artist: "Mild High Club"}
	m.openAlbum = &alb
	m.albumTracks = m.results[:5] // a strip needs a tracklist under it
	m.albumArtImg = image.NewRGBA(image.Rect(0, 0, 64, 64))
	m.albumArtURL = "https://example/album.jpg"
	m.albumArtSendN = coverSendFrames

	if !m.albumArtOnScreen() {
		t.Fatal("album art should be on screen with the album open")
	}
	if !strings.Contains(m.View(), "i=1338") {
		t.Error("the open album's frame carries no album-art escape")
	}

	// Close the album through Update so the reconcile sees it.
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = nm.(Model)
	if m.openAlbum != nil {
		t.Fatal("esc did not close the album")
	}
	if m.albumArtClearN <= 0 {
		t.Fatal("closing the album did not schedule the art delete")
	}
	if !strings.Contains(m.View(), coverart.KittyClearID(coverart.AlbumImageID)) {
		t.Error("the frame after closing does not carry the album-art delete")
	}
}

// TestAlbumArtAppearsOnLoad: the frame after the art arrives must place
// the image and must NOT delete it. The delete escape rides the player
// bar, which renders after the strip — so a delete scheduled alongside
// the transmit removed the image the same frame drew, and the art only
// appeared after something re-sent it (a page switch and back).
func TestAlbumArtAppearsOnLoad(t *testing.T) {
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "1")

	m := worstCaseModel(t, 150, 40)
	alb := ytmusic.Album{Title: "Homage", Artist: "Tesla"}
	m.openAlbum = &alb
	m.albumTracks = m.results[:5]

	nm, _ := m.Update(AlbumArtLoadedMsg{
		URL: "https://example/album.jpg",
		Img: image.NewRGBA(image.Rect(0, 0, 64, 64)),
		Seq: m.albumSeq,
	})
	m = nm.(Model)

	frame := m.View()
	if !strings.Contains(frame, "i=1338") {
		t.Fatal("the frame after the art arrived carries no album-art escape")
	}
	if strings.Contains(frame, coverart.KittyClearID(coverart.AlbumImageID)) {
		t.Error("the same frame deletes the image it just placed")
	}
}

// TestAlbumClickMatchesRow: the header strip shifts the tracklist down
// by its rows, and the click math has to shift with it — without the
// correction every click in an open album landed a row and a half off.
func TestAlbumClickMatchesRow(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	alb := ytmusic.Album{Title: "Homage", Artist: "Tesla"}
	m.openAlbum = &alb
	m.albumTracks = m.results[:10]
	m.searchCursor, m.searchOffset = 0, 0

	// Find where each numbered row actually renders, then click it.
	lines := strings.Split(m.View(), "\n")
	for want := 0; want < 3; want++ {
		marker := fmt.Sprintf("%02d. ", want+1)
		row := -1
		for i, l := range lines {
			if strings.Contains(l, marker) {
				row = i
				break
			}
		}
		if row < 0 {
			t.Fatalf("track %s not on screen", marker)
		}
		nm, _ := m.handleClick(10, row)
		if nm.searchCursor != want {
			t.Errorf("click on rendered row of track %d selected %d", want+1, nm.searchCursor)
		}
	}
}

// TestVolumeBarClickIsExact: clicking cell i of the volume bar must
// fill the bar exactly through cell i. The old zone ran through the
// percentage text after the bar, so its divisor was wider than the bar
// itself and the mapping drifted rightward.
func TestVolumeBarClickIsExact(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying
	m.duration = 200
	l := m.playerRowLayout()
	if l.volBarCells == 0 {
		t.Fatal("full-width layout should carry a volume bar")
	}
	for cell := 0; cell < l.volBarCells; cell++ {
		nm, _ := m.handlePlayerRowClick(l.volBarStart+cell, false)
		// The handler rounds up so the clicked cell crosses the fill
		// threshold — 12.5 truncated to 12 would leave it unlit.
		want := ((cell+1)*100 + l.volBarCells - 1) / l.volBarCells
		if nm.volume != want {
			t.Errorf("click on cell %d set volume %d, want %d", cell, nm.volume, want)
		}
		// And the rendered bar must show exactly cell+1 filled cells.
		filled := int(float64(nm.volume) / 100.0 * float64(l.volBarCells))
		if filled != cell+1 {
			t.Errorf("cell %d: volume %d renders %d filled cells, want %d", cell, nm.volume, filled, cell+1)
		}
	}
	// One past the bar is the percentage text: it must do nothing.
	nm, _ := m.handlePlayerRowClick(l.volBarEnd+1, false)
	if nm.volume != m.volume {
		t.Errorf("click on the percentage text changed the volume")
	}
}

// TestDownloadJumpsToDownloadsPage: an explicit x lands the user on the
// downloads page so the job it started is in front of them.
func TestDownloadJumpsToDownloadsPage(t *testing.T) {
	// An empty PATH keeps the enqueued job from launching a real
	// yt-dlp: the worker's exec fails instantly and the job just sits
	// there failed, which is all this test needs.
	t.Setenv("PATH", t.TempDir())

	m := worstCaseModel(t, 150, 40)
	// A URL in hand means x enqueues directly instead of resolving
	// first — the async half is covered by the resolved-URL handler.
	m.results[0].URL = "https://music.youtube.com/watch?v=m9SMT5ipbxk"
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = nm.(Model)
	if m.activePage != PageDownloads {
		t.Errorf("x left the user on page %v, want the downloads page", m.activePage)
	}
	if m.downloader == nil || len(m.downloader.Jobs()) == 0 {
		t.Fatal("x queued nothing")
	}
	m.downloader.Close()
	// Wait for the worker to finish failing the job, so its mkdir
	// cannot race the temp dir's cleanup.
	for i := 0; i < 100; i++ {
		if st := m.downloader.Jobs()[0].Status; st != 0 && m.downloader.Jobs()[0].Progress == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestHintsToggleKeepsGeometry: hiding the hints shortens titles and
// player clusters; the frame must stay exact and every player zone must
// stay inside its row, in both states.
func TestHintsToggleKeepsGeometry(t *testing.T) {
	for _, size := range [][2]int{{200, 50}, {150, 40}, {120, 35}, {80, 24}} {
		w, h := size[0], size[1]
		for _, show := range []bool{true, false} {
			m := worstCaseModel(t, w, h)
			m.settings.ShowHints = show
			m.queue.SetCurrentIndex(0)
			m.playerState = player.StatePlaying
			m.duration = 200
			label := "hints on"
			if !show {
				label = "hints off"
			}
			checkPanelGeometry(t, m, w, h, label)

			l := m.playerRowLayout()
			end := 3 + (w - 6)
			for _, z := range [][2]int{{l.prevEnd, l.playEnd}, {l.playEnd, l.transportEnd},
				{l.barStart, l.barStart + l.barWidth}, {l.rightStart, l.volEnd}} {
				if z[0] < 3 || z[1] > end || z[0] > z[1] {
					t.Errorf("%dx%d %s: zone [%d,%d] outside content [3,%d]", w, h, label, z[0], z[1], end)
				}
			}
		}
	}
}
