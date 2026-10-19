package gui

import (
	"errors"
	"strings"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
)

// PageTrack is a song on an album or artist page: a queue track, plus
// the play count YouTube Music shows there. It goes into the queue as it
// is; the count is left behind.
type PageTrack struct {
	queue.Track
	Plays string `json:"plays"`
}

func pageTracks(results []search.Result) []PageTrack {
	out := make([]PageTrack, len(results))
	for i, r := range results {
		plays := r.Plays
		// An album row's count reads "4.2M plays"; the number is enough.
		plays = strings.TrimSuffix(plays, " plays")
		out[i] = PageTrack{Track: r.ToTrack(), Plays: plays}
	}
	return out
}

// AlbumPage is an album: its header and its tracks, in album order.
type AlbumPage struct {
	BrowseID       string      `json:"browseID"`
	Title          string      `json:"title"`
	Artist         string      `json:"artist"`
	ArtistBrowseID string      `json:"artistBrowseID"`
	Year           string      `json:"year"`
	CoverURL       string      `json:"coverURL"`
	Seconds        int         `json:"seconds"` // the whole album
	Tracks         []PageTrack `json:"tracks"`
}

// Album opens an album page, as the TUI's a does.
func (a *App) Album(browseID string) (AlbumPage, error) {
	if browseID == "" {
		return AlbumPage{}, problem(errNoAlbum)
	}
	full, err := a.album(browseID)
	if err != nil {
		return AlbumPage{}, problem(err)
	}
	p := AlbumPage{
		BrowseID: full.BrowseID, Title: full.Title, Artist: full.Artist,
		ArtistBrowseID: full.ArtistBrowseID, Year: full.Year, CoverURL: full.CoverURL,
		Tracks: pageTracks(search.AlbumResults(full)),
	}
	for _, t := range full.Tracks {
		p.Seconds += t.Duration
	}
	return p, nil
}

// Release is one album or single in an artist's discography.
type Release struct {
	BrowseID string `json:"browseID"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Year     string `json:"year"`
	CoverURL string `json:"coverURL"`
}

// ArtistView is an artist's page: who they are, their most played
// songs, and what they have released (albums, then singles and EPs).
type ArtistView struct {
	BrowseID    string      `json:"browseID"`
	Name        string      `json:"name"`
	Subscribers string      `json:"subscribers"`
	PhotoURL    string      `json:"photoURL"`
	Songs       []PageTrack `json:"songs"`
	Releases    []Release   `json:"releases"`
}

// Artist opens an artist page, as the TUI's A does.
func (a *App) Artist(browseID string) (ArtistView, error) {
	if browseID == "" {
		return ArtistView{}, problem(errNoArtist)
	}
	page, err := a.artist(browseID)
	if err != nil {
		return ArtistView{}, problem(err)
	}
	v := ArtistView{
		BrowseID: page.BrowseID, Name: page.Name, Subscribers: page.Subscribers,
		PhotoURL: page.ThumbURL, Songs: pageTracks(search.ArtistSongs(page)), Releases: []Release{},
	}
	for _, al := range page.Albums {
		v.Releases = append(v.Releases, Release{BrowseID: al.BrowseID, Title: al.Title, Artist: al.Artist, Year: al.Year, CoverURL: al.CoverURL})
	}
	return v, nil
}

// ArtistOfAlbum finds the artist a song belongs to through its album,
// for songs saved before they carried an artist id: the album page's
// byline links to the artist, as the TUI goes about it.
func (a *App) ArtistOfAlbum(albumBrowseID string) (ArtistView, error) {
	if albumBrowseID == "" {
		return ArtistView{}, problem(errNoArtist)
	}
	full, err := a.album(albumBrowseID)
	if err != nil {
		return ArtistView{}, problem(err)
	}
	if full.ArtistBrowseID == "" {
		return ArtistView{}, problem(errNoArtist)
	}
	return a.Artist(full.ArtistBrowseID)
}

var (
	errNoAlbum  = errors.New("no album page for this song")
	errNoArtist = errors.New("no artist page for this song")
)
