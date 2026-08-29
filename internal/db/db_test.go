package db

import (
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
