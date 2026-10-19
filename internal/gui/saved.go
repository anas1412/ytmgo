package gui

import (
	"slices"

	"ytmgo/internal/db"
)

// Followed artists and liked albums, kept in the database the TUI reads
// too (db.Saved), for Favourites' Artists and Albums.

// Saved lists what is kept of one kind, "artist" or "album", newest
// first.
func (a *App) Saved(kind string) []db.Saved {
	if a.loadSaved == nil {
		return []db.Saved{}
	}
	s, err := a.loadSaved(kind)
	if err != nil || s == nil {
		return []db.Saved{}
	}
	return s
}

// IsSaved says whether an artist is followed, or an album liked.
func (a *App) IsSaved(kind, id string) bool {
	return slices.ContainsFunc(a.Saved(kind), func(s db.Saved) bool { return s.ID == id })
}

// ToggleSaved follows or unfollows an artist, likes or unlikes an album,
// and says which it is now.
func (a *App) ToggleSaved(s db.Saved) (bool, error) {
	if a.IsSaved(s.Kind, s.ID) {
		return false, problem(a.unsave(s.Kind, s.ID))
	}
	return true, problem(a.save(s))
}
