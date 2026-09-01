package tui

import (
	"fmt"
	"strings"
	"time"

	"ytmgo/internal/player"

	"github.com/charmbracelet/lipgloss"
)

// playerRowLayout is the player bar's controls-and-progress geometry.
// The view renders the two rows and the mouse reads the zone offsets;
// both come from the same builder so they cannot disagree — the old
// separate rows kept two copies of this arithmetic in sync by hand.
type playerRowLayout struct {
	// controlsRow is transport on the left, modes and volume flush
	// right. progressRow is the full-width seek line beneath it: time,
	// bar with a playhead, time.
	controlsRow, progressRow string

	// Transport zones: absolute terminal x, exclusive ends.
	transportStart, prevEnd, playEnd, transportEnd int

	// Seek bar (on progressRow).
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
// left — sized for four rows of square art. Fixed, not fitted to the
// artwork, so the click zones and the text never shift when the art
// changes shape or hasn't arrived yet.
const playerCoverCols = 10

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
	shuffleTxt := "⇄ SHFL"
	if l.compact {
		shuffleTxt = "⇄"
	}
	shuffleLabel := hint("[s]") + shuffleStyle.Render(shuffleTxt)

	var repeatTxt string
	var repeatOn bool
	switch {
	case m.queue.IsRepeat():
		repeatTxt, repeatOn = "↻ ONE", true
	case m.queue.IsRepeatAll():
		repeatTxt, repeatOn = "↻ ALL", true
	default:
		repeatTxt, repeatOn = "↻ OFF", false
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

	// ── Controls row: transport left, modes and volume flush right ──
	transportW := l.transportEnd - contentStartX
	l.rightStart = contentStartX + innerW - rightW
	gap := innerW - transportW - rightW
	if gap < 2 {
		gap = 2
		l.rightStart = contentStartX + transportW + gap
	}
	l.controlsRow = transport + strings.Repeat(" ", gap) + right

	// ── Progress row: its own full-width line ──
	// [h] MM:SS ▮▮▮●┄┄┄┄ MM:SS [l] — a playhead where the fill ends and
	// a dotted track for what is left.
	cur, tot := "--:--", "--:--"
	if m.duration > 0 && m.playerState != player.StateStopped {
		cur = formatTime(m.displayPosition())
		tot = formatTime(m.duration)
		if len(cur) < len(tot) {
			cur = strings.Repeat(" ", len(tot)-len(cur)) + cur
		}
	}
	hPart, lPart := "", ""
	if m.settings.ShowHints {
		hPart = styleKeyHint.Render("[h]") + " "
		lPart = " " + styleKeyHint.Render("[l]")
	}
	l.barWidth = innerW - lipgloss.Width(hPart) - lipgloss.Width(lPart) - len(cur) - len(tot) - 2
	if l.barWidth < 5 {
		l.barWidth = 5
	}
	l.barStart = contentStartX + lipgloss.Width(hPart) + len(cur) + 1

	displayPct := 0.0
	if m.duration > 0 {
		displayPct = (m.displayPosition() / m.duration) * 100.0
	}
	l.progressRow = hPart + styleTime.Render(cur) + " " +
		renderSeekBar(displayPct, l.barWidth) + " " +
		styleTime.Render(tot) + lPart

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
