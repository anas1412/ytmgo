package ytmusic

import (
	"encoding/json"
	"testing"
)

// A card is whatever it leads to: an album page, a playlist, an artist
// or a song; a card that leads nowhere known is left out.
func TestHomeItemKinds(t *testing.T) {
	card := func(nav string) any {
		var v any
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

// A release's year, whether its subtitle is "2023" or "Single • 2023".
func TestReleaseYear(t *testing.T) {
	for sub, want := range map[string]string{
		`[{"text":"2023"}]`: "2023",
		`[{"text":"Single"},{"text":" • "},{"text":"2021"}]`:          "2021",
		`[{"text":"Daft Punk"},{"text":" • "},{"text":"905M views"}]`: "",
	} {
		var v any
		json.Unmarshal([]byte(`{"subtitle":{"runs":`+sub+`}}`), &v)
		if got := releaseYear(v); got != want {
			t.Errorf("%s: %q, want %q", sub, got, want)
		}
	}
}
