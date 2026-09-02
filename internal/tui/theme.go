package tui

import "github.com/charmbracelet/lipgloss"

// Theme names the palette the UI draws with.
//
// ytmgo and terminal both take their cue from the terminal and paint no
// background of their own, so they inherit whatever it has —
// transparency and background images included. ytmgo is the app's own
// colours, picking light or dark values from the terminal's background;
// terminal borrows the terminal's ANSI slots outright. The named schemes
// below are complete colour schemes and do paint their background, which
// is what lets a dark scheme stay readable on a light terminal and the
// other way round.
type Theme string

const (
	ThemeYtmgo    Theme = "ytmgo"
	ThemeTerminal Theme = "terminal"
)

// scheme is one colour scheme, written in the roles the UI needs rather
// than in the sixteen ANSI slots, so each entry says what it means.
type scheme struct {
	name    Theme
	bg      string // window background
	panel   string // panel and surface fill
	hover   string // selection and input fill
	border  string
	text    string
	dim     string // scroll hints, inactive
	mid     string // secondary text
	title   string
	primary string // accent, focus, header, progress
	active  string // playing, complete
	danger  string // error, destructive
	warn    string // warning, downloading
}

// schemes are offered in the settings cycle in this order.
var schemes = []scheme{
	{
		name: "gruvbox", bg: "#282828", panel: "#32302f", hover: "#504945",
		border: "#504945", text: "#ebdbb2", dim: "#928374", mid: "#a89984",
		title: "#fbf1c7", primary: "#d3869b", active: "#b8bb26",
		danger: "#fb4934", warn: "#fabd2f",
	},
	{
		name: "nord", bg: "#2e3440", panel: "#3b4252", hover: "#434c5e",
		border: "#4c566a", text: "#d8dee9", dim: "#616e88", mid: "#aebacf",
		title: "#eceff4", primary: "#88c0d0", active: "#a3be8c",
		danger: "#bf616a", warn: "#ebcb8b",
	},
	{
		name: "dracula", bg: "#282a36", panel: "#343746", hover: "#44475a",
		border: "#44475a", text: "#f8f8f2", dim: "#6272a4", mid: "#bdbdd7",
		title: "#ffffff", primary: "#bd93f9", active: "#50fa7b",
		danger: "#ff5555", warn: "#f1fa8c",
	},
	{
		name: "catppuccin", bg: "#1e1e2e", panel: "#181825", hover: "#313244",
		border: "#45475a", text: "#cdd6f4", dim: "#6c7086", mid: "#a6adc8",
		title: "#f5e0dc", primary: "#cba6f7", active: "#a6e3a1",
		danger: "#f38ba8", warn: "#f9e2af",
	},
	{
		name: "tokyo-night", bg: "#1a1b26", panel: "#16161e", hover: "#292e42",
		border: "#3b4261", text: "#c0caf5", dim: "#565f89", mid: "#9aa5ce",
		title: "#ffffff", primary: "#bb9af7", active: "#9ece6a",
		danger: "#f7768e", warn: "#e0af68",
	},
	{
		name: "rose-pine", bg: "#191724", panel: "#1f1d2e", hover: "#26233a",
		border: "#403d52", text: "#e0def4", dim: "#6e6a86", mid: "#908caa",
		title: "#ffffff", primary: "#c4a7e7", active: "#9ccfd8",
		danger: "#eb6f92", warn: "#f6c177",
	},
	{
		name: "everforest", bg: "#2d353b", panel: "#343f44", hover: "#3d484d",
		border: "#4f585e", text: "#d3c6aa", dim: "#859289", mid: "#a7b0a4",
		title: "#e8e0ce", primary: "#d699b6", active: "#a7c080",
		danger: "#e67e80", warn: "#dbbc7f",
	},
	{
		name: "solarized-light", bg: "#fdf6e3", panel: "#eee8d5", hover: "#ddd6c1",
		border: "#c8c2ad", text: "#586e75", dim: "#93a1a1", mid: "#657b83",
		title: "#073642", primary: "#6c71c4", active: "#2aa198",
		danger: "#dc322f", warn: "#b58900",
	},
	{
		name: "latte", bg: "#eff1f5", panel: "#e6e9ef", hover: "#dce0e8",
		border: "#bcc0cc", text: "#4c4f69", dim: "#9ca0b0", mid: "#6c6f85",
		title: "#1e1e2e", primary: "#8839ef", active: "#179299",
		danger: "#d20f39", warn: "#df8e1d",
	},
}

