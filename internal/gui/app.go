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
	favs     []queue.Track

	// search and saveQueue are search.Search and the database's
	// SaveQueue, swapped out in tests so they need no network or disk.
	search       func(query string, limit int) ([]search.Result, error)
	saveQueue    func(tracks []queue.Track, current int, shuffle, repeat, repeatAll bool) error
	saveSettings func(*settings.Settings) error
	// history is the track ids of the newest plays, newest first;
	// recommend is search.FetchRecommendations; recordPlay is the
	// database's RecordPlay.
	history    func(limit int) ([]string, error)
	recommend  func(limit int, seeds []string) ([]search.Result, error)
	recordPlay func(queue.Track) error
	saveFavs   func([]queue.Track) error
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
	favs, err := d.LoadFavorites()
	if err != nil {
		favs = []queue.Track{}
	}
	return &App{
		settings:     s,
		q:            q,
		favs:         favs,
		search:       search.Search,
		saveQueue:    d.SaveQueue,
		saveSettings: d.SaveSettings,
		history: func(limit int) ([]string, error) {
			entries, err := d.LoadPlayHistory(limit, 0)
			ids := make([]string, len(entries))
			for i, e := range entries {
				ids[i] = e.TrackID
			}
			return ids, err
		},
		recommend:  search.FetchRecommendations,
		recordPlay: d.RecordPlay,
		saveFavs:   d.SaveFavorites,
	}
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
	return asTracks(results), nil
}

func asTracks(results []search.Result) []queue.Track {
	tracks := make([]queue.Track, len(results))
	for i, r := range results {
		tracks[i] = r.ToTrack()
	}
	return tracks
}

// seeds are the radio seeds, from the newest plays, as the TUI picks
// them for its recommendations and autoplay.
func (a *App) seeds() ([]string, error) {
	ids, err := a.history(search.SeedHistory)
	if err != nil {
		return nil, err
	}
	return search.Seeds(ids, search.SeedCount), nil
}

// Recommendations is the home list when nothing is searched: songs
// related to what was played lately, as many as the search limit. With
// nothing played yet there is nothing to go on, and it is empty.
func (a *App) Recommendations() ([]queue.Track, error) {
	seeds, err := a.seeds()
	if err != nil {
		return nil, problem(err)
	}
	if len(seeds) == 0 {
		return []queue.Track{}, nil
	}
	limit := a.settings.SearchLimit
	if limit <= 0 {
		limit = settings.Defaults().SearchLimit
	}
	results, err := a.recommend(limit, seeds)
	if err != nil {
		return nil, problem(err)
	}
	return asTracks(results), nil
}

// Played records that t has started, as the TUI does at the start of a
// song rather than after a while. Playing the same song again straight
// after only updates its time (db.RecordPlay).
func (a *App) Played(t queue.Track) error {
	return problem(a.recordPlay(t))
}

// StreamURL turns a track's watch URL into a direct audio stream the
// page's <audio> element can play. Streams expire after some hours, so
// the page asks for one each time a track starts.
func (a *App) StreamURL(watchURL string) (string, error) {
	url, err := ytresolve.StreamURL(watchURL)
	return url, problem(err)
}
