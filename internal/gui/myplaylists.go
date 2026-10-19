package gui

import (
	"errors"

	"ytmgo/internal/db"
	"ytmgo/internal/queue"
)

// Playlists of your own ("my playlists", apart from YouTube Music's,
// which Playlist opens), kept in the database the TUI reads too.

// MyPlaylists lists them, the one changed last first.
func (a *App) MyPlaylists() []db.PlaylistInfo {
	if a.pl == nil {
		return []db.PlaylistInfo{}
	}
	list, err := a.pl.list()
	if err != nil || list == nil {
		return []db.PlaylistInfo{}
	}
	return list
}

// MyPlaylistPage is one of them, opened: its songs, ✓ on those on disk.
type MyPlaylistPage struct {
	ID      int64         `json:"id"`
	Name    string        `json:"name"`
	Seconds int           `json:"seconds"`
	Tracks  []queue.Track `json:"tracks"`
}

// MyPlaylist opens one.
func (a *App) MyPlaylist(id int64) (MyPlaylistPage, error) {
	p, err := a.pl.load(id)
	if err != nil {
		return MyPlaylistPage{}, problem(err)
	}
	v := MyPlaylistPage{ID: p.ID, Name: p.Name, Tracks: a.mark(p.Tracks)}
	for _, t := range p.Tracks {
		v.Seconds += t.DurationSec
	}
	return v, nil
}

// CreatePlaylist makes one, empty or with songs: New playlist, or Save
// as playlist on a pasted playlist.
func (a *App) CreatePlaylist(name string, tracks []queue.Track) (int64, error) {
	id, err := a.pl.create(name, tracks)
	return id, problem(err)
}

// AddToPlaylist puts songs at the end of a playlist, leaving out those
// it has already, and says how many went in.
func (a *App) AddToPlaylist(id int64, tracks []queue.Track) (int, error) {
	p, err := a.pl.load(id)
	if err != nil {
		return 0, problem(err)
	}
	have := map[string]bool{}
	for _, t := range p.Tracks {
		have[t.ID] = true
	}
	added := 0
	for _, t := range tracks {
		if !have[t.ID] {
			have[t.ID] = true
			p.Tracks = append(p.Tracks, t)
			added++
		}
	}
	if added == 0 {
		return 0, nil
	}
	return added, problem(a.pl.set(id, p.Tracks))
}

// RemoveFromPlaylist takes the song at i out of a playlist.
func (a *App) RemoveFromPlaylist(id int64, i int) (MyPlaylistPage, error) {
	return a.editPlaylist(id, func(ts []queue.Track) ([]queue.Track, error) {
		if i < 0 || i >= len(ts) {
			return nil, errNoSuchSong
		}
		return append(ts[:i:i], ts[i+1:]...), nil
	})
}

// MovePlaylistTrack puts the song at from at position to: dragging a
// row on the playlist's page.
func (a *App) MovePlaylistTrack(id int64, from, to int) (MyPlaylistPage, error) {
	return a.editPlaylist(id, func(ts []queue.Track) ([]queue.Track, error) {
		if from < 0 || from >= len(ts) || to < 0 || to >= len(ts) {
			return nil, errNoSuchSong
		}
		t := ts[from]
		ts = append(ts[:from:from], ts[from+1:]...)
		return append(ts[:to:to], append([]queue.Track{t}, ts[to:]...)...), nil
	})
}

func (a *App) editPlaylist(id int64, edit func([]queue.Track) ([]queue.Track, error)) (MyPlaylistPage, error) {
	p, err := a.pl.load(id)
	if err != nil {
		return MyPlaylistPage{}, problem(err)
	}
	ts, err := edit(p.Tracks)
	if err != nil {
		return MyPlaylistPage{}, problem(err)
	}
	if err := a.pl.set(id, ts); err != nil {
		return MyPlaylistPage{}, problem(err)
	}
	return a.MyPlaylist(id)
}

// RenamePlaylist renames one.
func (a *App) RenamePlaylist(id int64, name string) error {
	return problem(a.pl.rename(id, name))
}

// DeletePlaylist deletes one. Its songs stay wherever else they are.
func (a *App) DeletePlaylist(id int64) error {
	return problem(a.pl.remove(id))
}

var errNoSuchSong = errors.New("that song is not in the playlist any more")

// playlistStore is the database's playlists; tests swap it.
type playlistStore struct {
	list   func() ([]db.PlaylistInfo, error)
	load   func(id int64) (db.Playlist, error)
	create func(name string, tracks []queue.Track) (int64, error)
	set    func(id int64, tracks []queue.Track) error
	rename func(id int64, name string) error
	remove func(id int64) error
}
