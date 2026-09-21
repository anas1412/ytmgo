package tui

import (
	"strings"
	"testing"

	"ytmgo/internal/settings"

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
		"Copy Link As", "Discord RPC", "Last.fm Scrobbling",
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

// The mode names say what you get. "Hybrid" named the implementation,
// and the thing a reader actually needs to know — does this keep the
// track on disk — was not in any of the three words.
func TestPlaybackModeLabelsSayWhatHappens(t *testing.T) {
	for _, tc := range []struct {
		mode int
		want string
	}{
		{settings.PlaybackStream, "nothing is saved"},
		{settings.PlaybackHybrid, "Download while playing"},
		{settings.PlaybackOffline, "Download first"},
	} {
		got := settings.PlaybackModeLabel(tc.mode)
		if !strings.Contains(got, tc.want) {
			t.Errorf("mode %d reads %q, want it to mention %q", tc.mode, got, tc.want)
		}
	}
	// An unrecognised value used to read "Hybrid" — the one mode that
	// writes files — while the default is to write none.
	if got := settings.PlaybackModeLabel(99); got != settings.PlaybackModeLabel(settings.PlaybackStream) {
		t.Errorf("an unknown mode reads %q, want the default's label", got)
	}
	// Each mode is distinct on the page, or cycling looks like nothing
	// happened.
	seen := map[string]bool{}
	for _, mode := range []int{settings.PlaybackStream, settings.PlaybackHybrid, settings.PlaybackOffline} {
		l := settings.PlaybackModeLabel(mode)
		if seen[l] {
			t.Errorf("two modes both read %q", l)
		}
		seen[l] = true
	}
}
