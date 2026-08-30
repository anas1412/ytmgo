package tui

import "github.com/charmbracelet/lipgloss"

// Theme decides which palette the UI draws with.
//
// auto and the two forced modes share one adaptive palette: every colour
// in it carries a light and a dark value, and lipgloss picks between
// them from the terminal's background. terminal is a different palette
// altogether — the terminal's own ANSI slots, so ytmgo takes on whatever
// colour scheme the terminal is already set to.
type Theme string

const (
	ThemeAuto     Theme = "auto"
	ThemeDark     Theme = "dark"
	ThemeLight    Theme = "light"
	ThemeTerminal Theme = "terminal"
)

var themeOrder = []Theme{ThemeAuto, ThemeDark, ThemeLight, ThemeTerminal}

// ParseTheme maps a stored setting back to a Theme, falling back to auto
// for anything unrecognised (an older config, or a hand-edited one).
func ParseTheme(s string) Theme {
	for _, t := range themeOrder {
		if string(t) == s {
			return t
		}
	}
	return ThemeAuto
}

// detectedDark is the terminal's own background, sampled once before any
// forced mode overwrites it, so switching back to auto can restore it
// rather than keeping whatever was last forced.
var detectedDark = true

// palette is one complete set of UI colours.
type palette struct {
	bg, bgPanel, bgSurface, bgHover lipgloss.TerminalColor
	border, borderFoc               lipgloss.TerminalColor
	accent, accent2, accent3        lipgloss.TerminalColor
	text, textDim, textMid, title   lipgloss.TerminalColor
	playing, download, done         lipgloss.TerminalColor
	errColor, warning, header       lipgloss.TerminalColor
	barFill, barEmpty               lipgloss.TerminalColor
}

// adaptive keeps ytmgo's own violet-and-mint identity, with a second set
// of values for light terminals. The light values are not the dark ones
// lightened: on white, the neons wash out, so each one is darkened until
// it carries enough contrast against a pale background.
func adaptive() palette {
	c := func(light, dark string) lipgloss.TerminalColor {
		return lipgloss.AdaptiveColor{Light: light, Dark: dark}
	}
	return palette{
		bg:        c("#fafafa", "#0d0d0d"),
		bgPanel:   c("#f4f4f6", "#111111"),
		bgSurface: c("#f7f7fa", "#0d0d12"),
		bgHover:   c("#e8e6f2", "#1a1a2e"),
		border:    c("#c9c7d4", "#2a2a3e"),
		borderFoc: c("#6d28d9", "#7c3aed"),
		accent:    c("#6d28d9", "#7c3aed"),
		accent2:   c("#047857", "#06d6a0"),
		accent3:   c("#be123c", "#f72585"),
		text:      c("#1c1c22", "#e0e0e0"),
		textDim:   c("#8a8794", "#555566"),
		textMid:   c("#5a5766", "#9999aa"),
		title:     c("#000000", "#ffffff"),
		playing:   c("#047857", "#06d6a0"),
		download:  c("#b45309", "#f4a261"),
		done:      c("#047857", "#06d6a0"),
		errColor:  c("#be123c", "#f72585"),
		warning:   c("#b45309", "#f4a261"),
		header:    c("#6d28d9", "#7c3aed"),
		barFill:   c("#6d28d9", "#7c3aed"),
		barEmpty:  c("#d4d2dd", "#2a2a3e"),
	}
}

// terminalScheme draws from the ANSI slots the terminal fills in from
// whatever scheme it is running, so ytmgo matches gruvbox, nord,
// solarized and the rest without knowing anything about them. Slots 0
// and 7/15 are the scheme's own background and foreground, which is why
// this reads correctly on light and dark schemes alike without a second
// set of values.
func terminalScheme() palette {
	a := func(n string) lipgloss.TerminalColor { return lipgloss.Color(n) }
	return palette{
		bg:        a("0"),
		bgPanel:   a("0"),
		bgSurface: a("0"),
		bgHover:   a("8"),
		border:    a("8"),
		borderFoc: a("5"), // magenta = focused
		accent:    a("5"),
		accent2:   a("6"), // cyan = active
		accent3:   a("1"), // red = destructive
		text:      a("7"),
		textDim:   a("8"),
		textMid:   a("7"),
		title:     a("15"),
		playing:   a("6"),
		download:  a("3"), // yellow = in progress
		done:      a("2"), // green = complete
		errColor:  a("1"),
		warning:   a("3"),
		header:    a("5"),
		barFill:   a("5"),
		barEmpty:  a("8"),
	}
}

// setPalette copies a palette into the colour variables the styles are
// built from.
func setPalette(p palette) {
	colorBg, colorBgPanel, colorBgSurface, colorBgHover = p.bg, p.bgPanel, p.bgSurface, p.bgHover
	colorBorder, colorBorderFoc = p.border, p.borderFoc
	colorAccent, colorAccent2, colorAccent3 = p.accent, p.accent2, p.accent3
	colorText, colorTextDim, colorTextMid, colorTitle = p.text, p.textDim, p.textMid, p.title
	colorPlaying, colorDownload, colorDone = p.playing, p.download, p.done
	colorError, colorWarning, colorHeader = p.errColor, p.warning, p.header
	colorBarFill, colorBarEmpty = p.barFill, p.barEmpty
}

// ApplyTheme installs a theme and rebuilds every style against it.
func ApplyTheme(t Theme) {
	switch t {
	case ThemeTerminal:
		setPalette(terminalScheme())
	case ThemeDark:
		lipgloss.SetHasDarkBackground(true)
		setPalette(adaptive())
	case ThemeLight:
		lipgloss.SetHasDarkBackground(false)
		setPalette(adaptive())
	default: // auto
		lipgloss.SetHasDarkBackground(detectedDark)
		setPalette(adaptive())
	}
	buildStyles()
}

func init() {
	// Sample the terminal before anything can force a mode, then build
	// once so the package is usable even if no theme is applied later.
	detectedDark = lipgloss.HasDarkBackground()
	ApplyTheme(ThemeAuto)
}
