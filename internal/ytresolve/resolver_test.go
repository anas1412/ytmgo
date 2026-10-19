package ytresolve

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeDir holds the one fake yt-dlp every test in the package runs.
// One for all of them, because deps.Path resolves a tool once per
// process: a fake made per test would be deleted along with that test's
// temp dir, and the next test would run the cached path to nothing.
var fakeDir string

// TestMain puts a fake yt-dlp on PATH. It writes its arguments to
// $FAKE_ARGS and prints $FAKE_OUT, both set by each test through fake.
// Unix only: it is a shell script. On Windows the tests that use it
// skip — what they check is the arguments and the parsing, which do not
// change per platform, and running the command is proc's job, which has
// its own Windows test.
func TestMain(m *testing.M) {
	if runtime.GOOS != "windows" {
		dir, err := os.MkdirTemp("", "ytresolve-fake")
		if err != nil {
			panic(err)
		}
		script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_ARGS\"\ncat \"$FAKE_OUT\"\n"
		if err := os.WriteFile(filepath.Join(dir, "yt-dlp"), []byte(script), 0755); err != nil {
			panic(err)
		}
		// In front of the real PATH: the script itself needs cat.
		os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		fakeDir = dir
	}
	code := m.Run()
	if fakeDir != "" {
		os.RemoveAll(fakeDir)
	}
	os.Exit(code)
}

// fake makes the fake yt-dlp print out, and returns the file its
// arguments land in.
func fake(t *testing.T, out string) (argsFile string) {
	t.Helper()
	if fakeDir == "" {
		t.Skip("fakes yt-dlp with a shell script")
	}
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out")
	if err := os.WriteFile(outFile, []byte(out), 0644); err != nil {
		t.Fatal(err)
	}
	argsFile = filepath.Join(dir, "args")
	t.Setenv("FAKE_OUT", outFile)
	t.Setenv("FAKE_ARGS", argsFile)
	return argsFile
}

func readArgs(t *testing.T, argsFile string) string {
	t.Helper()
	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("yt-dlp was not run: %v", err)
	}
	return string(args)
}

// TestResolveURL runs Resolve against the fake, printing a canned
// --flat-playlist entry. Flat entries carry "url" but no "webpage_url",
// so this also covers the fallback.
func TestResolveURL(t *testing.T) {
	argsFile := fake(t, `{"id":"abc123","title":"Song","url":"https://www.youtube.com/watch?v=abc123","duration":215,"channel":"Artist"}`+"\n")

	got, err := ResolveURL("Artist", "  Song \n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://www.youtube.com/watch?v=abc123"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	if want := "--flat-playlist\n--dump-json\nytsearch1:Artist - Song\n"; readArgs(t, argsFile) != want {
		t.Errorf("yt-dlp args = %q, want %q", readArgs(t, argsFile), want)
	}
}

func TestStreamURL(t *testing.T) {
	argsFile := fake(t, "https://rr1.googlevideo.com/videoplayback?id=1\n")

	got, err := StreamURL("https://music.youtube.com/watch?v=abc123")
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://rr1.googlevideo.com/videoplayback?id=1"; got != want {
		t.Errorf("url = %q, want %q", got, want)
	}
	want := "-f\nbestaudio\n--get-url\n--no-playlist\nhttps://music.youtube.com/watch?v=abc123\n"
	if got := readArgs(t, argsFile); got != want {
		t.Errorf("yt-dlp args = %q, want %q", got, want)
	}
}

// Output that is not a URL is an error, not a stream the player would
// fail on later with nothing to say why.
func TestStreamURLRejectsNonURL(t *testing.T) {
	argsFile := fake(t, "WARNING: something\n")

	_, err := StreamURL("https://music.youtube.com/watch?v=abc123")
	readArgs(t, argsFile) // it must have got as far as running yt-dlp
	if err == nil {
		t.Fatal("non-URL output was accepted")
	}
}
