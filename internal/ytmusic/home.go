package ytmusic

import (
	"strings"
)

// HomeItem is a two-row card, as YouTube Music's carousels hold them:
// an album, a playlist, a song or an artist, by Kind.
type HomeItem struct {
	Kind     string // "album", "playlist", "song" or "artist"
	ID       string // album MPREb_…, playlist VL…, artist UC…, or a song's videoId
	Title    string
	Subtitle string // "Single • Nordo", or a playlist's artists
	CoverURL string
	Artist   string // a song's artist, from the subtitle
}

// homeItem reads a two-row card. What it is comes from where it leads:
// a page type for albums, playlists and artists, a videoId for songs.
func homeItem(item interface{}) (HomeItem, bool) {
	if item == nil {
		return HomeItem{}, false
	}
	var sub []string
	runs, _ := dig(item, "subtitle", "runs").([]interface{})
	for _, r := range runs {
		sub = append(sub, digString(r, "text"))
	}
	it := HomeItem{
		Title:    digString(item, "title", "runs", 0, "text"),
		Subtitle: strings.Join(sub, ""),
		CoverURL: largestThumbnail(dig(item, "thumbnailRenderer", "musicThumbnailRenderer", "thumbnail", "thumbnails")),
	}
	nav := dig(item, "navigationEndpoint")
	if id := digString(nav, "watchEndpoint", "videoId"); id != "" {
		it.Kind, it.ID = "song", id
		// "Song • Artist • 2.1M plays" or just the artist: the artist is
		// the first run that is neither a label nor a separator.
		for _, r := range runs {
			s := strings.TrimSpace(digString(r, "text"))
			if s != "" && s != "•" && s != "Song" && s != "Video" {
				it.Artist = s
				break
			}
		}
	} else {
		it.ID = digString(nav, "browseEndpoint", "browseId")
		switch digString(nav, "browseEndpoint", "browseEndpointContextSupportedConfigs",
			"browseEndpointContextMusicConfig", "pageType") {
		case "MUSIC_PAGE_TYPE_ALBUM":
			it.Kind = "album"
		case "MUSIC_PAGE_TYPE_PLAYLIST":
			it.Kind = "playlist"
		case "MUSIC_PAGE_TYPE_ARTIST":
			it.Kind = "artist"
		}
	}
	return it, it.Kind != "" && it.ID != "" && it.Title != ""
}
