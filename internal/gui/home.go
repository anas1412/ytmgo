package gui

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ytmgo/internal/db"

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

// HomeView is Home: shelves made of your own listening. YouTube
// Music's home, signed out, is regional, made for wherever the request
// comes from, so none of it is used. "Recommended for you" is the
// page's own, from Recommendations.
type HomeView struct {
	Shelves []Shelf `json:"shelves"`
}

// Home builds the shelves, in this order, leaving out any that would be
// empty:
//
//   - New releases: what the artists you follow put out this year or
//     last, newest first.
//   - Your top artists: the ones History has you playing most.
//   - More from …: the top songs of the followed artists you play most.
//   - Fans also like: artists like the ones you follow, the ones liked
//     by most of them first.
//   - Jump back in: albums and playlists played lately, past the quick
//     tiles.
//   - Liked albums.
//
// Artist pages are fetched six at a time and kept for an hour, so Home
// is quick after the first time.
func (a *App) Home() (HomeView, error) {
	var followed []db.Saved
	if a.loadSaved != nil {
		followed, _ = a.loadSaved("artist")
	}
	var plays []db.PlayHistoryEntry
	if a.plays != nil {
		plays, _ = a.plays(homeHistory, 0)
	}
	counts, top := playsByArtist(plays)

	ids := make([]string, 0, len(followed))
	for _, f := range followed {
		ids = append(ids, f.ID)
	}
	// The top artists' pages are found through a song of theirs, as Go
	// to artist does for a song that came without the id.
	found := make([]string, len(top))
	var wg sync.WaitGroup
	for i, e := range top {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if t, err := a.Locate(queue.Track{ID: e.TrackID, Title: e.Title, Artist: e.Artist}); err == nil {
				found[i] = t.ArtistBrowseID
			}
		}()
	}
	wg.Wait()
	for _, id := range found {
		if id != "" {
			ids = append(ids, id)
		}
	}
	pages := a.artistPages(ids)
	if len(pages) == 0 && len(ids) > 0 {
		// Every fetch failed: offline, most likely. The page says so
		// rather than showing only what is on disk as if that were all.
		_, err := a.artistPage(ids[0])
		return HomeView{}, problem(err)
	}

	v := HomeView{Shelves: []Shelf{}}
	add := func(title, strapline string, cards []Card) {
		if len(cards) > 0 {
			v.Shelves = append(v.Shelves, Shelf{Title: title, Strapline: strapline, Cards: cards})
		}
	}
	add("New releases", "From artists you follow", a.newReleases(followed, pages))

	isFollowed := map[string]bool{}
	for _, f := range followed {
		isFollowed[f.ID] = true
	}
	var tops []Card
	seen := map[string]bool{}
	for _, id := range ids[len(followed):] {
		if p, ok := pages[id]; ok && !seen[id] {
			seen[id] = true
			tops = append(tops, Card{Kind: "artist", ID: id, Title: p.Name, Subtitle: "Artist", CoverURL: p.ThumbURL})
		}
	}
	add("Your top artists", "From what you play most", tops)

	// The followed artists you play most, each a shelf of their songs.
	byPlays := append([]db.Saved{}, followed...)
	sort.SliceStable(byPlays, func(i, j int) bool { return counts[artistKey(byPlays[i].Title)] > counts[artistKey(byPlays[j].Title)] })
	for _, f := range byPlays[:min(moreFromShelves, len(byPlays))] {
		p, ok := pages[f.ID]
		if !ok {
			continue
		}
		var cards []Card
		songs := search.ArtistSongs(p)
		for _, r := range songs[:min(shelfSize, len(songs))] {
			t := a.mark([]queue.Track{r.ToTrack()})[0]
			cards = append(cards, Card{Kind: "song", ID: t.ID, Title: t.Title, Subtitle: t.Artist, CoverURL: t.CoverURL, Track: &t})
		}
		add("More from "+p.Name, "An artist you follow", cards)
	}

	add("Fans also like", "Like the artists you follow", similarArtists(followed, pages, isFollowed))

	if a.sources != nil {
		if src, err := a.sources(recentShown + shelfSize); err == nil && len(src) > recentShown {
			var cards []Card
			for _, s := range src[recentShown:] {
				cards = append(cards, Card{Kind: s.Kind, ID: s.ID, Title: s.Title, Subtitle: s.Subtitle, CoverURL: s.CoverURL})
			}
			add("Jump back in", "Played lately", cards)
		}
	}
	if a.loadSaved != nil {
		if liked, err := a.loadSaved("album"); err == nil {
			var cards []Card
			for _, s := range liked {
				cards = append(cards, Card{Kind: "album", ID: s.ID, Title: s.Title, Subtitle: s.Subtitle, CoverURL: s.CoverURL})
			}
			add("Liked albums", "", cards)
		}
	}
	return v, nil
}

