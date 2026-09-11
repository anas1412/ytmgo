package db

import (
	"path/filepath"
	"testing"
)

// The copy-link host is a setting, so it has to come back after a
// restart — the point of putting it in the database rather than in the
// model. A column added by migration also has to be readable on a
// database created before it existed.
func TestCopyLinkHostSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ytmgo.db")

	d, err := openAt(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s, err := d.LoadSettings()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.CopyMusicLinks {
		t.Error("a fresh install copies music.youtube.com links, want youtube.com")
	}
	s.CopyMusicLinks = true
	if err := d.SaveSettings(s); err != nil {
		t.Fatalf("save: %v", err)
	}
	d.Close()

	d2, err := openAt(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer d2.Close()
	s2, err := d2.LoadSettings()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !s2.CopyMusicLinks {
		t.Error("the choice did not survive the restart")
	}
}
