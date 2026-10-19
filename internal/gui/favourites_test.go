package gui

import (
	"testing"

	"ytmgo/internal/queue"
)

func favApp() (*App, *[][]string) {
	var saved [][]string
	return &App{favs: []queue.Track{}, saveFavs: func(ts []queue.Track) error {
		ids := make([]string, len(ts))
		for i, t := range ts {
			ids[i] = t.ID
		}
		saved = append(saved, ids)
		return nil
	}}, &saved
}

// As the TUI's f: a new favourite goes to the top, the same song again
// takes it out, and every change is saved whole.
func TestToggleFavourite(t *testing.T) {
	a, saved := favApp()
	a.ToggleFavourite(tr("a"))
	a.ToggleFavourite(tr("b"))
	got := a.ToggleFavourite(tr("c"))
	if want := []string{"c", "b", "a"}; !eq(idsOf(got), want) {
		t.Errorf("favourites %v, want newest first %v", idsOf(got), want)
	}
	got = a.ToggleFavourite(tr("b"))
	if want := []string{"c", "a"}; !eq(idsOf(got), want) {
		t.Errorf("after removing b: %v, want %v", idsOf(got), want)
	}
	if len(*saved) != 4 || !eq((*saved)[3], []string{"c", "a"}) {
		t.Errorf("saves %v", *saved)
	}
}

// What the page is handed is a copy: changing it must not change the
// list Go keeps.
func TestFavouritesAreACopy(t *testing.T) {
	a, _ := favApp()
	a.ToggleFavourite(tr("a"))
	got := a.Favourites()
	got[0].ID = "changed"
	if a.Favourites()[0].ID != "a" {
		t.Error("the page's copy reached the stored list")
	}
}

func idsOf(ts []queue.Track) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}
