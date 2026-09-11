package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// The settings page's two titles ride the top border like every other
// panel's, and no content row is spent on them.
func TestSettingsTitlesRideTheBorder(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageSettings
	out := strings.Split(m.renderSettingsPanels(), "\n")
	if len(out) == 0 {
		t.Fatal("nothing rendered")
	}
	if !strings.Contains(out[0], "SETTINGS") || !strings.Contains(out[0], "KEYBOARD SHORTCUTS") {
		t.Errorf("titles are not on the first (border) row:\n%s", out[0])
	}
	if !strings.Contains(out[0], "╭─") {
		t.Errorf("the first row is not a titled border:\n%s", out[0])
	}
	for i, l := range out[1:4] {
		if strings.Contains(l, "SETTINGS") || strings.Contains(l, "KEYBOARD SHORTCUTS") {
			t.Errorf("row %d still spends a content row on a title:\n%s", i+1, l)
		}
	}
	t.Logf("\n%s", strings.Join(out[:6], "\n"))
}

// Clicking a settings row selects the row that was clicked. The title
// moving off the content shifted every item up one line.
func TestSettingsClickHitsTheRowUnderTheCursor(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageSettings
	frame := strings.Split(m.View(), "\n")

	// Find the screen row the third setting's label is drawn on, then
	// click it and check the cursor lands on the third setting.
	want := 2
	label := settingDefs[want].label
	row := -1
	for y, l := range frame {
		if strings.Contains(l, label) {
			row = y
			break
		}
	}
	if row < 0 {
		t.Fatalf("setting %q is not on screen", label)
	}
	nm, _ := m.handleMouse(tea.MouseMsg{X: 20, Y: row, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	if got := nm.(Model).settingsCursor; got != want {
		t.Errorf("clicking row %d (%q) selected setting %d, want %d", row, label, got, want)
	}
}

// A description that does not fit is wrapped onto the next line, not
// cut off with an ellipsis — the half that got cut was the half that
// explained the setting.
func TestSettingsDescriptionsWrapInsteadOfTruncating(t *testing.T) {
	for _, w := range []int{100, 130, 150, 200} {
		m := worstCaseModel(t, w, 44)
		m.activePage = PageSettings
		// The left half only: the shortcuts panel beside it is a
		// fixed two-column table, not prose, and is not what wraps.
		var left string
		for _, l := range strings.Split(m.renderSettingsPanels(), "\n") {
			if len(l) > w/2 {
				l = l[:w/2]
			}
			left += l + "\n"
		}
		if strings.Contains(left, "…") {
			t.Errorf("width %d: a settings line is still truncated with an ellipsis:\n%s", w, left)
		}
	}
}

// Wrapped rows are taller than unwrapped ones, so a click has to be
// resolved by walking the rendered rows. Every line of the panel must
// select the item it is drawn under — checked against the frame, not
// against the builder that produced it.
func TestSettingsClickHitsTheRowItIsDrawnUnder(t *testing.T) {
	for _, w := range []int{100, 120, 150} {
		m := worstCaseModel(t, w, 44)
		m.activePage = PageSettings
		frame := strings.Split(m.View(), "\n")

		labelRow := map[int]int{} // frame line -> item index
		for y, l := range frame {
			if !strings.Contains(l, "│") {
				continue
			}
			half := l
			if len(half) > w/2 {
				half = half[:w/2]
			}
			for idx, def := range settingDefs {
				if strings.Contains(half, def.label) {
					labelRow[y] = idx
				}
			}
		}
		if len(labelRow) < 4 {
			t.Fatalf("width %d: only %d labels on screen", w, len(labelRow))
		}

		// Walk every line from the first label to the last, carrying
		// the item whose block that line belongs to.
		first, last := len(frame), 0
		for y := range labelRow {
			first, last = min(first, y), max(last, y)
		}
		cur, checked := -1, 0
		for y := first; y <= last; y++ {
			if idx, ok := labelRow[y]; ok {
				cur = idx
			}
			if cur < 0 {
				continue
			}
			nm, _ := m.handleMouse(tea.MouseMsg{
				X: 20, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
			})
			if got := nm.(Model).settingsCursor; got != cur {
				t.Errorf("width %d: clicking line %d (under %q) selected %q",
					w, y, settingDefs[cur].label, settingDefs[got].label)
			}
			checked++
		}
		t.Logf("width %d: %d lines across %d items all select their own row", w, checked, len(labelRow))
	}
}
