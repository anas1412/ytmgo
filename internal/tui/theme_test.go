package tui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"ytmgo/internal/settings"
)

// TestSchemesAreDistinct: every named scheme must actually land a
// different accent, which catches a copy-paste that leaves two schemes
// sharing a palette.
func TestSchemesAreDistinct(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	seen := map[string]Theme{}
	for _, sc := range schemes {
		ApplyTheme(sc.name)
		k := fmt.Sprintf("%v|%v|%v",
			styleLogo.GetForeground(), styleDoneLabel.GetForeground(), styleErrorLabel.GetForeground())
		if prev, dup := seen[k]; dup {
			t.Errorf("%s and %s render the same accent", sc.name, prev)
		}
		seen[k] = sc.name
		if !paintBackground {
			t.Errorf("%s is a named scheme but does not paint its background", sc.name)
		}
	}
	for _, tt := range []Theme{ThemeTerminal, ThemeYtmgo} {
		ApplyTheme(tt)
		if paintBackground {
			t.Errorf("%s must leave the terminal background alone", tt)
		}
	}
}

func TestThemesAllRender(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	for _, th := range themeOrder {
		ApplyTheme(th)
		m := worstCaseModel(t, 150, 40)
		checkPanelGeometry(t, m, 150, 40, "theme "+string(th))
		if got := ParseTheme(string(th)); got != th {
			t.Errorf("ParseTheme(%q) = %q", th, got)
		}
	}
	if got := ParseTheme("nonsense"); got != ThemeTerminal {
		t.Errorf("unknown theme should fall back to the default, got %q", got)
	}
}

// TestThemeSwapsColours: a palette change must actually reach the built
// styles — the whole point of rebuilding them.
func TestThemeSwapsColours(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	// Compare the colour the style carries, not its rendered output:
	// under `go test` the output is not a TTY, so lipgloss falls back to
	// the Ascii profile and every colour renders to the same empty
	// escape regardless of the palette.
	ApplyTheme(Theme("gruvbox"))
	dark := stylePanelTitle.GetForeground()
	ApplyTheme(ThemeTerminal)
	term := stylePanelTitle.GetForeground()
	if dark == term {
		t.Errorf("terminal palette did not reach the built styles (both %v)", dark)
	}
	ApplyTheme(Theme("gruvbox"))
	if back := stylePanelTitle.GetForeground(); back != dark {
		t.Errorf("switching back gave %v, want %v", back, dark)
	}
}

// TestLegacyAutoStillResolves: the ytmgo palette shipped as "auto"
// before it was named after the app, and configs written then still say
// that. Reading one must land on the same palette, not reset the user to
// a default they never chose.
func TestLegacyAutoStillResolves(t *testing.T) {
	if got := ParseTheme("auto"); got != ThemeYtmgo {
		t.Errorf(`ParseTheme("auto") = %q, want %q`, got, ThemeYtmgo)
	}
	// And "auto" is no longer offered in the cycle.
	for _, tr := range themeOrder {
		if tr == "auto" {
			t.Error(`"auto" is still in themeOrder`)
		}
	}
}

// TestDefaultIsTerminal pins the shipped default and the order the
// settings row cycles in.
func TestDefaultIsTerminal(t *testing.T) {
	if got := ParseTheme(settings.Defaults().Theme); got != ThemeTerminal {
		t.Errorf("shipped default resolves to %q, want %q", got, ThemeTerminal)
	}
	if themeOrder[0] != ThemeTerminal || themeOrder[1] != ThemeYtmgo {
		t.Errorf("cycle starts %v, want [terminal ytmgo ...]", themeOrder[:2])
	}
	if len(themeOrder) != len(schemes)+2 {
		t.Errorf("cycle has %d entries, want %d", len(themeOrder), len(schemes)+2)
	}
}

// TestNamedSchemePaintsEveryLine: lipgloss closes each styled run with
// 39/49 — "back to the terminal's default" — which punched a hole
// through the outer background and let the terminal show through for the
// rest of the line, leaving named schemes half-painted. paintBg has to
// rewrite those to the scheme's own colours.
//
// The logic is exercised directly rather than through View, because
// under `go test` the output is not a TTY and lipgloss drops to the
// Ascii profile, emitting no colour at all.
func TestNamedSchemePaintsEveryLine(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	ApplyTheme(Theme("dracula"))

	m := Model{width: 40}
	bg := sgrColor(colorBg, 48)
	if bg == "" {
		t.Fatal("scheme has no background colour")
	}

	// A line shaped like the ones lipgloss produces: a styled run that
	// closes by handing the terminal's default back.
	line := "\x1b[38;2;1;2;3mfoo\x1b[39m\x1b[49m bar"
	out := m.paintBg(line)

	if !strings.HasPrefix(out, bg) {
		t.Errorf("line does not open with the scheme background: %.40q", out)
	}
	if strings.Contains(out, "\x1b[49m") {
		t.Error("line still hands the backdrop back mid-line")
	}
	if w := lipgloss.Width(out); w != 40 {
		t.Errorf("painted line is %d cells, want the full 40", w)
	}

	// terminal and ytmgo own no backdrop, so paintBg must pass through.
	for _, th := range []Theme{ThemeTerminal, ThemeYtmgo} {
		ApplyTheme(th)
		if got := m.paintBg(line); got != line {
			t.Errorf("%s must leave the line untouched", th)
		}
	}
}

