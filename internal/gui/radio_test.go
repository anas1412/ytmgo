package gui

import (
	"path/filepath"
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
)

// A radio is the song, then songs like it, never the song again; for a
// song from a file it asks by the song found on YouTube Music.
func TestRadio(t *testing.T) {
	var seeds [][]string
	a, _, _ := dlApp(t)
	a.recommend = func(limit int, s []string) ([]search.Result, error) {
		seeds = append(seeds, s)
		return []search.Result{{ID: "aaaaaaaaaaa", Title: "Like"}, {ID: "seedseedsee", Title: "Same"}, {ID: "bbbbbbbbbbb", Title: "Also"}}, nil
	}
	a.search = func(q string, n int) ([]search.Result, error) {
		if q != "Queen Song" || n != 1 {
			t.Errorf("search %q, %d", q, n)
		}
		return []search.Result{{ID: "seedseedsee"}}, nil
	}

	got, err := a.Radio(queue.Track{ID: "seedseedsee", Title: "Song", Artist: "Queen"})
	if err != nil || len(got) != 3 || got[0].ID != "seedseedsee" || got[1].ID != "aaaaaaaaaaa" || got[2].ID != "bbbbbbbbbbb" {
		t.Fatalf("radio %+v, %v", got, err)
	}
	// From a file: found by artist and title, which is what it is seeded by.
	got, err = a.Radio(queue.Track{ID: "/music/Queen - Song.m4a", Title: "Song", Artist: "Queen", FilePath: "/music/Queen - Song.m4a"})
	if err != nil || got[0].FilePath == "" || len(got) != 3 {
		t.Fatalf("file radio %+v, %v", got, err)
	}
	if len(seeds) != 2 || seeds[0][0] != "seedseedsee" || seeds[1][0] != "seedseedsee" {
		t.Errorf("seeds %v", seeds)
	}

	a.recommend = func(int, []string) ([]search.Result, error) { return nil, nil }
	if _, err := a.Radio(queue.Track{ID: "seedseedsee"}); err == nil {
		t.Error("an empty radio was started")
	}
}

// Show in folder opens the folder of a library file, and nothing else.
func TestShowInFolder(t *testing.T) {
	f := file(t, "Song", "Queen")
	a, _ := libApp(t, f)
	a.Library()
	var opened []string
	a.open = func(p string) error { opened = append(opened, p); return nil }

	if err := a.ShowInFolder("/etc/passwd"); err == nil || len(opened) != 0 {
		t.Errorf("opened %v, %v", opened, err)
	}
	if err := a.ShowInFolder(f.FilePath); err != nil || len(opened) != 1 || opened[0] != filepath.Dir(f.FilePath) {
		t.Errorf("opened %v, %v", opened, err)
	}
}
