package db

import (
	"fmt"
	"testing"

	"ytmgo/internal/library"
	"ytmgo/internal/queue"
)

// openTestDB gives each test its own SQLite file via a fake HOME.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	d, err := Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSettingsRoundTrip(t *testing.T) {
	d := openTestDB(t)
	s, err := d.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.DefaultVolume != 80 || s.DownloadFormat != "m4a" {
		t.Fatalf("unexpected defaults: %+v", s)
	}

	s.DefaultVolume = 55
	s.AutoplayEnabled = false
	if err := d.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := d.LoadSettings()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.DefaultVolume != 55 || got.AutoplayEnabled {
		t.Fatalf("settings not persisted: %+v", got)
	}
}

func TestQueueRoundTrip(t *testing.T) {
	d := openTestDB(t)
	tracks := []queue.Track{
		{ID: "sZxzPcT1Meg", Title: "ラブ・ストーリーは突然に", Artist: "Kazumasa Oda", URL: "https://music.youtube.com/watch?v=sZxzPcT1Meg"},
	}
	if err := d.SaveQueue(tracks, 0, true, false, true); err != nil {
		t.Fatalf("SaveQueue: %v", err)
	}
	got, shuffle, repeat, repeatAll, err := d.LoadQueue()
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if len(got) != 1 || got[0].URL == "" || got[0].Title != tracks[0].Title {
		t.Fatalf("queue round trip lost data: %+v", got)
	}
	if !shuffle || repeat || !repeatAll {
		t.Fatalf("flags = %v %v %v, want true false true", shuffle, repeat, repeatAll)
	}
}

func TestFavoritesKeepURL(t *testing.T) {
	d := openTestDB(t)
	favs := []queue.Track{{ID: "abcdefghijk", Title: "T", Artist: "A", URL: "https://music.youtube.com/watch?v=abcdefghijk", CoverURL: "https://img"}}
	if err := d.SaveFavorites(favs); err != nil {
		t.Fatalf("SaveFavorites: %v", err)
	}
	got, err := d.LoadFavorites()
	if err != nil {
		t.Fatalf("LoadFavorites: %v", err)
	}
	if len(got) != 1 || got[0].URL != favs[0].URL || got[0].CoverURL != favs[0].CoverURL {
		t.Fatalf("favorites lost url/cover: %+v", got)
	}
}

func TestRecordPlayDedupsConsecutive(t *testing.T) {
	d := openTestDB(t)
	tr := queue.Track{ID: "aaaaaaaaaaa", Title: "T", Artist: "A"}
	for i := 0; i < 3; i++ {
		if err := d.RecordPlay(tr); err != nil {
			t.Fatalf("RecordPlay: %v", err)
		}
	}
	d.RecordPlay(queue.Track{ID: "bbbbbbbbbbb", Title: "U"})
	entries, err := d.LoadPlayHistory(10, 0)
	if err != nil {
		t.Fatalf("LoadPlayHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("history has %d entries, want 2 (consecutive plays deduped)", len(entries))
	}
}

func TestURLCache(t *testing.T) {
	d := openTestDB(t)
	if got, err := d.LoadCachedURL("nope"); err != nil || got != "" {
		t.Fatalf("missing entry = %q err=%v, want empty nil", got, err)
	}
	if err := d.SaveCachedURL("id1", "https://u1"); err != nil {
		t.Fatalf("SaveCachedURL: %v", err)
	}
	if err := d.SaveCachedURL("id1", "https://u2"); err != nil {
		t.Fatalf("SaveCachedURL replace: %v", err)
	}
	if got, _ := d.LoadCachedURL("id1"); got != "https://u2" {
		t.Fatalf("cached url = %q, want replaced value", got)
	}
}

func TestLyricsCache(t *testing.T) {
	d := openTestDB(t)

	// Never looked up: found=false, distinct from a recorded miss.
	if _, _, found, err := d.LoadCachedLyrics("nope"); err != nil || found {
		t.Fatalf("missing entry: found=%v err=%v, want false nil", found, err)
	}

	// A synced hit round-trips.
	if err := d.SaveCachedLyrics("id1", "[00:05.00] hello", true); err != nil {
		t.Fatalf("SaveCachedLyrics: %v", err)
	}
	text, synced, found, err := d.LoadCachedLyrics("id1")
	if err != nil || !found || text != "[00:05.00] hello" || !synced {
		t.Fatalf("cached lyrics = %q synced=%v found=%v err=%v", text, synced, found, err)
	}

	// A recorded miss ("no lyrics exist") is found with empty text, so
	// callers skip the refetch without confusing it with never-tried.
	if err := d.SaveCachedLyrics("id2", "", false); err != nil {
		t.Fatalf("SaveCachedLyrics miss: %v", err)
	}
	if text, _, found, err := d.LoadCachedLyrics("id2"); err != nil || !found || text != "" {
		t.Fatalf("recorded miss = %q found=%v err=%v, want empty true nil", text, found, err)
	}
}

func TestLibraryCacheRoundTrip(t *testing.T) {
	d := openTestDB(t)
	in := library.DurationCache{
		"/music/a.m4a": {Mtime: 100, DurationSec: 200},
		"/music/b.mp3": {Mtime: 300, DurationSec: 400},
	}
	if err := d.SaveLibraryCache(in); err != nil {
		t.Fatalf("SaveLibraryCache: %v", err)
	}
	got, err := d.LoadLibraryCache()
	if err != nil {
		t.Fatalf("LoadLibraryCache: %v", err)
	}
	if len(got) != 2 || got["/music/a.m4a"].DurationSec != 200 || got["/music/b.mp3"].Mtime != 300 {
		t.Fatalf("cache round trip mismatch: %+v", got)
	}
}

// TestThemeRoundTrips: the theme is the setting most likely to be
// noticed if it silently resets, and it shipped without a column — saved
// fine for the session, then came back empty on the next launch.
func TestThemeRoundTrips(t *testing.T) {
	d := openTestDB(t)

	s, err := d.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if s.Theme == "" {
		t.Error("a fresh database should carry the default theme, not an empty string")
	}

	s.Theme = "gruvbox"
	if err := d.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	got, err := d.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings: %v", err)
	}
	if got.Theme != "gruvbox" {
		t.Errorf("theme came back as %q, want gruvbox", got.Theme)
	}
}

// TestLyricsCacheIsBounded: lyrics are cached forever by design — a
// song's words do not change — so the table needs a size bound or it
// grows for the life of the install.
func TestLyricsCacheIsBounded(t *testing.T) {
	d := openTestDB(t)

	defer func(n int) { lyricsCacheMax = n }(lyricsCacheMax)
	lyricsCacheMax = 50

	for i := 0; i < lyricsCacheMax+20; i++ {
		if err := d.SaveCachedLyrics(fmt.Sprintf("vid%05d", i), "la la la", false); err != nil {
			t.Fatalf("SaveCachedLyrics: %v", err)
		}
	}
	var n int
	if err := d.QueryRow(`SELECT count(*) FROM lyrics_cache`).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n > lyricsCacheMax {
		t.Errorf("cache holds %d rows, cap is %d", n, lyricsCacheMax)
	}
	// The most recent write must survive the trim that follows it.
	newest := fmt.Sprintf("vid%05d", lyricsCacheMax+19)
	if _, _, found, err := d.LoadCachedLyrics(newest); err != nil || !found {
		t.Errorf("newest entry was evicted (found=%v, err=%v)", found, err)
	}
}
