package tui

import (
	"strings"
	"testing"
)

// The footer names the two keys that get you anywhere from a track.
// They were reachable only by memory or by opening Settings.
func TestFooterShowsAlbumAndArtist(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	bar := m.renderHelpBar()
	for _, want := range []string{"album", "artist", "copy link", "quit"} {
		if !strings.Contains(bar, want) {
			t.Errorf("the footer does not offer %q:\n%s", want, bar)
		}
	}
	// The letters come from the bindings, so a rebind cannot leave the
	// footer advertising a key that does nothing.
	for _, b := range Keys.ShortHelp() {
		if len(b.Keys()) == 0 || b.Help().Key == "" {
			t.Errorf("footer entry %q has no key behind it", b.Help().Desc)
		}
	}
	if got := Keys.ShortHelp()[0].Keys()[0]; got != Keys.AlbumInfo.Keys()[0] {
		t.Errorf("the footer's album entry is bound to %q, not %q", got, Keys.AlbumInfo.Keys()[0])
	}
	t.Logf("footer: %s", strings.TrimSpace(bar))
}