const (
	homeHistory     = 500 // plays read for the top artists
	topArtists      = 8
	moreFromShelves = 3
	shelfSize       = 12
)

// newReleases is what the followed artists put out this year or last,
// newest first, at most a shelf of it. A release on two of their pages
// shows once.
//
// ponytail: years only, as artist pages give no dates; within a year
// the order is each page's own, newest first.
func (a *App) newReleases(followed []db.Saved, pages map[string]ytmusic.ArtistPage) []Card {
	since := strconv.Itoa(time.Now().Year() - 1)
	var rel []ytmusic.Album
	seen := map[string]bool{}
	for _, f := range followed {
		for _, al := range pages[f.ID].Albums {
			if al.Released >= since && !seen[al.BrowseID] {
				seen[al.BrowseID] = true
				rel = append(rel, al)
			}
		}
	}
	sort.SliceStable(rel, func(i, j int) bool { return rel[i].Released > rel[j].Released })
	var cards []Card
	for _, al := range rel[:min(shelfSize*2, len(rel))] {
		kind := al.Year // "Single" or "EP"; an album's is its year
		if kind == al.Released {
			kind = "Album"
		}
		cards = append(cards, Card{Kind: "album", ID: al.BrowseID, Title: al.Title, Subtitle: kind + " • " + al.Artist, CoverURL: al.CoverURL})
	}
	return cards
}

// similarArtists are the followed artists' "Fans might also like",
// those more of them list first, without the ones already followed.
func similarArtists(followed []db.Saved, pages map[string]ytmusic.ArtistPage, isFollowed map[string]bool) []Card {
	count := map[string]int{}
	var order []ytmusic.HomeItem
	for _, f := range followed {
		for _, r := range pages[f.ID].Related {
			if isFollowed[r.ID] {
				continue
			}
			if count[r.ID] == 0 {
				order = append(order, r)
			}
			count[r.ID]++
		}
	}
	sort.SliceStable(order, func(i, j int) bool { return count[order[i].ID] > count[order[j].ID] })
	var cards []Card
	for _, r := range order[:min(shelfSize, len(order))] {
		cards = append(cards, Card{Kind: "artist", ID: r.ID, Title: r.Title, Subtitle: "Artist", CoverURL: r.CoverURL})
	}
	return cards
}

// playsByArtist counts plays by artist, and picks the most played
// artists with a play of theirs each to find their page by.
func playsByArtist(plays []db.PlayHistoryEntry) (map[string]int, []db.PlayHistoryEntry) {
	counts := map[string]int{}
	first := map[string]db.PlayHistoryEntry{}
	var order []string
	for _, e := range plays {
		k := artistKey(e.Artist)
		if k == "" {
			continue
		}
		if counts[k] == 0 {
			order = append(order, k)
			first[k] = e
		}
		counts[k]++
	}
	sort.SliceStable(order, func(i, j int) bool { return counts[order[i]] > counts[order[j]] })
	var top []db.PlayHistoryEntry
	for _, k := range order {
		if len(top) == topArtists || counts[k] < 2 {
			break
		}
		top = append(top, first[k])
	}
	return counts, top
}

// artistKey is the first artist of a byline, "Daft Punk" of "Daft
// Punk, Pharrell Williams", in lower case, to count plays by.
func artistKey(byline string) string {
	for _, sep := range []string{",", " & ", " feat", " ft.", " x "} {
		if i := strings.Index(strings.ToLower(byline), sep); i > 0 {
			byline = byline[:i]
		}
	}
	return strings.ToLower(strings.TrimSpace(byline))
}

// artistPage is an artist's page, fetched at most once an hour.
func (a *App) artistPage(id string) (ytmusic.ArtistPage, error) {
	a.pagesMu.Lock()
	c, ok := a.pageCache[id]
	a.pagesMu.Unlock()
	if ok && time.Since(c.at) < time.Hour {
		return c.page, nil
	}
	p, err := a.artist(id)
	if err != nil {
		return p, err
	}
	a.pagesMu.Lock()
	if a.pageCache == nil {
		a.pageCache = map[string]cachedPage{}
	}
	a.pageCache[id] = cachedPage{page: p, at: time.Now()}
	a.pagesMu.Unlock()
	return p, nil
}

type cachedPage struct {
	page ytmusic.ArtistPage
	at   time.Time
}

// artistPages fetches pages six at a time; the ones that fail are left
// out.
func (a *App) artistPages(ids []string) map[string]ytmusic.ArtistPage {
	out := map[string]ytmusic.ArtistPage{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	// An artist can be listed twice (followed, and in the top artists);
	// fetched at the same moment, both would miss the cache.
	for _, id := range slices.Compact(slices.Sorted(slices.Values(ids))) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if p, err := a.artistPage(id); err == nil {
				mu.Lock()
				out[id] = p
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return out
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
