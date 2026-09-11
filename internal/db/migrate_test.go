package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"ytmgo/internal/queue"
)

// seedLegacy writes a database at path holding one queue track, with
// that write still sitting in the write-ahead log.
//
// Getting there takes a detour: SQLite checkpoints and deletes the -wal
// when the last connection closes cleanly, so simply writing and
// closing leaves nothing to lose and would make the test below pass no
// matter what the migration did. A -wal survives an *unclean* exit —
// ytmgo killed, or the machine going down — so that is what this
// simulates, by copying the files out from under a live connection.
func seedLegacy(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(t.TempDir(), "live.db")
	d, err := openAt(live)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveQueue([]queue.Track{{ID: "abc", Title: "Kept", Artist: "Someone"}}, 0, false, false, false); err != nil {
		t.Fatal(err)
	}
	// Snapshot while the connection is still open: the -wal is on disk
	// and holds the write, exactly as after a kill.
	for _, suffix := range []string{"", "-wal", "-shm"} {
		b, err := os.ReadFile(live + suffix)
		if err != nil {
			continue
		}
		if err := os.WriteFile(path+suffix, b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	d.DB.Close()

	if fi, err := os.Stat(path + "-wal"); err != nil || fi.Size() == 0 {
		t.Fatalf("seed did not leave a populated -wal; the migration test would prove nothing")
	}
}

// TestMigrateKeepsTheDatabase: the pre-v1 database moves to the new
// path intact.
func TestMigrateKeepsTheDatabase(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, ".config", "ytmgo", "ytmgo.db")
	newPath := filepath.Join(root, ".local", "share", "ytmgo", "ytmgo.db")
	seedLegacy(t, oldPath)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}

	if err := migrateLegacyDB(oldPath, newPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Errorf("the old database is still there")
	}

	d, err := openAt(newPath)
	if err != nil {
		t.Fatalf("open migrated: %v", err)
	}
	defer d.DB.Close()
	tracks, _, _, _, err := d.LoadQueue()
	if err != nil {
		t.Fatalf("load queue: %v", err)
	}
	// The whole point: this row was in the write-ahead log, not the main
	// file, when the move happened. Renaming the .db alone loses it.
	if len(tracks) != 1 || tracks[0].Title != "Kept" {
		t.Errorf("migrated queue is %+v, want the one seeded track", tracks)
	}
}

// TestMigrateLeavesTheSidecars: nothing of the old database may remain,
// or a later downgrade reopens a stale -wal against no main file.
func TestMigrateLeavesNothingBehind(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old", "ytmgo.db")
	newPath := filepath.Join(root, "new", "ytmgo.db")
	seedLegacy(t, oldPath)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := migrateLegacyDB(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(oldPath + suffix); !os.IsNotExist(err) {
			t.Errorf("%s survived the migration", filepath.Base(oldPath+suffix))
		}
	}
}

// TestMigrateWillNotOverwrite: a database already at the new path wins.
// Running an old and a new build alternately must not clobber the one
// the new build has been writing to.
func TestMigrateWillNotOverwrite(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old", "ytmgo.db")
	newPath := filepath.Join(root, "new", "ytmgo.db")
	seedLegacy(t, oldPath)

	// A different database at the destination, with a different track.
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	d, err := openAt(newPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveQueue([]queue.Track{{ID: "xyz", Title: "Current"}}, 0, false, false, false); err != nil {
		t.Fatal(err)
	}
	d.DB.Close()

	if err := migrateLegacyDB(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	d2, err := openAt(newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer d2.DB.Close()
	tracks, _, _, _, err := d2.LoadQueue()
	if err != nil {
		t.Fatal(err)
	}
	if len(tracks) != 1 || tracks[0].Title != "Current" {
		t.Errorf("migration overwrote the newer database: got %+v", tracks)
	}
}

// TestMigrateOnFreshInstall: no old database is not an error.
func TestMigrateOnFreshInstall(t *testing.T) {
	root := t.TempDir()
	if err := migrateLegacyDB(filepath.Join(root, "nope", "ytmgo.db"), filepath.Join(root, "new", "ytmgo.db")); err != nil {
		t.Errorf("a fresh install should not fail: %v", err)
	}
}

// TestMigrateIsIdempotent: running it twice is a no-op the second time,
// which is what every launch after the upgrade does.
func TestMigrateIsIdempotent(t *testing.T) {
	root := t.TempDir()
	oldPath := filepath.Join(root, "old", "ytmgo.db")
	newPath := filepath.Join(root, "new", "ytmgo.db")
	seedLegacy(t, oldPath)
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := migrateLegacyDB(oldPath, newPath); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if _, err := sql.Open("sqlite", newPath); err != nil {
		t.Fatal(err)
	}
}

// TestPaneVisibilityRoundTrips: hiding the spectrum or the lyrics has
// to survive a restart, which means the two flags must make it through
// SaveSettings and back out of LoadSettings.
func TestPaneVisibilityRoundTrips(t *testing.T) {
	d, err := openAt(filepath.Join(t.TempDir(), "ytmgo.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.DB.Close()

	// A fresh database opens with both panes on.
	got, err := d.LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !got.VisualizerOn || !got.LyricsOn {
		t.Errorf("a new install starts with visualizer=%v lyrics=%v, want both on",
			got.VisualizerOn, got.LyricsOn)
	}

	for _, c := range []struct{ viz, lyr bool }{
		{false, false}, {true, false}, {false, true}, {true, true},
	} {
		got.VisualizerOn, got.LyricsOn = c.viz, c.lyr
		if err := d.SaveSettings(got); err != nil {
			t.Fatal(err)
		}
		back, err := d.LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if back.VisualizerOn != c.viz || back.LyricsOn != c.lyr {
			t.Errorf("saved visualizer=%v lyrics=%v, loaded %v/%v",
				c.viz, c.lyr, back.VisualizerOn, back.LyricsOn)
		}
	}
}
