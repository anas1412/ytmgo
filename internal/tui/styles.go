package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Colour Palette ─────────────────────────────────────────────────
// Assigned by setPalette from the active theme, so every style below is
// rebuilt against whichever palette is in force. Declared as
// TerminalColor because a palette may hold adaptive light/dark values
// or plain ANSI indices.
var (
	colorBg, colorBgPanel, colorBgSurface     lipgloss.TerminalColor
	colorBgHover, colorBorder, colorBorderFoc lipgloss.TerminalColor
	colorAccent, colorAccent2, colorAccent3   lipgloss.TerminalColor
	colorText, colorTextDim, colorTextMid     lipgloss.TerminalColor
	colorPlaying, colorDownload, colorDone    lipgloss.TerminalColor
	colorError, colorWarning, colorTitle      lipgloss.TerminalColor
	colorHeader, colorBarFill, colorBarEmpty  lipgloss.TerminalColor
)

// Progress bar characters
var (
	barCharFull  = "█"
	barCharEmpty = "░"
)

// ─── Styles ─────────────────────────────────────────────────────────
var (
	panelBorder, panelBorderFocused, styleHeader                    lipgloss.Style
	styleLogo, styleVersion, styleUpdateAvail                       lipgloss.Style
	stylePlayerBox, stylePlayerBoxStopped, styleNowTitle            lipgloss.Style
	styleNowIndicator, styleTime, styleCtrlBtn                      lipgloss.Style
	styleCtrlBtnActive, styleModeActive, styleModeInactive          lipgloss.Style
	styleModeFlash, styleVolumeLabel, styleCtrlSep                  lipgloss.Style
	styleDownloadLabel, styleDoneLabel, styleErrorLabel             lipgloss.Style
	stylePanelTitle, styleNavTab, styleNavTabActive                 lipgloss.Style
	styleSettingsLabel, styleSettingsValue, styleSettingsDesc       lipgloss.Style
	styleSettingsBoolOn, styleSettingsBoolOff, styleSettingsOpenBtn lipgloss.Style
	styleHelp, styleHelpKey, styleKeyHint                           lipgloss.Style
	styleStatus, styleStatusIdle, styleStatusErr                    lipgloss.Style
	styleVizLow, styleVizMid, styleVizHigh                          lipgloss.Style
)

