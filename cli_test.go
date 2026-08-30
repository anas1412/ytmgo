package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ytmgo/internal/settings"
)

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
		{"play rejects options", []string{"play", "-f", "mp3", "x"}, true, 2},
		{"play rejects album flag", []string{"play", "-a", "x"}, true, 2},
	}
	for _, c := range cases {
		handled, code := runCLI(c.args)
		if handled != c.wantHandled || code != c.wantCode {
			t.Errorf("%s: runCLI(%q) = (%v, %d), want (%v, %d)",
				c.name, c.args, handled, code, c.wantHandled, c.wantCode)
		}
	}
}

func TestProgressBar(t *testing.T) {
	cases := []struct{ pct, width int }{{0, 10}, {50, 10}, {100, 10}}
	for _, c := range cases {
		got := progressBar(float64(c.pct), c.width)
		// "[" + width cells + "]"
		if n := len([]rune(got)); n != c.width+2 {
			t.Errorf("progressBar(%d,%d) = %q, want %d runes", c.pct, c.width, got, c.width+2)
		}
	}
	if progressBar(0, 10) != "[░░░░░░░░░░]" {
		t.Errorf("0%% bar = %q", progressBar(0, 10))
	}
	if progressBar(100, 10) != "[██████████]" {
		t.Errorf("100%% bar = %q", progressBar(100, 10))
	}
	// out-of-range input must not panic or overflow the width
	if got := progressBar(150, 10); len([]rune(got)) != 12 {
		t.Errorf("over-100%% bar = %q", got)
	}
	if got := progressBar(-5, 10); len([]rune(got)) != 12 {
		t.Errorf("negative bar = %q", got)
	}
	if progressBar(50, 0) != "" {
		t.Error("zero width should render nothing")
	}
}

func TestResolveFormat(t *testing.T) {
	set := &settings.Settings{DownloadFormat: "m4a"}
	for in, want := range map[string]string{"": "m4a", "mp3": "mp3", "MP3": "mp3", "m4a": "m4a"} {
		got, err := resolveFormat(in, set)
		if err != nil || got != want {
			t.Errorf("resolveFormat(%q) = (%q, %v), want %q", in, got, err, want)
		}
	}
	if _, err := resolveFormat("flac", set); err == nil {
		t.Error("unsupported format should error")
	}
}

func TestResolveLocation(t *testing.T) {
	dir := t.TempDir()
	set := &settings.Settings{DownloadDir: dir}

	if got, _ := resolveLocation("", set); got != dir {
		t.Errorf("empty location = %q, want configured dir %q", got, dir)
	}
	// "." resolves to the working directory, absolute
	got, err := resolveLocation(".", set)
	if err != nil {
		t.Fatalf("resolveLocation(\".\") errored: %v", err)
	}
	wd, _ := os.Getwd()
	if got != wd {
		t.Errorf("resolveLocation(\".\") = %q, want %q", got, wd)
	}
	// a nested path is created on demand
	nested := filepath.Join(dir, "a", "b")
	if got, err := resolveLocation(nested, set); err != nil || got != nested {
		t.Errorf("nested = (%q, %v), want %q created", got, err, nested)
	}
	if fi, err := os.Stat(nested); err != nil || !fi.IsDir() {
		t.Error("nested location was not created")
	}
}

func TestSanitizePathSegment(t *testing.T) {
	cases := map[string]string{
		"Mild High Club - Timeline": "Mild High Club - Timeline",
		"AC/DC - Back In Black":     "AC_DC - Back In Black",
		`Hi: "there" <x>|y*z?`:      `Hi_ _there_ _x__y_z_`,
		"  padded  ":                "padded",
		"ラブ・ストーリーは突然に":              "ラブ・ストーリーは突然に",
	}
	for in, want := range cases {
		if got := sanitizePathSegment(in); got != want {
			t.Errorf("sanitizePathSegment(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestAlbumFlagParsing checks that -a is consumed as a flag rather than
// swallowed into the query, on both commands that accept it.
func TestAlbumFlagParsing(t *testing.T) {
	// No query after the flag is still a missing-query error, which the
	// dispatcher maps to exit 2 — proving -a was parsed, not treated as
	// the search text.
	for _, args := range [][]string{
		{"search", "-a"},
		{"search", "--album"},
		{"download", "-a"},
		{"download", "-a", "-f", "mp3"},
	} {
		handled, code := runCLI(args)
		if !handled || code != 2 {
			t.Errorf("runCLI(%q) = (%v, %d), want (true, 2)", args, handled, code)
		}
	}
	// An unknown flag is misuse, not a runtime failure.
	if handled, code := runCLI([]string{"download", "--nope", "x"}); !handled || code != 2 {
		t.Errorf("unknown flag = (%v, %d), want (true, 2)", handled, code)
	}
}

// TestHoistFlags covers the parsing bug where options placed after the
// query were silently swallowed into the search text — so "download
// bebalee -f mp3" downloaded the wrong format into the wrong place.
func TestHoistFlags(t *testing.T) {
	cases := []struct {
		name      string
		in        []string
		wantFlags []string
		wantQuery []string
	}{
		{"flags first", []string{"-a", "bebalee"}, []string{"-a"}, []string{"bebalee"}},
		{"flags last", []string{"bebalee", "-a"}, []string{"-a"}, []string{"bebalee"}},
		{"value flag after query", []string{"bebalee", "-f", "mp3"}, []string{"-f", "mp3"}, []string{"bebalee"}},
		{"interleaved", []string{"-a", "bebalee", "-l", ".", "-f", "mp3"},
			[]string{"-a", "-l", ".", "-f", "mp3"}, []string{"bebalee"}},
		{"long form with equals", []string{"--format=mp3", "x"}, []string{"--format=mp3"}, []string{"x"}},
		{"multi-word query", []string{"-a", "mild", "high", "club"}, []string{"-a"}, []string{"mild", "high", "club"}},
		{"double dash makes text literal", []string{"--", "-weird-title"}, nil, []string{"-weird-title"}},
	}
	for _, c := range cases {
		flags, query := hoistFlags(c.in)
		if strings.Join(flags, " ") != strings.Join(c.wantFlags, " ") {
			t.Errorf("%s: flags = %q, want %q", c.name, flags, c.wantFlags)
		}
		if strings.Join(query, " ") != strings.Join(c.wantQuery, " ") {
			t.Errorf("%s: query = %q, want %q", c.name, query, c.wantQuery)
		}
	}
}

// TestJoinQueryDropsDashSeparator: our listings print "Title — Artist",
// so a pasted-back line must not search for the dash.
func TestJoinQueryDropsDashSeparator(t *testing.T) {
	if got := joinQuery([]string{"Bebalee", "—", "Fairuz"}); got != "Bebalee Fairuz" {
		t.Errorf("em-dash not dropped: %q", got)
	}
	if got := joinQuery([]string{"Bebalee", "–", "Fairuz"}); got != "Bebalee Fairuz" {
		t.Errorf("en-dash not dropped: %q", got)
	}
	// A hyphen inside a real title must survive.
	if got := joinQuery([]string{"Jay-Z", "song"}); got != "Jay-Z song" {
		t.Errorf("hyphenated word broken: %q", got)
	}
}
