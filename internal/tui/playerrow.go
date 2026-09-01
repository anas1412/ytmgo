package tui

import (
	"fmt"
	"strings"
	"time"

	"ytmgo/internal/player"

	"github.com/charmbracelet/lipgloss"
)

// playerRowLayout is the combined progress-and-controls row of the
// player bar, together with its click geometry. The view renders .row
// and the mouse reads the zone offsets; both come from the same builder
// so they cannot disagree — the old separate rows kept two copies of
// this arithmetic in sync by hand.
type playerRowLayout struct {
	row string

	// Transport zones: absolute terminal x, exclusive ends.
	transportStart, prevEnd, playEnd, transportEnd int

	// Seek bar.
	barStart, barWidth int

	// Right cluster: modes and volume. volBarStart/volBarEnd bound the
	// volume bar's actual cells — not the percentage text after it.
	rightStart, shuffleEnd, repeatStart, repeatEnd int
	volStart, volDownEnd, volUpStart, volEnd       int
	volBarStart, volBarEnd, volBarCells            int

	// compact drops the button words, the [h]/[l] hints and the volume
	// bar so the row still fits a narrow terminal.
	compact bool
}

// playerCoverCols is the width of the cover slot on the player bar's
// left. Fixed, not fitted to the artwork, so the click zones and the
// text never shift when the art changes shape or hasn't arrived yet.
const playerCoverCols = 8

// playerCoverSlot reports whether the player bar reserves its cover
// column: whenever a current track exists, playing or paused, so the
// bar does not reflow on pause or while the art loads.
func (m Model) playerCoverSlot() bool {
	idx := m.queue.CurrentIndex()
	return m.queue.Len() > 0 && idx >= 0 && idx < m.queue.Len() &&
		m.playerState != player.StateStopped
}