// themeOrder is every option the settings row cycles through.
var themeOrder = func() []Theme {
	out := []Theme{ThemeTerminal, ThemeYtmgo}
	for _, s := range schemes {
		out = append(out, s.name)
	}
	return out
}()

// ParseTheme maps a stored setting back to a Theme, falling back to the
// default for anything unrecognised (a hand-edited config, or a scheme
// dropped in a later version).
func ParseTheme(s string) Theme {
	// The ytmgo palette shipped as "auto" before it was named after the
	// app. Configs written then still say that, and must keep resolving
	// to the palette those users have been looking at — not fall through
	// to the default, which is a different theme now.
	if s == "auto" {
		return ThemeYtmgo
	}
	for _, t := range themeOrder {
		if string(t) == s {
			return t
		}
	}
	return ThemeTerminal
}

// ThemeDesc is the one-line explanation shown under the settings row.
func ThemeDesc(t Theme) string {
	switch t {
	case ThemeYtmgo:
		return "ytmgo's own colours, following your terminal's light or dark background"
	case ThemeTerminal:
		return "your terminal's ANSI colours — matches whatever scheme it already runs"
	default:
		return string(t) + " — a full scheme; a dark terminal's background shows through"
	}
}

// paintBackground reports whether the active theme owns the backdrop.
// Nothing paints it on a dark terminal, so transparency survives under
// every theme. The fill exists only for a light terminal, where a named
// scheme's pale foregrounds would otherwise be read against white.
var paintBackground bool

// detectedDark is the terminal's own background, sampled once before any
// theme can overwrite it, so the ytmgo palette has something to return
// to after a named scheme has forced a value.
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

// palette expands a scheme's roles into every colour the UI draws with.
func (s scheme) palette() palette {
	c := func(v string) lipgloss.TerminalColor { return lipgloss.Color(v) }
	return palette{
		bg: c(s.bg), bgPanel: c(s.panel), bgSurface: c(s.panel), bgHover: c(s.hover),
		border: c(s.border), borderFoc: c(s.primary),
		accent: c(s.primary), accent2: c(s.active), accent3: c(s.danger),
		text: c(s.text), textDim: c(s.dim), textMid: c(s.mid), title: c(s.title),
		playing: c(s.active), download: c(s.warn), done: c(s.active),
		errColor: c(s.danger), warning: c(s.warn), header: c(s.primary),
		barFill: c(s.primary), barEmpty: c(s.border),
	}
}

// adaptive keeps ytmgo's own violet-and-mint identity, with a second set
// of values for light terminals. The light values are not the dark ones
// lightened: on white the neons wash out, so each is darkened until it
// carries enough contrast against a pale background.
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
// solarized and the rest without knowing anything about them.
func terminalScheme() palette {
	a := func(n string) lipgloss.TerminalColor { return lipgloss.Color(n) }
	return palette{
		bg: a("0"), bgPanel: a("0"), bgSurface: a("0"), bgHover: a("8"),
		border: a("8"), borderFoc: a("5"),
		accent: a("5"), accent2: a("6"), accent3: a("1"),
		text: a("7"), textDim: a("8"), textMid: a("7"), title: a("15"),
		playing: a("6"), download: a("3"), done: a("2"),
		errColor: a("1"), warning: a("3"), header: a("5"),
		barFill: a("5"), barEmpty: a("8"),
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
		paintBackground = false
		setPalette(terminalScheme())
	case ThemeYtmgo:
		paintBackground = false
		lipgloss.SetHasDarkBackground(detectedDark)
		setPalette(adaptive())
	default:
		for _, s := range schemes {
			if s.name == t {
				// Every named scheme is dark, so on a dark terminal its
				// foregrounds already read correctly against the bare
				// backdrop — leaving it unpainted is what lets the
				// terminal's transparency show through the whole UI.
				// A light terminal still needs the fill.
				paintBackground = !detectedDark
				setPalette(s.palette())
				buildStyles()
				return
			}
		}
		// Unknown name: fall back rather than leave the UI half-built.
		ApplyTheme(ThemeTerminal)
		return
	}
	buildStyles()
}

func init() {
	// Sample the terminal before anything can force a mode, then build
	// once so the package is usable even if no theme is applied later.
	detectedDark = lipgloss.HasDarkBackground()
	ApplyTheme(ThemeTerminal)
}