// TestSearchFieldFitsItsBox: the textinput renders its Width plus three
// cells (the prompt and the cursor) and must fit the wrapper exactly. A
// lipgloss fixed width wraps rather than truncates, and wraps on word
// boundaries — so an input allowed to render wider than its box moved a
// space-less query wholesale to a second line that Height(1) then threw
// away, and the field showed its prompt and nothing else.
func TestSearchFieldFitsItsBox(t *testing.T) {
	m := worstCaseModel(t, 200, 50)
	boxContent := searchBoxWidth - 2*searchBoxPadding

	if m.searchInput.Width != searchInputWidth {
		t.Errorf("input width is %d, want %d — something resized it past its box",
			m.searchInput.Width, searchInputWidth)
	}
	for _, n := range []int{0, 5, 23, 40, 200} {
		m.searchInput.SetValue(strings.Repeat("a", n))
		if w := lipgloss.Width(m.searchInput.View()); w > boxContent {
			t.Errorf("value of %d chars renders %d cells, box holds %d", n, w, boxContent)
		}
	}

	// A long query must still leave a usable header: the field keeps its
	// size, the row stays one line, and the page tabs survive.
	m.searchInput.SetValue(strings.Repeat("z", 120))
	header := m.renderHeader()
	if strings.Contains(header, "\n") {
		t.Error("header wrapped to a second line")
	}
	if !strings.Contains(header, "Settings") {
		t.Error("header lost its page tabs to the search field")
	}
	// The wordmark lives in the help bar now, not the header.
	if !strings.Contains(m.renderHelpBar(), "ytmgo") {
		t.Error("help bar is not carrying the wordmark")
	}
}

// TestSearchFieldFillOnlyWhenPainted: the field's well is filled on
// themes that paint their own background and left alone on the two that
// do not. The fill is laid over the finished field in one pass, so this
// checks the rendered header rather than any single style — several
// styles each painting their own idea of the colour is exactly the bug
// that made the field look stitched together.
func TestSearchFieldFillOnlyWhenPainted(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	for _, th := range themeOrder {
		ApplyTheme(th)
		m := worstCaseModel(t, 150, 40)
		header := m.renderHeader()
		fill := sgrColor(colorBgHover, 48)
		got := fill != "" && strings.Contains(header, fill)
		if got != paintBackground {
			t.Errorf("%s: header fill = %v, want %v", th, got, paintBackground)
		}
		// However it is filled, the run must close — an unterminated
		// fill carried the colour across the page tabs beside it.
		if paintBackground && !strings.Contains(header, "\x1b[49m") {
			t.Errorf("%s: the field's fill is never closed", th)
		}
	}
}

// TestNoInvisibleText: a 16-colour palette has one usable surface
// shade, so terminal maps bgHover, border and textDim all to ANSI 8.
// Anything that draws dim text on the hover fill therefore renders the
// same colour on itself — which is what made the search placeholder
// vanish on that theme. Every theme is checked, since a scheme could
// pick a matching pair by accident too.
func TestNoInvisibleText(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	// Compared by value, not through RGBA: a lipgloss.Color does not
	// parse its hex without a renderer, so RGBA returns zero for every
	// colour under `go test` and would call them all identical.
	same := func(a, b lipgloss.TerminalColor) bool {
		if a == nil || b == nil {
			return false
		}
		if _, no := a.(lipgloss.NoColor); no {
			return false
		}
		if _, no := b.(lipgloss.NoColor); no {
			return false
		}
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}
	// The trap itself: on terminal these two are the same ANSI slot, so
	// any style pairing them renders nothing. Recorded here so the
	// reason this test exists survives the palette changing.
	ApplyTheme(ThemeTerminal)
	if !same(colorTextDim, colorBgHover) {
		t.Log("note: terminal no longer maps textDim and bgHover to one slot")
	}

	for _, th := range themeOrder {
		ApplyTheme(th)
		for _, s := range []struct {
			name  string
			style lipgloss.Style
		}{
			{"placeholder", textinputPlaceholder},
			{"search text", textinputStyle},
			{"search box", styleSearchBox},
			{"search box focused", styleSearchBoxFocused},
		} {
			if same(s.style.GetForeground(), s.style.GetBackground()) {
				t.Errorf("%s: %s draws its text in its own background colour", th, s.name)
			}
		}
	}
}

// TestNoLiteralANSIInFrames: lipgloss underlines by styling content
// rune-by-rune, which splits apart any ANSI sequences already inside
// the string and prints them as literal text — "[3;90mSearch" in the
// header is how it looked. So no rendered frame, on any theme, may
// contain an escape-shaped fragment that is not led by a real ESC.
func TestNoLiteralANSIInFrames(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	re := regexp.MustCompile(`[^\x1b]\[[0-9;]+m`)
	for _, th := range themeOrder {
		ApplyTheme(th)
		m := worstCaseModel(t, 150, 40)
		for _, line := range strings.Split(m.View(), "\n") {
			if loc := re.FindString(line); loc != "" {
				t.Fatalf("%s: frame contains a literal ANSI fragment %q — an inner escape was mangled", th, loc)
			}
		}
	}
}
