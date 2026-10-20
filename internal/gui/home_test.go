package gui

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ytmgo/internal/db"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

// Home is made of what you follow and play: new releases newest first,
// once each; top artists by plays; songs of the followed artist played
// most; artists like the followed ones, the most shared first; then
// what was played lately past the tiles, and the liked albums.
func TestHomeShelves(t *testing.T) {
	now := strconv.Itoa(time.Now().Year())
	last := strconv.Itoa(time.Now().Year() - 1)
	pages := map[string]ytmusic.ArtistPage{
		"UCa": {BrowseID: "UCa", Name: "Alpha",
			Albums: []ytmusic.Album{
				{BrowseID: "MPREb_old", Title: "Old", Artist: "Alpha", Year: "2001", Released: "2001"},
				{BrowseID: "MPREb_new", Title: "New", Artist: "Alpha", Year: "Single", Released: now},
			},
			TopSongs: []ytmusic.Track{{VideoID: "aaaaaaaaaaa", Title: "Hit", Artist: "Alpha"}},
			Related:  []ytmusic.HomeItem{{Kind: "artist", ID: "UCb", Title: "Beta"}, {Kind: "artist", ID: "UCy", Title: "Y"}, {Kind: "artist", ID: "UCx", Title: "X"}},
		},
		"UCb": {BrowseID: "UCb", Name: "Beta",
			Albums: []ytmusic.Album{
				{BrowseID: "MPREb_b", Title: "Bee", Artist: "Beta", Year: last, Released: last},
				{BrowseID: "MPREb_new", Title: "New", Artist: "Alpha", Year: "Single", Released: now},
			},
			Related: []ytmusic.HomeItem{{Kind: "artist", ID: "UCx", Title: "X"}},
		},
		"UCg": {BrowseID: "UCg", Name: "Gamma", ThumbURL: "https://g"},
	}
	fetched := map[string]int{}
	var mu sync.Mutex
	play := func(id, artist string) db.PlayHistoryEntry {
		return db.PlayHistoryEntry{TrackID: id, Title: "Song " + id, Artist: artist}
	}
	a := &App{settings: settings.Defaults(), q: queue.New(),
		artist: func(id string) (ytmusic.ArtistPage, error) {
			mu.Lock()
			fetched[id]++
			mu.Unlock()
			return pages[id], nil
		},
		loadSaved: func(kind string) ([]db.Saved, error) {
			if kind == "artist" {
				return []db.Saved{{Kind: "artist", ID: "UCb", Title: "Beta"}, {Kind: "artist", ID: "UCa", Title: "Alpha"}}, nil
			}
			return []db.Saved{{Kind: "album", ID: "MPREb_liked", Title: "Liked"}}, nil
		},
		plays: func(int, int) ([]db.PlayHistoryEntry, error) {
			return []db.PlayHistoryEntry{play("g1", "Gamma"), play("a1", "Alpha, Someone"), play("g2", "Gamma"),
				play("b1", "Beta"), play("g3", "Gamma"), play("a2", "Alpha")}, nil
		},
		search: func(q string, _ int) ([]search.Result, error) {
			// Locate searches "<artist> <title>".
			id := map[string]string{"Gamma": "UCg", "Alpha, Someone": "UCa"}[strings.Split(q, " Song ")[0]]
			return []search.Result{{ID: "x", Title: "x", ArtistBrowseID: id}}, nil
		},
		sources: func(limit int) ([]db.Source, error) {
			var s []db.Source
			for i := range recentShown + 2 {
				s = append(s, db.Source{Kind: "album", ID: strconv.Itoa(i), Title: "Played " + strconv.Itoa(i)})
			}
			return s, nil
		},
	}
	v, err := a.Home()
	if err != nil {
		t.Fatal(err)
	}
	shelf := map[string][]string{}
	var titles []string
	for _, s := range v.Shelves {
		titles = append(titles, s.Title)
		for _, c := range s.Cards {
			shelf[s.Title] = append(shelf[s.Title], c.ID+" "+c.Subtitle)
		}
	}
	want := []string{"New releases", "Your top artists", "More from Alpha", "Fans also like", "Jump back in", "Liked albums"}
	if !slices.Equal(titles, want) {
		t.Fatalf("shelves %q", titles)
	}
	for name, cards := range map[string][]string{
		"New releases":     {"MPREb_new Single • Alpha", "MPREb_b Album • Beta"},
		"Your top artists": {"UCg Artist", "UCa Artist"},
		"More from Alpha":  {"aaaaaaaaaaa Alpha"},
		"Fans also like":   {"UCx Artist", "UCy Artist"},
		"Jump back in":     {"7 ", "8 "},
		"Liked albums":     {"MPREb_liked "},
	} {
		if !slices.Equal(shelf[name], cards) {
			t.Errorf("%s: %q, want %q", name, shelf[name], cards)
		}
	}
	a.Home()
	if fetched["UCa"] != 1 || fetched["UCg"] != 1 {
		t.Errorf("pages fetched %v, want each once", fetched)
	}
}

// A playlist opens by its page id, VL and all.
func TestPlaylistDropsVL(t *testing.T) {
	a := &App{settings: settings.Defaults(), playlist: func(ref search.PlaylistRef) (string, []search.Result, int, error) {
		if ref.ID != "RDCLAK5uy_x" || ref.Source != "youtube" {
			t.Errorf("ref %+v", ref)
		}
		return "Chill", []search.Result{{ID: "a", Title: "A", Duration: 60}, {ID: "b", Title: "B", Duration: 30}}, 0, nil
	}}
	p, err := a.Playlist("VLRDCLAK5uy_x")
	if err != nil || p.Title != "Chill" || len(p.Tracks) != 2 || p.Seconds != 90 {
		t.Fatalf("%+v, %v", p, err)
	}
}
