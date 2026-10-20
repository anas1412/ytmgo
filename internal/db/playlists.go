package db

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"ytmgo/internal/queue"
)

// Playlists of your own, shared by the TUI and the GUI. A playlist's
// songs are kept whole, in order, as the queue's are.

// PlaylistInfo is a playlist as a list shows it: its name, how many
// songs, how long, and the covers its picture is made of (Covers).
type PlaylistInfo struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Count   int      `json:"count"`
	Seconds int      `json:"seconds"`
	Covers  []string `json:"covers"`
}

// Playlist is one playlist and its songs.
type Playlist struct {
	ID     int64         `json:"id"`
	Name   string        `json:"name"`
	Tracks []queue.Track `json:"tracks"`
}

// CreatePlaylist makes a playlist, empty or with songs, and returns its
// id. A blank name is an error: a playlist is found by its name.
func (d *DB) CreatePlaylist(name string, tracks []queue.Track) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, fmt.Errorf("create playlist: no name")
	}
	raw, err := encodeTracks(tracks)
	if err != nil {
		return 0, fmt.Errorf("create playlist: %w", err)
	}
	res, err := d.Exec(`INSERT INTO playlists (name, tracks, touched) VALUES (?, ?, `+nextTouch+`)`, name, raw)
	if err != nil {
		return 0, fmt.Errorf("create playlist: %w", err)
	}
	return res.LastInsertId()
}

// LoadPlaylists lists every playlist, the one changed last first.
func (d *DB) LoadPlaylists() ([]PlaylistInfo, error) {
	rows, err := d.Query(`SELECT id, name, tracks FROM playlists ORDER BY touched DESC`)
	if err != nil {
		return nil, fmt.Errorf("load playlists: %w", err)
	}
	defer rows.Close()
	out := []PlaylistInfo{}
	for rows.Next() {
		var p PlaylistInfo
		var raw string
		if err := rows.Scan(&p.ID, &p.Name, &raw); err != nil {
			return nil, fmt.Errorf("load playlists: scan: %w", err)
		}
		var tracks []queue.Track
		json.Unmarshal([]byte(raw), &tracks) // a damaged list reads as empty
		p.Count = len(tracks)
		for _, t := range tracks {
			p.Seconds += t.DurationSec
		}
		p.Covers = Covers(tracks)
		out = append(out, p)
	}
	return out, rows.Err()
}

// LoadPlaylist reads one playlist with its songs.
func (d *DB) LoadPlaylist(id int64) (Playlist, error) {
	p := Playlist{ID: id}
	var raw string
	if err := d.QueryRow(`SELECT name, tracks FROM playlists WHERE id = ?`, id).Scan(&p.Name, &raw); err != nil {
		return p, fmt.Errorf("load playlist %d: %w", id, err)
	}
	if err := json.Unmarshal([]byte(raw), &p.Tracks); err != nil {
		return p, fmt.Errorf("load playlist %d: parse tracks: %w", id, err)
	}
	if p.Tracks == nil {
		p.Tracks = []queue.Track{}
	}
	return p, nil
}

// SetPlaylistTracks replaces a playlist's songs: after adding, removing
// or moving one.
func (d *DB) SetPlaylistTracks(id int64, tracks []queue.Track) error {
	raw, err := encodeTracks(tracks)
	if err != nil {
		return fmt.Errorf("set playlist tracks: %w", err)
	}
	return d.touchPlaylist(id, `tracks = ?`, raw)
}

// RenamePlaylist gives a playlist a new name.
func (d *DB) RenamePlaylist(id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("rename playlist: no name")
	}
	return d.touchPlaylist(id, `name = ?`, name)
}

// DeletePlaylist deletes a playlist. Its songs stay wherever else they
// are.
func (d *DB) DeletePlaylist(id int64) error {
	if _, err := d.Exec(`DELETE FROM playlists WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete playlist: %w", err)
	}
	return nil
}

func (d *DB) touchPlaylist(id int64, set string, value any) error {
	res, err := d.Exec(`UPDATE playlists SET `+set+`, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), touched = `+nextTouch+` WHERE id = ?`, value, id)
	if err != nil {
		return fmt.Errorf("playlist %d: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("playlist %d: no such playlist", id)
	}
	return nil
}

// nextTouch numbers each change, so "changed last" is exact even when
// two changes share a millisecond.
const nextTouch = `(SELECT COALESCE(MAX(touched), 0) + 1 FROM playlists)`

func encodeTracks(tracks []queue.Track) (string, error) {
	if tracks == nil {
		tracks = []queue.Track{}
	}
	raw, err := json.Marshal(tracks)
	return string(raw), err
}

// Covers is what a playlist of your own shows for a picture, having
// none of its own: up to four different covers of its songs, first
// first. The apps draw four as a grid, fewer as the first alone, and
// none as no picture at all.
func Covers(tracks []queue.Track) []string {
	out := []string{}
	for _, t := range tracks {
		if t.CoverURL != "" && !slices.Contains(out, t.CoverURL) {
			out = append(out, t.CoverURL)
			if len(out) == 4 {
				break
			}
		}
	}
	return out
}

// AppendNew puts tracks at the end of have, leaving out those it has
// already, and says how many went in: adding to a playlist, in both
// apps.
func AppendNew(have, tracks []queue.Track) ([]queue.Track, int) {
	seen := map[string]bool{}
	for _, t := range have {
		seen[t.ID] = true
	}
	added := 0
	for _, t := range tracks {
		if !seen[t.ID] {
			seen[t.ID] = true
			have = append(have, t)
			added++
		}
	}
	return have, added
}
