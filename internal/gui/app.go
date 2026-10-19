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
	"sync"

	"ytmgo/internal/db"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/version"
	"ytmgo/internal/ytresolve"
)

// App is what the frontend talks to.
//
// Wails calls bound methods from goroutines of its own, so everything
// that reads or changes the queue holds mu: a change is often several
// queue calls that must not interleave with another.
type App struct {
	settings *settings.Settings
	mu       sync.Mutex
	q        *queue.Queue

	// search and saveQueue are search.Search and the database's
	// SaveQueue, swapped out in tests so they need no network or disk.
	search       func(query string, limit int) ([]search.Result, error)
	saveQueue    func(tracks []queue.Track, current int, shuffle, repeat, repeatAll bool) error
	saveSettings func(*settings.Settings) error
}

// New returns the App to bind, reading the user's settings and saved
// queue from d. A database that cannot be read leaves the defaults and
// an empty queue in place rather than failing to open the window.
//
// The queue is the same one the TUI saves: on Linux, where both run,
// they share it.
func New(d *db.DB) *App {
	s, err := d.LoadSettings()
	if err != nil {
		s = settings.Defaults()
	}
	q := queue.New()
	if tracks, shuffle, repeat, repeatAll, err := d.LoadQueue(); err == nil {
		q.LoadData(tracks, shuffle, repeat, repeatAll)
	}
	return &App{settings: s, q: q, search: search.Search, saveQueue: d.SaveQueue, saveSettings: d.SaveSettings}
}

// Volume is the volume to start at: the TUI's Default Volume, which
// both apps keep up to date with the last level used.
func (a *App) Volume() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.DefaultVolume
}

// SetVolume saves the level, as the TUI does on every change, so the
// next start, in either app, comes back to it. It stays within 0–100.
func (a *App) SetVolume(v int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.DefaultVolume = min(max(v, 0), 100)
	a.saveSettings(a.settings)
}

// Version is the build's version, e.g. "v1.4.0" or "dev".
func (a *App) Version() string { return version.Version }

// Search runs a YouTube Music song search, returning as many results as
// the user's search limit allows. An empty query returns nothing and
// asks nobody. Results come as queue tracks, the one shape the page
// works with, so a result can go into the queue as it is.
func (a *App) Search(query string) ([]queue.Track, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []queue.Track{}, nil
	}
	limit := a.settings.SearchLimit
	if limit <= 0 {
		limit = settings.Defaults().SearchLimit
	}
	results, err := a.search(query, limit)
	if err != nil {
		return nil, problem(err)
	}
	tracks := make([]queue.Track, len(results))
	for i, r := range results {
		tracks[i] = r.ToTrack()
	}
	return tracks, nil
}

// StreamURL turns a track's watch URL into a direct audio stream the
// page's <audio> element can play. Streams expire after some hours, so
// the page asks for one each time a track starts.
func (a *App) StreamURL(watchURL string) (string, error) {
	url, err := ytresolve.StreamURL(watchURL)
	return url, problem(err)
}
