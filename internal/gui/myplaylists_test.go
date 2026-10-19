package gui

import (
	"strings"
	"testing"

	"ytmgo/internal/db"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
)

// memPlaylists is the database's playlists, in memory.
func memPlaylists() *playlistStore {
	lists := map[int64]*db.Playlist{}
	var next int64
	return &playlistStore{
		list: func() ([]db.PlaylistInfo, error) {
			out := []db.PlaylistInfo{}
			for _, p := range lists {
				out = append(out, db.PlaylistInfo{ID: p.ID, Name: p.Name, Count: len(p.Tracks)})
			}
			return out, nil
		},
		load: func(id int64) (db.Playlist, error) {
			p := *lists[id]
			p.Tracks = append([]queue.Track{}, p.Tracks...)
			return p, nil
		},
		create: func(name string, ts []queue.Track) (int64, error) {
			next++
			lists[next] = &db.Playlist{ID: next, Name: name, Tracks: append([]queue.Track{}, ts...)}
			return next, nil
		},
		set:    func(id int64, ts []queue.Track) error { lists[id].Tracks = ts; return nil },
		rename: func(id int64, name string) error { lists[id].Name = name; return nil },
		remove: func(id int64) error { delete(lists, id); return nil },
	}
}

func songIDs(p MyPlaylistPage) string {
	var out []string
	for _, t := range p.Tracks {
		out = append(out, t.ID)
	}
	return strings.Join(out, " ")
}

func TestMyPlaylists(t *testing.T) {
	a := &App{settings: settings.Defaults(), q: queue.New(), pl: memPlaylists()}
	id, err := a.CreatePlaylist("Chill", []queue.Track{{ID: "a"}, {ID: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	// Songs it has already are left out.
	if n, err := a.AddToPlaylist(id, []queue.Track{{ID: "b"}, {ID: "c"}, {ID: "c"}}); n != 1 || err != nil {
		t.Errorf("added %d, %v; want 1", n, err)
	}
	p, _ := a.MovePlaylistTrack(id, 2, 0)
	if songIDs(p) != "c a b" {
		t.Errorf("after move %q", songIDs(p))
	}
	p, _ = a.RemoveFromPlaylist(id, 1)
	if songIDs(p) != "c b" {
		t.Errorf("after remove %q", songIDs(p))
	}
	if _, err := a.RemoveFromPlaylist(id, 9); err == nil {
		t.Error("removed a song past the end")
	}
	a.RenamePlaylist(id, "Chill out")
	if list := a.MyPlaylists(); len(list) != 1 || list[0].Name != "Chill out" || list[0].Count != 2 {
		t.Errorf("list %+v", list)
	}
	a.DeletePlaylist(id)
	if len(a.MyPlaylists()) != 0 {
		t.Error("not deleted")
	}
}
