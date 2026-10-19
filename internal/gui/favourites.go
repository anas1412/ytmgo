package gui

import "ytmgo/internal/queue"

// Favourites are the TUI's: newest first, saved whole on every change
// (db.SaveFavorites), shared with the TUI on Linux.

// Favourites lists them, newest first.
func (a *App) Favourites() []queue.Track {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]queue.Track{}, a.favs...)
}

// ToggleFavourite adds t to the top of the favourites, or removes it if
// it is there already, as the TUI's f does, and answers with the list.
func (a *App) ToggleFavourite(t queue.Track) []queue.Track {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, have := range a.favs {
		if have.ID == t.ID {
			a.favs = append(a.favs[:i:i], a.favs[i+1:]...)
			a.saveFavs(a.favs)
			return append([]queue.Track{}, a.favs...)
		}
	}
	a.favs = append([]queue.Track{t}, a.favs...)
	a.saveFavs(a.favs)
	return append([]queue.Track{}, a.favs...)
}
