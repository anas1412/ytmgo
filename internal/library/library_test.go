package library

import (
	"os"
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
	tracks, updates, err := ScanDir(dir, nil)
	if err != nil {
		t.Fatalf("ScanDir: %v", err)
	}
	if len(tracks) != 1 || len(updates) != 1 {
		t.Fatalf("first scan: %d tracks, %d updates, want 1/1", len(tracks), len(updates))
	}

	// Second scan: cache hit must be used verbatim and nothing re-probed.
	cache := DurationCache{path: {Mtime: info.ModTime().Unix(), DurationSec: 123}}
	tracks, updates, err = ScanDir(dir, cache)
	if err != nil {
		t.Fatalf("ScanDir with cache: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("cached scan produced %d updates, want 0", len(updates))
	}
	if tracks[0].DurationSec != 123 {
		t.Fatalf("cached duration = %d, want 123", tracks[0].DurationSec)
	}

	// Stale mtime must force a re-probe.
	cache[path] = CacheEntry{Mtime: info.ModTime().Unix() - 10, DurationSec: 123}
	_, updates, err = ScanDir(dir, cache)
	if err != nil {
		t.Fatalf("ScanDir stale cache: %v", err)
	}
	if len(updates) != 1 {
		t.Fatalf("stale scan produced %d updates, want 1", len(updates))
	}
}