// playerRowLayout builds the combined row for the current model state.
func (m Model) playerRowLayout() playerRowLayout {
	contentStartX := 3 // double border (1) + left padding (2)
	innerW := m.width - 6
	if m.playerCoverSlot() {
		// The cover column and its gap sit left of every content row.
		contentStartX += playerCoverCols + 2
		innerW -= playerCoverCols + 2
	}
	l := playerRowLayout{compact: innerW < 110, transportStart: contentStartX}

	// ── Transport cluster ──
	// With hints hidden the buttons keep their words and stay clickable;
	// only the bracketed key tokens go. The zones derive from whatever
	// is rendered, so the mouse follows automatically.
	hint := func(k string) string {
		if !m.settings.ShowHints {
			return ""
		}
		return styleKeyHint.Render(k) + " "
	}
	pHint := hint("[p]")
	spaceHint := hint("[space]")
	nHint := hint("[n]")
	playing := m.playerState == player.StatePlaying
	prevTxt, nextTxt, playTxt := "⏮ Prev", "⏭ Next", "▶ Play"
	if playing {
		playTxt = "⏸ Pause"
	}
	if l.compact {
		prevTxt, nextTxt, playTxt = "⏮", "⏭", "▶"
		if playing {
			playTxt = "⏸"
		}
	}
	playStyle := styleCtrlBtn
	if playing {
		playStyle = styleCtrlBtnActive
	}
	prevGroup := pHint + styleCtrlBtn.Render(prevTxt)
	playGroup := spaceHint + playStyle.Render(playTxt)
	nextGroup := nHint + styleCtrlBtn.Render(nextTxt)
	l.prevEnd = contentStartX + lipgloss.Width(prevGroup)
	l.playEnd = l.prevEnd + 2 + lipgloss.Width(playGroup)
	l.transportEnd = l.playEnd + 2 + lipgloss.Width(nextGroup)
	transport := prevGroup + "  " + playGroup + "  " + nextGroup

	// ── Right cluster ──
	flashActive := time.Now().Before(m.modeFlashUntil)
	shuffleStyle := styleModeInactive
	if flashActive && m.modeFlashTarget == "shuffle" {
		shuffleStyle = styleModeFlash
	} else if m.queue.IsShuffle() {
		shuffleStyle = styleModeActive
	}
	shuffleTxt := "🔀 SHFL"
	if l.compact {
		shuffleTxt = "🔀"
	}
	shuffleLabel := hint("[s]") + shuffleStyle.Render(shuffleTxt)

	var repeatTxt string
	var repeatOn bool
	switch {
	case m.queue.IsRepeat():
		repeatTxt, repeatOn = "🔁 ONE", true
	case m.queue.IsRepeatAll():
		repeatTxt, repeatOn = "🔁 ALL", true
	default:
		repeatTxt, repeatOn = "🔁 OFF", false
	}
	repeatStyle := styleModeInactive
	if flashActive && m.modeFlashTarget == "repeat" {
		repeatStyle = styleModeFlash
	} else if repeatOn {
		repeatStyle = styleModeActive
	}
	repeatLabel := hint("[r]") + repeatStyle.Render(repeatTxt)

	volDown := styleKeyHint.Render("[-]")
	volUp := styleKeyHint.Render("[+]")
	if !m.settings.ShowHints {
		// The volume steppers stay as click targets, just quieter.
		volDown = styleKeyHint.Render("−")
		volUp = styleKeyHint.Render("+")
	}
	volMid := fmt.Sprintf("%d%%", m.volume)
	if !l.compact {
		l.volBarCells = 8
		volMid = renderVolumeBar(m.volume, l.volBarCells) + " " + volMid
	}
	volLabel := volDown + " " + volMid + " " + volUp
	right := shuffleLabel + "  " + repeatLabel + "  " + volLabel
	rightW := lipgloss.Width(right)

	// ── Time and seek bar fill whatever is left ──
	timeInfo := ""
	// Below this the compact clusters alone fill the row (the cover
	// slot costs ten columns on top of a narrow terminal); the time is
	// the least load-bearing part — the bar still shows the position.
	if m.duration > 0 && m.playerState != player.StateStopped && innerW >= 72 {
		displayPos := m.displayPosition()
		cur := formatTime(displayPos)
		tot := formatTime(m.duration)
		if len(cur) < len(tot) {
			cur = strings.Repeat(" ", len(tot)-len(cur)) + cur
		}
		sep := " / "
		if l.compact {
			sep = "/" // every cell counts at 80 columns
		}
		timeInfo = cur + sep + tot
	}
	hPart, lPart := "", ""
	if !l.compact && m.settings.ShowHints {
		hPart = styleKeyHint.Render("[h]") + " "
		lPart = " " + styleKeyHint.Render("[l]")
	}
	transportW := l.transportEnd - contentStartX
	fixed := transportW + 2 + lipgloss.Width(hPart) + lipgloss.Width(lPart) + rightW + 2
	if timeInfo != "" {
		fixed += lipgloss.Width(timeInfo) + 1
	}
	// The bar takes what the clusters leave. It also absorbs any
	// overflow: everything else on the row is fixed-width, so a row
	// wider than the box could only push the right cluster's click
	// zones past the border.
	l.barWidth = innerW - fixed
	if l.barWidth < 3 {
		l.barWidth = 3
	}
	l.barStart = l.transportEnd + 2 + lipgloss.Width(hPart)

	displayPct := 0.0
	if m.duration > 0 {
		displayPct = (m.displayPosition() / m.duration) * 100.0
	}
	bar := renderProgressBar(displayPct, l.barWidth)

	mid := hPart + bar
	if timeInfo != "" {
		mid += " " + styleTime.Render(timeInfo)
	}
	mid += lPart

	// Right cluster flush against the right edge.
	l.rightStart = contentStartX + innerW - rightW
	used := transportW + 2 + lipgloss.Width(mid)
	spacer := innerW - used - rightW
	if spacer < 1 {
		spacer = 1
		l.rightStart = contentStartX + used + 1
	}
	l.row = transport + "  " + mid + strings.Repeat(" ", spacer) + right

	l.shuffleEnd = l.rightStart + lipgloss.Width(shuffleLabel)
	l.repeatStart = l.shuffleEnd + 2
	l.repeatEnd = l.repeatStart + lipgloss.Width(repeatLabel)
	l.volStart = l.repeatEnd + 2
	l.volEnd = l.volStart + lipgloss.Width(volLabel)
	l.volDownEnd = l.volStart + lipgloss.Width(volDown)
	l.volUpStart = l.volEnd - lipgloss.Width(volUp)
	if l.volBarCells > 0 {
		l.volBarStart = l.volDownEnd + 1 // after "[-] "
		l.volBarEnd = l.volBarStart + l.volBarCells
	}
	return l
}
