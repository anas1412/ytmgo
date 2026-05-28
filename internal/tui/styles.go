package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette — deep dark terminal with neon accents
var (
	colorBg        = lipgloss.Color("#0d0d0d")
	colorBgPanel   = lipgloss.Color("#111111")
	colorBgHover   = lipgloss.Color("#1a1a2e")
	colorBorder    = lipgloss.Color("#2a2a3e")
	colorBorderFoc = lipgloss.Color("#7c3aed") // violet focus
	colorAccent    = lipgloss.Color("#7c3aed") // violet
	colorAccent2   = lipgloss.Color("#06d6a0") // mint green
	colorAccent3   = lipgloss.Color("#f72585") // hot pink
	colorText      = lipgloss.Color("#e0e0e0")
	colorTextDim   = lipgloss.Color("#555566")
	colorTextMid   = lipgloss.Color("#9999aa")
	colorPlaying   = lipgloss.Color("#06d6a0") // mint = currently playing
	colorDownload  = lipgloss.Color("#f4a261") // orange = downloading
	colorDone      = lipgloss.Color("#06d6a0") // mint = done
	colorError     = lipgloss.Color("#f72585") // pink = error
	colorTitle     = lipgloss.Color("#ffffff")
	colorHeader    = lipgloss.Color("#7c3aed")
	colorBarFill   = lipgloss.Color("#7c3aed")
	colorBarEmpty  = lipgloss.Color("#2a2a3e")
)

// Panel border styles
var (
	panelBorder = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorder)

	panelBorderFocused = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colorBorderFoc)
)

// Header
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

// List items
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

// Player bar
var (
	styleNowPlaying = lipgloss.NewStyle().
	Foreground(colorAccent2).
	Bold(true).
	PaddingLeft(1)

	styleNowTitle = lipgloss.NewStyle().
	Foreground(colorTitle).
	Bold(true)

	styleNowArtist = lipgloss.NewStyle().
	Foreground(colorTextMid)

	styleTime = lipgloss.NewStyle().
	Foreground(colorTextDim)

	stylePlayerCtrl = lipgloss.NewStyle().
	Foreground(colorText).
	PaddingLeft(1)

	styleCtrlActive = lipgloss.NewStyle().
	Foreground(colorAccent).
	Bold(true)

	styleCtrlInactive = lipgloss.NewStyle().
	Foreground(colorTextDim)
)

// Download bar
var (
	styleDownloadLabel = lipgloss.NewStyle().
	Foreground(colorDownload).
	Bold(true)

	styleDownloadTitle = lipgloss.NewStyle().
	Foreground(colorText)

	styleDoneLabel = lipgloss.NewStyle().
	Foreground(colorDone)

	styleErrorLabel = lipgloss.NewStyle().
	Foreground(colorError)
)

// Panel titles
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

// Help bar
var (
	styleHelp = lipgloss.NewStyle().
	Foreground(colorTextDim).
	PaddingLeft(1)

	styleHelpKey = lipgloss.NewStyle().
	Foreground(colorAccent2)

	styleHelpSep = lipgloss.NewStyle().
	Foreground(colorTextDim)
)

// renderProgressBar draws a clean horizontal progress line
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
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarFill).Render("━"))
		} else {
			bar.WriteString(lipgloss.NewStyle().Foreground(colorBarEmpty).Render("─"))
		}
	}
	return bar.String()
}

// renderVolumeBar draws a high-fidelity visual indicator block
func renderVolumeBar(vol int, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(float64(vol) / 100.0 * float64(width))
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
