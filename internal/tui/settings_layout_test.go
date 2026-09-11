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
