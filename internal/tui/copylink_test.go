package tui

import (
	"strings"
	"testing"

	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

// u copies a youtube.com link by default, and music.youtube.com once
// the setting is flipped. Both point at the same recording — only the
// site that opens differs — and a plain youtube.com link opens for
// people who do not use YouTube Music at all.
func TestCopyLinkHost(t *testing.T) {
	const id = "65-UZY0mN3o"
	for _, tc := range []struct {
		music bool
		want  string
	}{
		{false, "https://www.youtube.com/watch?v=" + id},
		{true, "https://music.youtube.com/watch?v=" + id},
	} {
		if got := ytmusic.ShareURL(id, tc.music); got != tc.want {
			t.Errorf("ShareURL(music=%v) = %q, want %q", tc.music, got, tc.want)
		}
	}

	// Playback is not a share link: mpv keeps resolving through the
	// music host whatever the user picked for the clipboard.
	if got := ytmusic.WatchURL(id); !strings.HasPrefix(got, "https://music.youtube.com/") {
		t.Errorf("playback URL changed to %q", got)
	}
}

// The setting reaches the clipboard: the status line reports what was
// copied, so it stands in for the clipboard itself (which needs a tool
// that CI does not have).
func TestCopyLinkActionFollowsTheSetting(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.searchCursor = 0
	m.results = []search.Result{{ID: "65-UZY0mN3o", Title: "A Song"}}
	m.settings = settings.Defaults()

	m.copyLinkAction()
	first := m.statusMessage
	m.settings.CopyMusicLinks = true
	m.copyLinkAction()
	second := m.statusMessage

	// Without a clipboard tool both report the same failure; the test
	// is only meaningful when a copy actually happened.
	if strings.HasPrefix(first, "Copied link:") {
		if !strings.Contains(first, "www.youtube.com") {
			t.Errorf("default copied %q, want a www.youtube.com link", first)
		}
		if !strings.Contains(second, "music.youtube.com") {
			t.Errorf("with the setting on, copied %q, want a music.youtube.com link", second)
		}
	} else {
		t.Skipf("no clipboard tool here: %q", first)
	}
}
