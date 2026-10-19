package ytmusic

import (
	"encoding/json"
	"testing"
)

// A card is whatever it leads to: an album page, a playlist, an artist
// or a song; a card that leads nowhere known is left out.
func TestHomeItemKinds(t *testing.T) {
	card := func(nav string) interface{} {
		var v interface{}
		json.Unmarshal([]byte(`{"title":{"runs":[{"text":"T"}]},"subtitle":{"runs":[{"text":"Song"},{"text":" • "},{"text":"Daft Punk"}]},"navigationEndpoint":`+nav+`}`), &v)
		return v
	}
	browse := func(id, pageType string) string {
		return `{"browseEndpoint":{"browseId":"` + id + `","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"` + pageType + `"}}}}`
	}
	for _, c := range []struct{ nav, kind, id string }{
		{browse("MPREb_x", "MUSIC_PAGE_TYPE_ALBUM"), "album", "MPREb_x"},
		{browse("VLRDx", "MUSIC_PAGE_TYPE_PLAYLIST"), "playlist", "VLRDx"},
		{browse("UCx", "MUSIC_PAGE_TYPE_ARTIST"), "artist", "UCx"},
		{`{"watchEndpoint":{"videoId":"abcdefghijk"}}`, "song", "abcdefghijk"},
	} {
		it, ok := homeItem(card(c.nav))
		if !ok || it.Kind != c.kind || it.ID != c.id || it.Subtitle != "Song • Daft Punk" {
			t.Errorf("%s: got %+v, %v", c.kind, it, ok)
		}
		if c.kind == "song" && it.Artist != "Daft Punk" {
			t.Errorf("song artist %q", it.Artist)
		}
	}
	if _, ok := homeItem(card(browse("x", "MUSIC_PAGE_TYPE_PODCAST_SHOW_DETAIL_PAGE"))); ok {
		t.Error("a podcast card was kept")
	}
}

// Live: the home has moods and more shelves than its first page holds.
func TestHomeFeedLive(t *testing.T) {
	if testing.Short() {
		t.Skip("calls YouTube Music")
	}
	h, err := HomeFeed("")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Moods) == 0 || len(h.Shelves) < 3 {
		t.Fatalf("%d moods, %d shelves", len(h.Moods), len(h.Shelves))
	}
	m, err := HomeFeed(h.Moods[0].Params)
	if err != nil || len(m.Shelves) == 0 {
		t.Fatalf("mood %q: %v, %d shelves", h.Moods[0].Title, err, len(m.Shelves))
	}
	t.Logf("%d moods, %d shelves; %q has %d", len(h.Moods), len(h.Shelves), h.Moods[0].Title, len(m.Shelves))
}
