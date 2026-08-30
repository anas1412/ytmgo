package tui

import (
	"fmt"
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
