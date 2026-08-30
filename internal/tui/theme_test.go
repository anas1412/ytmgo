package tui

import (
	"fmt"
	"testing"
)

// TestSchemesAreDistinct: every named scheme must actually land a
// different accent, which catches a copy-paste that leaves two schemes
// sharing a palette.
func TestSchemesAreDistinct(t *testing.T) {
	defer ApplyTheme(ThemeAuto)
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
	for _, tt := range []Theme{ThemeAuto, ThemeTerminal} {
		ApplyTheme(tt)
		if paintBackground {
			t.Errorf("%s must leave the terminal background alone", tt)
		}
	}
}

func TestThemesAllRender(t *testing.T) {
	defer ApplyTheme(ThemeAuto)
	for _, th := range themeOrder {
		ApplyTheme(th)
		m := worstCaseModel(t, 150, 40)
		checkPanelGeometry(t, m, 150, 40, "theme "+string(th))
		if got := ParseTheme(string(th)); got != th {
			t.Errorf("ParseTheme(%q) = %q", th, got)
		}
	}
	if got := ParseTheme("nonsense"); got != ThemeAuto {
		t.Errorf("unknown theme should fall back to auto, got %q", got)
	}
}

// TestThemeSwapsColours: a palette change must actually reach the built
// styles — the whole point of rebuilding them.
func TestThemeSwapsColours(t *testing.T) {
	defer ApplyTheme(ThemeAuto)
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
