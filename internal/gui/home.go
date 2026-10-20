package gui

import (
	"strings"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
	"ytmgo/internal/ytmusic"
)

// Card is one tile on a home shelf. Kind says what opens: "album" and
// "playlist" open a page by ID, "artist" the artist's page, and "song"
// plays Track.
type Card struct {
	Kind     string       `json:"kind"`
	ID       string       `json:"id"`
	Title    string       `json:"title"`
	Subtitle string       `json:"subtitle"`
	CoverURL string       `json:"coverURL"`
	Track    *queue.Track `json:"track"` // songs only
}

// Shelf is a titled row of cards.
type Shelf struct {
	Title     string `json:"title"`
	Strapline string `json:"strapline"`
	Cards     []Card `json:"cards"`
}

// HomeView is YouTube Music's home, as its web app shows it signed out.
type HomeView struct {
	Shelves []Shelf `json:"shelves"`
}

// Home reads YouTube Music's home feed.
func (a *App) Home() (HomeView, error) {
	h, err := a.home()
	if err != nil {
		return HomeView{}, problem(err)
	}
	v := HomeView{Shelves: []Shelf{}}
	for _, s := range h.Shelves {
		shelf := Shelf{Title: s.Title, Strapline: s.Strapline}
		for _, it := range s.Items {
			c := Card{Kind: it.Kind, ID: it.ID, Title: it.Title, Subtitle: it.Subtitle, CoverURL: it.CoverURL}
			if it.Kind == "song" {
				t := a.mark([]queue.Track{{
					ID: it.ID, Title: it.Title, Artist: it.Artist,
					URL: ytmusic.WatchURL(it.ID), CoverURL: it.CoverURL,
				}})[0]
				c.Track = &t
			}
			shelf.Cards = append(shelf.Cards, c)
		}
		v.Shelves = append(v.Shelves, shelf)
	}
	return v, nil
}

// PlaylistPage is a YouTube Music playlist opened from the home: its
// title and its songs.
type PlaylistPage struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Seconds int         `json:"seconds"`
	Tracks  []PageTrack `json:"tracks"`
}

// Playlist opens a YouTube Music playlist by its id, with or without
// the VL its page id carries.
func (a *App) Playlist(id string) (PlaylistPage, error) {
	title, results, _, err := a.playlist(search.PlaylistRef{Source: "youtube", ID: strings.TrimPrefix(id, "VL")})
	if err != nil {
		return PlaylistPage{}, problem(err)
	}
	p := PlaylistPage{ID: id, Title: title, Tracks: a.pageTracks(results)}
	for _, r := range results {
		p.Seconds += r.Duration
	}
	return p, nil
}
