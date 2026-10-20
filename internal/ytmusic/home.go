package ytmusic

import (
	"fmt"
	"net/url"
	"strings"
)

// HomeItem is one card on a home shelf: an album, a playlist, a song or
// an artist, by Kind.
type HomeItem struct {
	Kind     string // "album", "playlist", "song" or "artist"
	ID       string // album MPREb_…, playlist VL…, artist UC…, or a song's videoId
	Title    string
	Subtitle string // "Single • Nordo", or a playlist's artists
	CoverURL string
	Artist   string // a song's artist, from the subtitle
}

// HomeShelf is one titled row of cards.
type HomeShelf struct {
	Title     string
	Strapline string // the small line above some titles, "DISCOVER THE HITS!"
	Items     []HomeItem
}

// Home is YouTube Music's home feed, as the web app shows it signed out.
type Home struct {
	Shelves []HomeShelf
}

// homePages is how many pages of shelves HomeFeed reads: the first, and
// the ones the web app loads as the page is scrolled. Three hold every
// shelf a signed-out home has.
const homePages = 3

// HomeFeed reads the home feed. Shelves come a page at a time; the
// pages after the first are continuations, which YouTube Music answers
// only for the same visitor as the first, so its visitorData goes along
// with them.
func HomeFeed() (Home, error) {
	root, err := post("browse", map[string]interface{}{"context": clientContext(), "browseId": "FEmusic_home"})
	if err != nil {
		return Home{}, err
	}
	h := Home{Shelves: homeShelves(root)}

	visitor := digString(root, "responseContext", "visitorData")
	for page := 1; page < homePages && visitor != ""; page++ {
		next := findKey(root, "nextContinuationData")
		token := digString(next, "continuation")
		if token == "" {
			break
		}
		ctx := clientContext()
		ctx["client"].(map[string]interface{})["visitorData"] = visitor
		q := url.QueryEscape(token)
		root, err = post("browse?ctoken="+q+"&continuation="+q+"&type=next&itct="+
			url.QueryEscape(digString(next, "clickTrackingParams")), map[string]interface{}{"context": ctx})
		if err != nil {
			break // the shelves already read still make a home
		}
		h.Shelves = append(h.Shelves, homeShelves(root)...)
	}
	if len(h.Shelves) == 0 {
		return h, fmt.Errorf("ytmusic home: no shelves in the response")
	}
	return h, nil
}

func homeShelves(root interface{}) []HomeShelf {
	var shelves []HomeShelf
	for _, shelf := range findAll(root, "musicCarouselShelfRenderer") {
		head := dig(shelf, "header", "musicCarouselShelfBasicHeaderRenderer")
		s := HomeShelf{
			Title:     digString(head, "title", "runs", 0, "text"),
			Strapline: digString(head, "strapline", "runs", 0, "text"),
		}
		contents, _ := dig(shelf, "contents").([]interface{})
		for _, c := range contents {
			if it, ok := homeItem(dig(c, "musicTwoRowItemRenderer")); ok {
				s.Items = append(s.Items, it)
			}
		}
		if s.Title != "" && len(s.Items) > 0 {
			shelves = append(shelves, s)
		}
	}
	return shelves
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
