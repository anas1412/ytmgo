package ytresolve

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveURL runs Resolve against a fake yt-dlp on PATH that records
// its arguments and prints a canned --flat-playlist entry. Flat entries
// carry "url" but no "webpage_url", so this also covers the fallback.
func TestResolveURL(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + argsFile + "\n" +
		`echo '{"id":"abc123","title":"Song","url":"https://www.youtube.com/watch?v=abc123","duration":215,"channel":"Artist"}'` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	got, err := ResolveURL("Artist", "  Song \n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://www.youtube.com/watch?v=abc123"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if want := "--flat-playlist\n--dump-json\nytsearch1:Artist - Song\n"; string(args) != want {
		t.Errorf("yt-dlp args = %q, want %q", args, want)
	}
}
