package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ─── Color Palette ──────────────────────────────────────────────────
// Industrial cyberpunk: deep backdrop, neon accents with semantic weight.
var (
	colorBg        = lipgloss.Color("#0d0d0d")
	colorBgPanel   = lipgloss.Color("#111111")
	colorBgSurface = lipgloss.Color("#0d0d12")
	colorBgHover   = lipgloss.Color("#1a1a2e")
	colorBorder    = lipgloss.Color("#2a2a3e")
	colorBorderFoc = lipgloss.Color("#7c3aed") // violet = focused
	colorAccent    = lipgloss.Color("#7c3aed") // violet = primary accent
	colorAccent2   = lipgloss.Color("#06d6a0") // mint = active/playing
	colorAccent3   = lipgloss.Color("#f72585") // pink = error/destructive
	colorText      = lipgloss.Color("#e0e0e0")
	colorTextDim   = lipgloss.Color("#555566")
	colorTextMid   = lipgloss.Color("#9999aa")
	colorPlaying   = lipgloss.Color("#06d6a0") // mint = now playing
	colorDownload  = lipgloss.Color("#f4a261") // orange = downloading
	colorDone      = lipgloss.Color("#06d6a0") // mint = complete
	colorError     = lipgloss.Color("#f72585") // pink = error
	colorWarning   = lipgloss.Color("#f4a261") // orange = warning/confirm
	colorTitle     = lipgloss.Color("#ffffff")
	colorHeader    = lipgloss.Color("#7c3aed")
	colorBarFill   = lipgloss.Color("#7c3aed")
	colorBarEmpty  = lipgloss.Color("#2a2a3e")
	// Progress bar gradient characters — denser fills for visual weight
	barCharFull  = "█"
	barCharMid   = "▓"
	barCharLight = "▒"
	barCharEmpty = "░"
)

// ─── Panel Borders ──────────────────────────────────────────────────

var (
	panelBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder)

	panelBorderFocused = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorderFoc)

	// Settings page uses same border style for consistency
	panelBorderSettings = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 2, 0, 2)

	// Confirmation dialog border — double line for modal weight
	styleConfirmBorder = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorAccent3).
				Padding(1, 3, 1, 3).
				Background(colorBg)
)

// ─── Header ─────────────────────────────────────────────────────────

var (
	styleHeader = lipgloss.NewStyle().
			Foreground(colorHeader).
			Bold(true).
			PaddingLeft(1)

	styleLogo = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	styleVersion = lipgloss.NewStyle().
			Foreground(colorTextDim)
)

// ─── Separator / Horizontal Rule ────────────────────────────────────

var (
	// Heavy separator line — used to divide major layout sections
	styleSeparatorHeavy = lipgloss.NewStyle().
				Foreground(colorBorder).
				PaddingLeft(1).
				PaddingRight(1)

	// Light separator — used between control groups
	styleSeparatorLight = lipgloss.NewStyle().
				Foreground(colorTextDim)
)

func renderSeparator(char string, width int) string {
	return lipgloss.NewStyle().
		Foreground(colorBorder).
		Render(strings.Repeat(char, width))
}

// ─── List Items ─────────────────────────────────────────────────────

var (
	styleListItem = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(1)

	styleListItemSelected = lipgloss.NewStyle().
				Foreground(colorTitle).
				Background(colorBgHover).
				PaddingLeft(1).
				Bold(true)

	styleListItemPlaying = lipgloss.NewStyle().
				Foreground(colorPlaying).
				PaddingLeft(1).
				Bold(true)

	styleListNum = lipgloss.NewStyle().
			Foreground(colorTextDim).
			Width(3)

	styleListDuration = lipgloss.NewStyle().
				Foreground(colorTextDim).
				Align(lipgloss.Right)

	styleListArtist = lipgloss.NewStyle().
			Foreground(colorTextMid)
)

// ─── Player Bar ─────────────────────────────────────────────────────

