package gui

import (
	"testing"

	"ytmgo/internal/db"
	"ytmgo/internal/settings"
)

func TestToggleSaved(t *testing.T) {
	kept := map[string]db.Saved{}
	a := &App{settings: settings.Defaults(),
		save:   func(s db.Saved) error { kept[s.Kind+s.ID] = s; return nil },
		unsave: func(kind, id string) error { delete(kept, kind+id); return nil },
		loadSaved: func(kind string) ([]db.Saved, error) {
			out := []db.Saved{}
			for _, s := range kept {
				if s.Kind == kind {
					out = append(out, s)
				}
			}
			return out, nil
		},
	}
	daft := db.Saved{Kind: "artist", ID: "UCdaft", Title: "Daft Punk"}
	if on, err := a.ToggleSaved(daft); !on || err != nil || !a.IsSaved("artist", "UCdaft") {
		t.Fatalf("follow: %v, %v", on, err)
	}
	if got := a.Saved("artist"); len(got) != 1 || got[0].Title != "Daft Punk" {
		t.Errorf("followed %+v", got)
	}
	if len(a.Saved("album")) != 0 {
		t.Error("an artist showed as an album")
	}
	if on, _ := a.ToggleSaved(daft); on || a.IsSaved("artist", "UCdaft") {
		t.Error("unfollow did not take")
	}
}
