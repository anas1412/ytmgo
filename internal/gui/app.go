// Package gui is the Go side of ytmgo's desktop app: the methods its
// frontend calls, built on the same internal packages as the TUI.
//
// It does not import Wails. cmd/ytmgo-gui binds an App to the window,
// and every exported method here becomes callable from the frontend.
// Keeping Wails out means this package builds and tests anywhere,
// without cgo or a WebView.
package gui

import (
	"strings"

	"ytmgo/internal/db"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/version"
)

// App is what the frontend talks to.
type App struct {
	settings *settings.Settings
	// search is search.Search, swapped out in tests so they need no
	// network.
	search func(query string, limit int) ([]search.Result, error)
}

// New returns the App to bind, reading the user's settings from d. A
// database that cannot be read leaves the defaults in place rather than
// failing to open the window.
func New(d *db.DB) *App {
	s, err := d.LoadSettings()
	if err != nil {
		s = settings.Defaults()
	}
	return &App{settings: s, search: search.Search}
}

// Version is the build's version, e.g. "v1.4.0" or "dev".
func (a *App) Version() string { return version.Version }

// Search runs a YouTube Music song search, returning as many results as
// the user's search limit allows. An empty query returns nothing and
// asks nobody.
func (a *App) Search(query string) ([]search.Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []search.Result{}, nil
	}
	limit := a.settings.SearchLimit
	if limit <= 0 {
		limit = settings.Defaults().SearchLimit
	}
	return a.search(query, limit)
}
