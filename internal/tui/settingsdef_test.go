package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Hints and Autoplay have their own keys on screen — z and the player
// bar's AUTO — so a duplicate row on the Settings page was a second
// place to change one value. They still persist when toggled from
// there, which is what TestKeyTogglesStillPersist checks.
func TestSettingsDoesNotDuplicateOnScreenToggles(t *testing.T) {
	for _, d := range settingDefs {
		switch d.label {
		case "Show Key Hints", "Autoplay":
			t.Errorf("%q is on the Settings page and has its own key too", d.label)
		}
	}
}

// The page reads in the order someone works through it: what plays,
// where it lands on disk, how much is fetched, then how it looks, then
// what leaves the app.
func TestSettingsOrder(t *testing.T) {
	want := []string{
		"Playback Mode", "Default Volume",
		"Download Dir", "Download Format",
		"Search Limit",
		"Theme", "Show Quotes",
		"Copy Link As", "Discord RPC",
	}
	if len(settingDefs) != len(want) {
		t.Fatalf("the page has %d settings, this test knows %d", len(settingDefs), len(want))
	}
	for i, w := range want {
		if settingDefs[i].label != w {
			t.Errorf("setting %d is %q, want %q", i, settingDefs[i].label, w)
		}
	}
}

// z and the autoplay control write through to the database, so removing
// their rows from the page cost nothing.
func TestKeyTogglesStillPersist(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.settings.ShowHints = true
	m.settings.AutoplayEnabled = false

	if cmd := m.toggleAutoplayAction(); cmd == nil {
		t.Error("toggling autoplay saves nothing")
	}
	if !m.settings.AutoplayEnabled {
		t.Error("autoplay did not flip")
	}
	nm, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
	if cmd == nil {
		t.Error("z saves nothing")
	}
	if nm.(Model).settings.ShowHints {
		t.Error("z did not flip the hints")
	}
}
