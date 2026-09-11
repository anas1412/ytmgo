package ytdlp

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), mode); err != nil {
		t.Fatal(err)
	}
}

// TestSiblingBeatsPath is the whole point: install.sh puts an upstream
// yt-dlp beside the ytmgo binary, but a packaged one in /usr/bin
// usually sits earlier on PATH. Ours has to win without the user
// reordering anything.
func TestSiblingBeatsPath(t *testing.T) {
	root := t.TempDir()
	ours := filepath.Join(root, "app", "yt-dlp")
	write(t, ours, 0755)

	// A different, executable yt-dlp on PATH — the stale packaged one.
	pathDir := filepath.Join(root, "usr-bin")
	write(t, filepath.Join(pathDir, "yt-dlp"), 0755)
	t.Setenv("PATH", pathDir)

	got := resolveBeside(filepath.Join(root, "app", "ytmgo"))
	if got != ours {
		t.Errorf("resolved %q, want the sibling %q", got, ours)
	}
}

// TestFallsBackToPath: without a sibling, whatever PATH offers is right.
func TestFallsBackToPath(t *testing.T) {
	root := t.TempDir()
	pathDir := filepath.Join(root, "usr-bin")
	onPath := filepath.Join(pathDir, "yt-dlp")
	write(t, onPath, 0755)
	t.Setenv("PATH", pathDir)

	got := resolveBeside(filepath.Join(root, "app", "ytmgo"))
	if got != onPath {
		t.Errorf("resolved %q, want %q from PATH", got, onPath)
	}
}

// TestNonExecutableSiblingIsIgnored: a yt-dlp that cannot be run is not
// a yt-dlp. A half-finished download would otherwise shadow a working
// copy on PATH.
func TestNonExecutableSiblingIsIgnored(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "app", "yt-dlp"), 0644) // no +x

	pathDir := filepath.Join(root, "usr-bin")
	onPath := filepath.Join(pathDir, "yt-dlp")
	write(t, onPath, 0755)
	t.Setenv("PATH", pathDir)

	got := resolveBeside(filepath.Join(root, "app", "ytmgo"))
	if got != onPath {
		t.Errorf("a non-executable sibling was used: %q", got)
	}
}

// TestNothingAnywhere: the bare name, so exec reports the failure in
// its own words rather than this package inventing one.
func TestNothingAnywhere(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", filepath.Join(root, "empty"))
	if got := resolveBeside(filepath.Join(root, "app", "ytmgo")); got != "yt-dlp" {
		t.Errorf("resolved %q, want the bare name", got)
	}
}