var (
	// Playing state — mint double border for a neon-glow terminal feel
	stylePlayerBox = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorPlaying).
			Padding(0, 2, 0, 2)

	// Stopped state — dim single border
	stylePlayerBoxStopped = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorTextDim).
				Padding(0, 2, 0, 2)

	styleNowTitle = lipgloss.NewStyle().
			Foreground(colorTitle).
			Bold(true)

	styleNowArtist = lipgloss.NewStyle().
			Foreground(colorTextMid)

	styleNowIndicator = lipgloss.NewStyle().
				Foreground(colorPlaying)

	styleTime = lipgloss.NewStyle().
			Foreground(colorTextDim)

	stylePlayerCtrl = lipgloss.NewStyle().
			Foreground(colorText).
			PaddingLeft(1)

	styleCtrlBtn = lipgloss.NewStyle().
			Foreground(colorTextMid).
			Bold(true)

	styleCtrlBtnActive = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true)

	styleCtrlBtnPlaying = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	// Mode indicators
	styleModeActive = lipgloss.NewStyle().
			Foreground(colorAccent2).
			Bold(true)

	styleModeInactive = lipgloss.NewStyle().
				Foreground(colorTextDim)

	// Volume bar
	styleVolumeLabel = lipgloss.NewStyle().
				Foreground(colorTextDim)

	// Separator for controls
	styleCtrlSep = lipgloss.NewStyle().
			Foreground(colorTextDim)
)

// ─── Download Bar ───────────────────────────────────────────────────

var (
	styleDownloadLabel = lipgloss.NewStyle().
				Foreground(colorDownload).
				Bold(true)

	styleDownloadTitle = lipgloss.NewStyle().
				Foreground(colorText)

	styleDoneLabel = lipgloss.NewStyle().
			Foreground(colorDone).
			Bold(true)

	styleErrorLabel = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)
)

// ─── Panel Titles ───────────────────────────────────────────────────

var (
	stylePanelTitle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true).
			PaddingLeft(1).
			PaddingRight(1)

	styleSearchInput = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorBgHover).
				PaddingLeft(1).
				PaddingRight(1)
)

// ─── Page Navigation Tabs ───────────────────────────────────────────

var (
	styleNavTab = lipgloss.NewStyle().
			Foreground(colorTextDim).
			PaddingLeft(1).
			PaddingRight(1)

	styleNavTabActive = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true).
				PaddingLeft(1).
				PaddingRight(1)
)

// ─── Settings ───────────────────────────────────────────────────────

var (
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

	styleSettingsCursor = lipgloss.NewStyle().
				Foreground(colorBgHover).
				Background(colorAccent).
				PaddingLeft(1).
				PaddingRight(1)

	styleSettingsBoolOn = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true)

	styleSettingsBoolOff = lipgloss.NewStyle().
				Foreground(colorTextDim)
)

// ─── Confirmation Dialog ────────────────────────────────────────────

var (
	styleConfirmTitle = lipgloss.NewStyle().
				Foreground(colorAccent3).
				Bold(true)

	styleConfirmText = lipgloss.NewStyle().
				Foreground(colorText).
				PaddingLeft(0)

	styleConfirmHint = lipgloss.NewStyle().
				Foreground(colorTextDim).
				Italic(true)
)

// ─── Help Bar ───────────────────────────────────────────────────────

var (
	styleHelp = lipgloss.NewStyle().
			Foreground(colorTextDim).
			PaddingLeft(1)

	styleHelpKey = lipgloss.NewStyle().
			Foreground(colorAccent2)

	styleHelpSep = lipgloss.NewStyle().
			Foreground(colorTextDim)
)

// ─── Status ─────────────────────────────────────────────────────────

var (
	styleStatus = lipgloss.NewStyle().
			Foreground(colorAccent2).
			PaddingLeft(1)

	styleStatusErr = lipgloss.NewStyle().
			Foreground(colorError).
			PaddingLeft(1)
)

// ─── Progress / Volume Bars ─────────────────────────────────────────

// renderProgressBar draws a proportional bar with gradient character density.
// Uses a 3-level gradient (█ ▓ ▒) for the filled portion to add depth,
// and ░ for the unfilled portion.
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
		switch {
		case i < filled:
			// Gradient from dense to light as bar fills
			ratio := float64(i) / float64(width)
			var ch string
			switch {
			case ratio > 0.7:
				ch = barCharLight
			case ratio > 0.3:
				ch = barCharMid
			default:
				ch = barCharFull
			}
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarFill).Render(ch))
		case i == filled && filled > 0 && filled < width:
			// Partial block at the boundary
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render("▒"))
		default:
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render(barCharEmpty))
		}
	}
	return bar.String()
}

// renderVolumeBar draws a block-style volume indicator.
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
