package coverart

import "testing"

// Which terminals get the real image. kitty defined the protocol but
// does not have it to itself: Ghostty and WezTerm implement it too, and
// identify themselves differently — looking only for kitty sent both to
// the half-block fallback though they could draw the real thing.
func TestKittySupportedDetectsEveryTerminalThatCan(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"kitty, by its own variable", map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "xterm-kitty"}, true},
		{"kitty, by TERM alone", map[string]string{"TERM": "xterm-kitty"}, true},
		{"ghostty", map[string]string{"TERM": "xterm-ghostty", "TERM_PROGRAM": "ghostty"}, true},
		{"ghostty, TERM only", map[string]string{"TERM": "xterm-ghostty"}, true},
		{"wezterm, which leaves TERM generic", map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "WezTerm"}, true},

		// No inline-image protocol at all — the fallback is correct.
		{"alacritty", map[string]string{"TERM": "alacritty"}, false},
		{"plain xterm", map[string]string{"TERM": "xterm-256color"}, false},
		{"the VS Code terminal", map[string]string{"TERM": "xterm-256color", "TERM_PROGRAM": "vscode"}, false},

		// A multiplexer swallows the escapes whatever is underneath.
		{"kitty inside tmux", map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "xterm-kitty", "TMUX": "/tmp/tmux-1000/default,1,0"}, false},
		{"ghostty inside tmux", map[string]string{"TERM_PROGRAM": "ghostty", "TMUX": "/tmp/x"}, false},
		{"kitty inside screen", map[string]string{"KITTY_WINDOW_ID": "1", "TERM": "screen.xterm-kitty"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range []string{"KITTY_WINDOW_ID", "TERM", "TERM_PROGRAM", "TMUX"} {
				t.Setenv(k, "")
			}
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if got := KittySupported(); got != tc.want {
				t.Errorf("KittySupported() = %v, want %v", got, tc.want)
			}
		})
	}
}
