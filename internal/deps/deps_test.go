package deps

import (
	"os"
	"path/filepath"
	"runtime"
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
	ours := filepath.Join(root, "app", fileName("yt-dlp"))
	write(t, ours, 0755)

	// A different, executable yt-dlp on PATH — the stale packaged one.
	pathDir := filepath.Join(root, "usr-bin")
	write(t, filepath.Join(pathDir, fileName("yt-dlp")), 0755)
	t.Setenv("PATH", pathDir)

	got := resolve("yt-dlp", filepath.Join(root, "app", "ytmgo"), "")
	if got != ours {
		t.Errorf("resolved %q, want the sibling %q", got, ours)
	}
}

// TestBinDirOrder: a tool ytmgo fetched itself comes after the one
// beside the binary but before PATH, for the same reason the sibling
// does.
func TestBinDirOrder(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "data", "bin")
	fetched := filepath.Join(binDir, fileName("ffmpeg"))
	write(t, fetched, 0755)
	pathDir := filepath.Join(root, "usr-bin")
	write(t, filepath.Join(pathDir, fileName("ffmpeg")), 0755)
	t.Setenv("PATH", pathDir)

	exe := filepath.Join(root, "app", "ytmgo")
	if got := resolve("ffmpeg", exe, binDir); got != fetched {
		t.Errorf("resolved %q, want the fetched %q over PATH", got, fetched)
	}

	sibling := filepath.Join(root, "app", fileName("ffmpeg"))
	write(t, sibling, 0755)
	if got := resolve("ffmpeg", exe, binDir); got != sibling {
		t.Errorf("resolved %q, want the sibling %q over the fetched one", got, sibling)
	}
}

// TestFallsBackToPath: without a sibling, whatever PATH offers is right.
func TestFallsBackToPath(t *testing.T) {
	root := t.TempDir()
	pathDir := filepath.Join(root, "usr-bin")
	onPath := filepath.Join(pathDir, fileName("yt-dlp"))
	write(t, onPath, 0755)
	t.Setenv("PATH", pathDir)

	got := resolve("yt-dlp", filepath.Join(root, "app", "ytmgo"), "")
	if got != onPath {
		t.Errorf("resolved %q, want %q from PATH", got, onPath)
	}
}

// TestNonExecutableSiblingIsIgnored: a yt-dlp that cannot be run is not
// a yt-dlp. A half-finished download would otherwise shadow a working
// copy on PATH.
func TestNonExecutableSiblingIsIgnored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no execute bit")
	}
	root := t.TempDir()
	write(t, filepath.Join(root, "app", "yt-dlp"), 0644) // no +x

	pathDir := filepath.Join(root, "usr-bin")
	onPath := filepath.Join(pathDir, "yt-dlp")
	write(t, onPath, 0755)
	t.Setenv("PATH", pathDir)

	got := resolve("yt-dlp", filepath.Join(root, "app", "ytmgo"), "")
	if got != onPath {
		t.Errorf("a non-executable sibling was used: %q", got)
	}
}

// TestNothingAnywhere: the bare name, so exec reports the failure in
// its own words rather than this package inventing one.
func TestNothingAnywhere(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PATH", filepath.Join(root, "empty"))
	if got := resolve("yt-dlp", filepath.Join(root, "app", "ytmgo"), ""); got != "yt-dlp" {
		t.Errorf("resolved %q, want the bare name", got)
	}
}
