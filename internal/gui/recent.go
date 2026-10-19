package gui

import "ytmgo/internal/db"

// recentShown is how many sources the quick tiles show after
// Favourites: two rows of four.
const recentShown = 7

// PlayedFrom notes that the album or playlist s was played from, for
// the quick tiles. A failed save loses only the tile.
func (a *App) PlayedFrom(s db.Source) {
	if a.recordSource != nil {
		a.recordSource(s)
	}
}

// Recent lists the albums and playlists played from lately, newest
// first.
func (a *App) Recent() []db.Source {
	if a.sources == nil {
		return []db.Source{}
	}
	s, err := a.sources(recentShown)
	if err != nil || s == nil {
		return []db.Source{}
	}
	return s
}
