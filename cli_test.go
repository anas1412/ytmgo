package main

import "testing"

func TestJoinQuery(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"homage"}, "homage"},
		{[]string{"mild", "high", "club"}, "mild high club"},
		{[]string{"ラブ・ストーリーは突然に"}, "ラブ・ストーリーは突然に"},
		{[]string{"", "a", "", "b"}, "a b"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := joinQuery(c.in); got != c.want {
			t.Errorf("joinQuery(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFmtDur(t *testing.T) {
	cases := map[int]string{0: "0:00", -5: "0:00", 7: "0:07", 178: "2:58", 3725: "1:02:05"}
	for in, want := range cases {
		if got := fmtDur(in); got != want {
			t.Errorf("fmtDur(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestRunCLIDispatch covers the routing that decides between a headless
// subcommand and opening the TUI. Network-backed commands are exercised
// only through their missing-query path, which returns before any call.
func TestRunCLIDispatch(t *testing.T) {
	cases := []struct {
		name        string
		args        []string
		wantHandled bool
		wantCode    int
	}{
		{"no args opens the TUI", nil, false, 0},
		{"unknown arg opens the TUI", []string{"wat"}, false, 0},
		{"help is handled", []string{"help"}, true, 0},
		{"--help is handled", []string{"--help"}, true, 0},
		{"search without query errors", []string{"search"}, true, 2},
		{"play without query errors", []string{"play"}, true, 2},
		{"download without query errors", []string{"download"}, true, 2},
		{"blank query errors", []string{"play", ""}, true, 2},
	}
	for _, c := range cases {
		handled, code := runCLI(c.args)
		if handled != c.wantHandled || code != c.wantCode {
			t.Errorf("%s: runCLI(%q) = (%v, %d), want (%v, %d)",
				c.name, c.args, handled, code, c.wantHandled, c.wantCode)
		}
	}
}
