package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Contrast checks for the selected row. A cursor row paints the accent
// behind text that was picked for the panel background, so a scheme can
// end up writing its title colour on a fill of nearly the same
// lightness — which is what "the title is barely visible when selected"
// looked like.
//
// Hex is parsed here rather than read back through lipgloss: a
// lipgloss.Color does not resolve without a renderer, so RGBA() answers
// zero for every colour under `go test`.

// TestSelectedRowIsReadable: on the cursor row of every scheme, the
// colour the row is written in must be legible against the accent fill
// painted behind it. Before onAccent existed the row used the scheme's
// title colour — chosen against the panel background, not the accent —
// and landed between 1.6:1 and 3.0:1 on every scheme shipped.
func TestSelectedRowIsReadable(t *testing.T) {
	for _, sc := range schemes {
		fg := onAccentFor(sc)
		if got := contrastRatio(fg, sc.primary); got < minSelectedContrast {
			t.Errorf("%s: selected row %s on fill %s is %.2f:1, want %.2f:1",
				sc.name, fg, sc.primary, got, minSelectedContrast)
		}
	}
	// ytmgo's accent is a deep violet in both variants, so its onAccent
	// is hard-coded white; hold that to the same floor.
	for _, v := range []struct{ name, accent string }{
		{"ytmgo light", "#6d28d9"},
		{"ytmgo dark", "#7c3aed"},
	} {
		if got := contrastRatio("#ffffff", v.accent); got < minSelectedContrast {
			t.Errorf("%s: white on accent %s is %.2f:1, want %.2f:1",
				v.name, v.accent, got, minSelectedContrast)
		}
	}
}

// TestEveryThemeSetsOnAccent: a palette that forgets the role leaves it
// nil, and lipgloss then draws the row in whatever the terminal's
// default foreground is — usually near-invisible on the accent.
func TestEveryThemeSetsOnAccent(t *testing.T) {
	defer ApplyTheme(ThemeTerminal)
	for _, th := range themeOrder {
		ApplyTheme(th)
		if colorOnAccent == nil || fmt.Sprintf("%v", colorOnAccent) == "" {
			t.Errorf("%s: onAccent is unset", th)
		}
		if fmt.Sprintf("%v", colorOnAccent) == fmt.Sprintf("%v", colorAccent) {
			t.Errorf("%s: onAccent equals the accent it is written on", th)
		}
	}
}

// TestSelectedRowRendersInOnAccent renders the real row and reads the
// escapes back, so the palette wiring is checked against output rather
// than against itself. lipgloss emits nothing without a profile under
// `go test`, hence the explicit TrueColor.
func TestSelectedRowRendersInOnAccent(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	defer ApplyTheme(ThemeTerminal)

	// The expected sequence comes from lipgloss rendering the same
	// colour, not from formatting the hex by hand: termenv truncates a
	// channel on the way out, so a hand-built "38;2;45;53;59" misses
	// the "38;2;44;52;59" it actually writes.
	fgParams := func(c lipgloss.TerminalColor) string {
		r := lipgloss.NewStyle().Foreground(c).Render("x")
		return strings.Trim(r[:strings.Index(r, "x")], "\x1b[m")
	}

	for _, sc := range schemes {
		ApplyTheme(sc.name)
		// isPlaying true is the case from the report: the cursor on the
		// playing track used to switch to the scheme's green.
		got := renderListItemBlock("1. Title", "  Artist", true, true, 40)
		if want := fgParams(colorOnAccent); !strings.Contains(got, want) {
			t.Errorf("%s: selected playing row does not use onAccent %s (%s)",
				sc.name, onAccentFor(sc), want)
		}
		if bad := fgParams(lipgloss.Color(sc.active)); strings.Contains(got, bad) {
			t.Errorf("%s: selected row still writes in the playing colour %s", sc.name, sc.active)
		}
	}
}

// TestReportSelectedContrast prints the table; not an assertion.
func TestReportSelectedContrast(t *testing.T) {
	for _, sc := range schemes {
		fg := onAccentFor(sc)
		fmt.Printf("%-16s fill %s  text %s  %.2f:1   (was: title %.2f, playing %.2f)\n",
			sc.name, sc.primary, fg,
			contrastRatio(fg, sc.primary),
			contrastRatio(sc.title, sc.primary),
			contrastRatio(sc.active, sc.primary))
	}
}

// TestSelectedRowIsHighlightedToTheEnd: the cursor row's highlight has
// to reach the duration on the right, in every theme. Each inner Render
// ends in a reset that clears the background too, so relying on one
// outer Background painted the title and nothing after it — the
// duration sat on the panel's own colour, unreadable.
func TestSelectedRowIsHighlightedToTheEnd(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(old)
	defer ApplyTheme(ThemeTerminal)

	bgParams := func(c lipgloss.TerminalColor) string {
		r := lipgloss.NewStyle().Background(c).Render("x")
		return strings.Trim(r[:strings.Index(r, "x")], "\x1b[m")
	}

	for _, th := range themeOrder {
		ApplyTheme(th)
		row := renderAlbumTrackLine("001. ", "A Song", " · Someone", "4:39", true, 60)
		want := bgParams(colorAccent)

		// The duration is the last thing on the row; the background has
		// to still be set where it starts.
		i := strings.LastIndex(row, "4:39")
		if i < 0 {
			t.Errorf("%s: the duration is missing from the row", th)
			continue
		}
		// Not "somewhere before" — the title supplies that whatever
		// happens. The background must still be set after the last
		// reset preceding the duration, which is what a span-by-span
		// background gives and a single outer one does not.
		before := row[:i]
		if r := strings.LastIndex(before, "\x1b[0m"); r >= 0 {
			before = before[r:]
		}
		if !strings.Contains(before, want) {
			t.Errorf("%s: the highlight is reset before the duration — it sits on the panel colour", th)
		}
	}
}
