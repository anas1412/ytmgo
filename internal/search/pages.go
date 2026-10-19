package search

import "ytmgo/internal/ytmusic"

// AlbumResults turns a fetched album's tracks into playable results, in
// album order. A track without its own cover takes the album's, and
// every one carries the album's title and ids, so it can lead back to
// the album and on to the artist.
func AlbumResults(album ytmusic.Album) []Result {
	out := make([]Result, 0, len(album.Tracks))
	for _, t := range album.Tracks {
		cover := t.CoverURL
		if cover == "" {
			cover = album.CoverURL
		}
		out = append(out, Result{
			ID:             t.VideoID,
			Title:          t.Title,
			Uploader:       t.Artist,
			Album:          album.Title,
			Duration:       t.Duration,
			URL:            ytmusic.WatchURL(t.VideoID),
			CoverURL:       cover,
			AlbumBrowseID:  album.BrowseID,
			ArtistBrowseID: album.ArtistBrowseID,
			Plays:          t.Plays,
		})
	}
	return out
}

// ArtistSongs turns an artist page's top songs into playable results,
// most played first. A song without its own cover takes the artist's
// photo, and every one carries the artist's id.
func ArtistSongs(a ytmusic.ArtistPage) []Result {
	out := make([]Result, 0, len(a.TopSongs))
	for _, t := range a.TopSongs {
		cover := t.CoverURL
		if cover == "" {
			cover = a.ThumbURL
		}
		out = append(out, Result{
			ID:             t.VideoID,
			Title:          t.Title,
			Uploader:       t.Artist,
			Album:          t.Album,
			Duration:       t.Duration,
			URL:            ytmusic.WatchURL(t.VideoID),
			CoverURL:       cover,
			AlbumBrowseID:  t.AlbumBrowseID,
			ArtistBrowseID: a.BrowseID,
			Plays:          t.Plays,
		})
	}
	return out
}
