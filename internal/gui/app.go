// Package gui is the Go side of ytmgo's desktop app: the methods its
// frontend calls, built on the same internal packages as the TUI.
//
// It does not import Wails. cmd/ytmgo-gui binds an App to the window,
// and every exported method here becomes callable from the frontend.
// Keeping Wails out means this package builds and tests anywhere,
// without cgo or a WebView.
package gui

import (
	"context"
	"path/filepath"
	"strings"
	"sync"

	"ytmgo/internal/db"
	"ytmgo/internal/deps"
	"ytmgo/internal/downloader"
	"ytmgo/internal/library"
	"ytmgo/internal/proc"
	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/version"
	"ytmgo/internal/ytmusic"
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
	// album, artist and playlist are the ytmusic and search fetches the
	// pages and pasted links use.
	album    func(browseID string) (ytmusic.Album, error)
	artist   func(browseID string) (ytmusic.ArtistPage, error)
	playlist func(search.PlaylistRef) (title string, results []search.Result, missed int, err error)

	lfm      lastfmAPI
	lfmToken string // while the user is approving ytmgo on last.fm

	// streamURL is ytresolve.StreamURL; open is proc.Open.
	streamURL func(watchURL string) (string, error)
	open      func(path string) error

	// fetchTool installs a tool missing on Windows, the first time it is
	// needed; quit cancels that when the window closes.
	fetchTool func(name string, progress func(done, total int64)) error
	toolMu    sync.Mutex
	quit      context.CancelFunc

	// lib is the library, nil until the first scan; scanLibrary is
	// library.Scan with the database's probe cache, and scanMu keeps
	// scans one at a time.
	lib         []queue.Track
	scanMu      sync.Mutex
	scanLibrary func(downloads string, extra []string) ([]queue.Track, error)

	host Host
	// dl is the downloader, started on first use by newDownloader;
	// dlMu guards it, apart from mu, which its progress needs.
	dlMu          sync.Mutex
	dl            downloads
	newDownloader func(dir, format string) downloads
	// local is the downloaded files PlaySource has handed out, by
	// track id, for Media to serve.
	localMu sync.Mutex
	local   map[string]string
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
	ctx, quit := context.WithCancel(context.Background())
	return &App{
		quit: quit,
		fetchTool: func(name string, progress func(done, total int64)) error {
			if filepath.IsAbs(deps.Path(name)) || !deps.CanFetch(name) {
				return nil
			}
			return deps.Fetch(ctx, name, progress)
		},
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
		lfm:        realLastFM,
		album:      ytmusic.AlbumTracks,
		artist:     ytmusic.Artist,
		playlist:   search.PlaylistTracks,
		scanLibrary: func(downloads string, extra []string) ([]queue.Track, error) {
			cache, _ := d.LoadLibraryCache() // without it, every file is read again
			tracks, updates, err := library.Scan(downloads, extra, cache)
			if err == nil {
				d.SaveLibraryCache(updates) // best effort, as in the TUI
			}
			return tracks, err
		},
		streamURL: ytresolve.StreamURL,
		open:      proc.Open,
		newDownloader: func(dir, format string) downloads {
			return downloader.New(dir, format)
		},
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

// Found is what a search found. A pasted YouTube or Spotify playlist
// link is read as that playlist (search.ParsePlaylist): then Playlist is
// its title and Missed the Spotify songs not found on YouTube Music, as
// the TUI reports them.
type Found struct {
	Tracks   []queue.Track `json:"tracks"`
	Playlist string        `json:"playlist"`
	Missed   int           `json:"missed"`
}

// Search runs a YouTube Music song search, returning as many results as
// the user's search limit allows, or a pasted playlist link's songs. An
// empty query returns nothing and asks nobody. Results come as queue
// tracks, the one shape the page works with, so a result can go into
// the queue as it is.
func (a *App) Search(query string) (Found, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Found{Tracks: []queue.Track{}}, nil
	}
	if ref, ok := search.ParsePlaylist(query); ok {
		title, results, missed, err := a.playlist(ref)
		if err != nil {
			return Found{}, problem(err)
		}
		return Found{Tracks: a.mark(asTracks(results)), Playlist: title, Missed: missed}, nil
	}
	limit := a.settings.SearchLimit
	if limit <= 0 {
		limit = settings.Defaults().SearchLimit
	}
	results, err := a.search(query, limit)
	if err != nil {
		return Found{}, problem(err)
	}
	return Found{Tracks: a.mark(asTracks(results))}, nil
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
	return a.mark(asTracks(results)), nil
}

// Played records that t has started, as the TUI does at the start of a
// song rather than after a while. Playing the same song again straight
// after only updates its time (db.RecordPlay).
func (a *App) Played(t queue.Track) error {
	a.nowPlaying(t)
	return problem(a.recordPlay(t))
}

// SettingsView is the settings the Settings page edits so far.
type SettingsView struct {
	SearchLimit    int    `json:"searchLimit"`
	Autoplay       bool   `json:"autoplay"`
	DownloadFormat string `json:"downloadFormat"` // "m4a" or "mp3"
	DownloadDir    string `json:"downloadDir"`    // where downloads go, resolved
	// LibraryFolders are the user's own music folders; the download
	// folder is always in the library besides.
	LibraryFolders []string `json:"libraryFolders"`
}

// Settings reads them.
func (a *App) Settings() SettingsView {
	a.mu.Lock()
	defer a.mu.Unlock()
	return SettingsView{
		SearchLimit: a.settings.SearchLimit, Autoplay: a.settings.AutoplayEnabled,
		DownloadFormat: a.settings.DownloadFormat, DownloadDir: a.settings.ResolveDownloadDir(),
		LibraryFolders: append([]string{}, a.settings.ResolveLibraryDirs()...),
	}
}

// SetSearchLimit sets how many results a search or the home list shows,
// within the TUI's 5–100, in its steps of 5, and saves it.
func (a *App) SetSearchLimit(n int) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	n = min(max((n+2)/5*5, 5), 100)
	a.settings.SearchLimit = n
	a.saveSettings(a.settings)
	return n
}
