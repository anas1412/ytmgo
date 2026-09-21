package library

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseFilename(t *testing.T) {
	cases := []struct {
		in            string
		title, artist string
	}{
		{"Kazumasa Oda - ラブ・ストーリーは突然に.m4a", "ラブ・ストーリーは突然に", "Kazumasa Oda"},
		{"Artist - Song (Official Video).mp3", "Song", "Artist"},
		{"NoDashTitle.mp3", "NoDashTitle", ""},
		{"A - B - C.m4a", "B - C", "A"},
	}
	for _, c := range cases {
		title, artist := parseFilename(c.in)
		if title != c.title || artist != c.artist {
			t.Errorf("parseFilename(%q) = (%q, %q), want (%q, %q)", c.in, title, artist, c.title, c.artist)
		}
	}
}

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"Song (Official Music Video)": "Song",
		"Song [Lyrics]":               "Song",
		"Song | extra":                "Song",
		"Plain":                       "Plain",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScanDirUsesCache verifies that a cached entry with a matching
// mtime skips the ffprobe call (the file here isn't real audio, so a
// probe returns 0 while the cache value is nonzero and must win).
func TestScanDirUsesCache(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Artist - Song.m4a")
	if err := os.WriteFile(path, []byte("not really audio"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// First scan: no cache, file gets probed (duration 0 for garbage)
	// and lands in updates.
	tracks, updates, err := Scan(dir, nil, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 1 || len(updates) != 1 {
		t.Fatalf("first scan: %d tracks, %d updates, want 1/1", len(tracks), len(updates))
	}

	// Second scan: cache hit must be used verbatim and nothing re-probed.
	cache := DurationCache{path: {Mtime: info.ModTime().Unix(), DurationSec: 123}}
	tracks, updates, err = Scan(dir, nil, cache)
	if err != nil {
		t.Fatalf("Scan with cache: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("cached scan produced %d updates, want 0", len(updates))
	}
	if tracks[0].DurationSec != 123 {
		t.Fatalf("cached duration = %d, want 123", tracks[0].DurationSec)
	}

	// Stale mtime must force a re-probe.
	cache[path] = CacheEntry{Mtime: info.ModTime().Unix() - 10, DurationSec: 123}
	_, updates, err = Scan(dir, nil, cache)
	if err != nil {
		t.Fatalf("Scan stale cache: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("stale scan produced %d updates, want 1", len(updates))
	}
}

// Files with no tags are named by whose they are. ytmgo wrote its own
// downloads as "Artist - Title", so those split; a file in the user's
// folder could be either way round, so the whole name is the title.
// Folders are walked recursively — album downloads sit in subfolders —
// and .opus is audio too.
func TestScanRulesByRoot(t *testing.T) {
	downloads := t.TempDir()
	mine := t.TempDir()
	write := func(path string) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not audio"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(downloads, "Artist - Song.m4a"))
	write(filepath.Join(downloads, "Some Artist - Some Album", "01 - Track.m4a"))
	write(filepath.Join(mine, "anas - xd.mp3"))
	write(filepath.Join(mine, "deep", "er", "loop.opus"))
	write(filepath.Join(mine, "cover.jpg"))

	tracks, _, err := Scan(downloads, []string{mine}, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	got := map[string]Track{}
	for _, tr := range tracks {
		got[filepath.Base(tr.FilePath)] = tr
	}
	if len(got) != 4 {
		t.Fatalf("found %d audio files, want 4 (jpg skipped, subfolders walked): %v", len(got), tracks)
	}
	if tr := got["Artist - Song.m4a"]; tr.Title != "Song" || tr.Artist != "Artist" {
		t.Errorf("download split wrong: %q / %q", tr.Title, tr.Artist)
	}
	if tr := got["01 - Track.m4a"]; tr.Title != "Track" || tr.Artist != "01" {
		t.Errorf("album download follows the same convention: %q / %q", tr.Title, tr.Artist)
	}
	if tr := got["anas - xd.mp3"]; tr.Title != "anas - xd" || tr.Artist != "" {
		t.Errorf("user's file was guessed at: %q / %q, want the whole name and no artist", tr.Title, tr.Artist)
	}
	if _, ok := got["loop.opus"]; !ok {
		t.Error("opus in a nested folder not found")
	}
}

// Tags win over filenames wherever the file is, and they must be read
// from every format ytmgo plays — Ogg and Opus keep theirs on the stream
// rather than the container, which is exactly the kind of thing that
// silently degrades to filenames.
func TestProbeReadsTagsAcrossFormats(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	for _, ext := range []string{"m4a", "mp3", "flac", "ogg", "opus"} {
		path := filepath.Join(dir, "whatever - name."+ext)
		cmd := exec.Command("ffmpeg", "-v", "quiet", "-y", "-f", "lavfi", "-i", "anullsrc=r=44100:cl=mono", "-t", "0.3",
			"-metadata", "title=Song T", "-metadata", "artist=Artist A", "-metadata", "album=Album B", path)
		if err := cmd.Run(); err != nil {
			t.Fatalf("ffmpeg %s: %v", ext, err)
		}
	}
	tracks, _, err := Scan(dir, nil, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(tracks) != 5 {
		t.Fatalf("got %d tracks, want 5", len(tracks))
	}
	for _, tr := range tracks {
		if tr.Title != "Song T" || tr.Artist != "Artist A" || tr.Album != "Album B" {
			t.Errorf("%s: tags not read: title=%q artist=%q album=%q", filepath.Ext(tr.FilePath), tr.Title, tr.Artist, tr.Album)
		}
	}
}