// buildStyles constructs every style from the colours currently in the
// palette. Lipgloss resolves a style's colours when the style is built,
// not when it renders, so a palette swap only takes effect once this has
// run again.
func buildStyles() {
	// ─── Color Palette ──────────────────────────────────────────────────
	// Industrial cyberpunk: deep backdrop, neon accents with semantic weight.
	// Progress bar characters

	// ─── Panel Borders ──────────────────────────────────────────────────

	panelBorder = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder)

	panelBorderFocused = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFoc)

		// ─── Header ─────────────────────────────────────────────────────────

	styleHeader = lipgloss.NewStyle().
		Foreground(colorHeader).
		Bold(true).
		PaddingLeft(1)

	styleLogo = lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true)

	styleVersion = lipgloss.NewStyle().
		Foreground(colorTextDim)

	styleUpdateAvail = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true)

		// ─── Player Bar ─────────────────────────────────────────────────────

	// The player's identity is its double border, not a bright colour:
	// a mint glow while playing competed with the focused panel's accent
	// border, leaving two things claiming attention. Playing shows in
	// the content — the seek line, the ▶ — while the border stays quiet.
	stylePlayerBox = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colorBorder).
		Padding(0, 2, 0, 2)

	stylePlayerBoxStopped = lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colorTextDim).
		Padding(0, 2, 0, 2)

	styleNowTitle = lipgloss.NewStyle().
		Foreground(colorTitle).
		Bold(true)

	styleNowIndicator = lipgloss.NewStyle().
		Foreground(colorPlaying)

	styleTime = lipgloss.NewStyle().
		Foreground(colorTextDim)

	styleCtrlBtn = lipgloss.NewStyle().
		Foreground(colorTextMid).
		Bold(true)

	styleCtrlBtnActive = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true)

	// Mode indicators
	styleModeActive = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true)

	styleModeInactive = lipgloss.NewStyle().
		Foreground(colorTextDim)

	// Mode flash — used for ~250ms after the user presses `s` or `r`,
	// so the SHFL / REPT label briefly pops to confirm the keypress
	// in the bar itself (not only via the status row).
	styleModeFlash = lipgloss.NewStyle().
		Foreground(colorTitle).
		Bold(true)

	// Volume bar
	styleVolumeLabel = lipgloss.NewStyle().
		Foreground(colorTextDim)

	// Separator for controls
	styleCtrlSep = lipgloss.NewStyle().
		Foreground(colorTextDim)

		// ─── Download Bar ───────────────────────────────────────────────────

	styleDownloadLabel = lipgloss.NewStyle().
		Foreground(colorDownload).
		Bold(true)

	styleDoneLabel = lipgloss.NewStyle().
		Foreground(colorDone).
		Bold(true)

	styleErrorLabel = lipgloss.NewStyle().
		Foreground(colorError).
		Bold(true)

		// ─── Panel Titles ───────────────────────────────────────────────────

	// No padding: the title rides inside a top border now, and boxTitled
	// supplies the spacing around it.
	stylePanelTitle = lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true)

		// ─── Page Navigation Tabs ───────────────────────────────────────────

	styleNavTab = lipgloss.NewStyle().
		Foreground(colorTextDim).
		PaddingLeft(1).
		PaddingRight(1)

	// Underline as well as bold and colour: on themes where the accent
	// is close to the text colour, bold alone did not mark the tab.
	styleNavTabActive = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true).
		Underline(true).
		PaddingLeft(1).
		PaddingRight(1)

		// ─── Settings ───────────────────────────────────────────────────────

	styleSettingsLabel = lipgloss.NewStyle().
		Foreground(colorAccent).
		Bold(true).
		PaddingLeft(1)

	styleSettingsValue = lipgloss.NewStyle().
		Foreground(colorText).
		PaddingLeft(1)

	styleSettingsDesc = lipgloss.NewStyle().
		Foreground(colorTextDim).
		Italic(true).
		PaddingLeft(3)

	styleSettingsBoolOn = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true)

	styleSettingsBoolOff = lipgloss.NewStyle().
		Foreground(colorTextDim)

	styleSettingsOpenBtn = lipgloss.NewStyle().
		Foreground(colorDownload).
		Bold(true)

		// ─── Help Bar ───────────────────────────────────────────────────────

	styleHelp = lipgloss.NewStyle().
		Foreground(colorTextDim).
		PaddingLeft(1)

	styleHelpKey = lipgloss.NewStyle().
		Foreground(colorAccent2)

		// ─── Inline Key Hints ───────────────────────────────────────────────
		//
		// Rendered directly next to the UI element a key controls (play button,
		// mode indicators, panel titles, header) instead of only in the help
		// bar. Uses the same accent as styleHelpKey for visual consistency.

	styleKeyHint = lipgloss.NewStyle().
		Foreground(colorAccent2).
		Bold(true)

		// ─── Status ─────────────────────────────────────────────────────────

	styleStatus = lipgloss.NewStyle().
		Foreground(colorAccent2).
		PaddingLeft(1)

	// Idle tip: dimmer than action status so the eye can tell them apart.
	styleStatusIdle = lipgloss.NewStyle().
		Foreground(colorTextDim).
		PaddingLeft(1).
		Italic(true)

	styleStatusErr = lipgloss.NewStyle().
		Foreground(colorError).
		PaddingLeft(1)

		// ─── Progress / Volume Bars ─────────────────────────────────────────

		// renderProgressBar draws a proportional bar using solid fill blocks.

		// renderVolumeBar draws a block-style volume indicator.

		// ─── Visualizer ─────────────────────────────────────────────────────
		// Bars run cool at the base to hot at the peak.

	styleVizLow = lipgloss.NewStyle().Foreground(colorAccent)
	styleVizMid = lipgloss.NewStyle().Foreground(colorAccent2)
	styleVizHigh = lipgloss.NewStyle().Foreground(colorAccent3)

	buildInlineStyles()
}

// renderSeekBar is the player's own progress line: solid fill, a
// playhead where the fill ends, and a dotted dim track for the rest —
// so the current position reads as a point, not just an edge.
func renderSeekBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct / 100.0 * float64(width))
	if filled >= width {
		filled = width - 1
	}
	if filled < 0 {
		filled = 0
	}
	var bar strings.Builder
	bar.WriteString(lipgloss.NewStyle().Foreground(colorBarFill).Render(strings.Repeat("━", filled)))
	bar.WriteString(lipgloss.NewStyle().Foreground(colorAccent2).Bold(true).Render("●"))
	bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render(strings.Repeat("┄", width-filled-1)))
	return bar.String()
}

func renderProgressBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct / 100.0 * float64(width))
	if filled > width {
		filled = width
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarFill).Render(barCharFull))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render(barCharEmpty))
		}
	}
	return bar.String()
}
func renderVolumeBar(vol int, width int) string {
	if width <= 0 {
		return ""
	}
	// Use finer granularity: each block = 2 units for better precision
	blocks := float64(width)
	filled := int(float64(vol) / 100.0 * blocks)
	if filled > width {
		filled = width
	}

	var bar strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			bar.WriteString(lipgloss.NewStyle().Foreground(colorAccent2).Render("█"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render("░"))
		}
	}
	return bar.String()
}
