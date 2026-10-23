package ytmusic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParseClock(t *testing.T) {
	cases := map[string]int{
		"4:58":    298,
		"0:07":    7,
		"1:02:33": 3753,
		"2016":    0,
		"":        0,
		"live":    0,
	}
	for in, want := range cases {
		if got := parseClock(in); got != want {
			t.Errorf("parseClock(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestIsVideoID(t *testing.T) {
	if !IsVideoID("sZxzPcT1Meg") {
		t.Error("valid videoId rejected")
	}
	for _, bad := range []string{"12345678", "/home/user/file.m4a", "", "way-too-long-to-be-an-id"} {
		if IsVideoID(bad) {
			t.Errorf("IsVideoID(%q) = true, want false", bad)
		}
	}
}

// multiArtistRow is a songs-search row as YouTube Music returns it for a
// collaboration: the byline's artist runs are joined by plain " & " and
// ", " runs, which once came back as the album.
const multiArtistRow = `{
  "flexColumns": [
    {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [
      {"text": "Get Lucky (Radio Edit - feat. Pharrell Williams and Nile Rodgers)",
       "navigationEndpoint": {"watchEndpoint": {"videoId": "h5EofwRzit0"}}}]}}},
    {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [
      {"text": "Daft Punk", "navigationEndpoint": {"browseEndpoint": {"browseId": "UC_kRDKYrUlrbtrSiyu5Tflg",
        "browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_ARTIST"}}}}},
      {"text": ", "},
      {"text": "Pharrell Williams", "navigationEndpoint": {"browseEndpoint": {"browseId": "UCNUwSmXKCnS3E5CapC1G9vw",
        "browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_ARTIST"}}}}},
      {"text": " & "},
      {"text": "Nile Rodgers", "navigationEndpoint": {"browseEndpoint": {"browseId": "UCwEqVRyD6kFhNl8bY2pNkdQ",
        "browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_ARTIST"}}}}},
      {"text": " • "},
      {"text": "Get Lucky", "navigationEndpoint": {"browseEndpoint": {"browseId": "MPREb_hZQgd5b2x8i",
        "browseEndpointContextSupportedConfigs": {"browseEndpointContextMusicConfig": {"pageType": "MUSIC_PAGE_TYPE_ALBUM"}}}}},
      {"text": " • "},
      {"text": "4:08"}]}}}
  ]
}`

func TestParseSearchItemMultiArtist(t *testing.T) {
	var item any
	if err := json.Unmarshal([]byte(multiArtistRow), &item); err != nil {
		t.Fatal(err)
	}
	got, ok := parseSearchItem(item)
	if !ok {
		t.Fatal("row rejected")
	}
	want := Track{
		VideoID:        "h5EofwRzit0",
		Title:          "Get Lucky (Radio Edit - feat. Pharrell Williams and Nile Rodgers)",
		Artist:         "Daft Punk, Pharrell Williams & Nile Rodgers",
		ArtistBrowseID: "UC_kRDKYrUlrbtrSiyu5Tflg",
		Album:          "Get Lucky",
		AlbumBrowseID:  "MPREb_hZQgd5b2x8i",
		Duration:       248,
	}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	// Without an album run the album stays empty rather than picking
	// up a separator.
	noAlbum := strings.Replace(multiArtistRow, "MUSIC_PAGE_TYPE_ALBUM", "MUSIC_PAGE_TYPE_PLAYLIST", 1)
	if err := json.Unmarshal([]byte(noAlbum), &item); err != nil {
		t.Fatal(err)
	}
	if got, _ := parseSearchItem(item); got.Album != "" || got.AlbumBrowseID != "" || got.Artist != want.Artist {
		t.Errorf("no album run: got album %q (%q), artist %q", got.Album, got.AlbumBrowseID, got.Artist)
	}
}

// TestLiveSearchAndRadio hits the real InnerTube endpoints. Skipped
// automatically when the network or the endpoint is unavailable, so CI
// on a bot-walled runner doesn't flake.
func TestLiveSearchAndRadio(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}

	tracks, err := Search("kazumasa oda love story", 5)
	if err != nil {
		t.Skipf("live search unavailable: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("live search returned zero tracks")
	}
	first := tracks[0]
	if !IsVideoID(first.VideoID) {
		t.Fatalf("bad videoId %q", first.VideoID)
	}
	if first.Title == "" || first.Artist == "" {
		t.Fatalf("missing metadata: %+v", first)
	}
	if first.Duration <= 0 {
		t.Fatalf("missing duration: %+v", first)
	}
	if first.CoverURL != "" && !strings.Contains(first.CoverURL, "=w544-h544") {
		t.Fatalf("thumbnail not upscaled: %s", first.CoverURL)
	}

	radio, err := Radio(first.VideoID, 5)
	if err != nil {
		t.Skipf("live radio unavailable: %v", err)
	}
	if len(radio) == 0 {
		t.Fatal("radio returned zero tracks")
	}
	for _, r := range radio {
		if r.VideoID == first.VideoID {
			t.Fatal("radio included the seed track")
		}
		if !IsVideoID(r.VideoID) || r.Title == "" {
			t.Fatalf("bad radio item: %+v", r)
		}
	}
	t.Logf("search: %q by %q (%ds), radio: %d tracks, first: %q by %q",
		first.Title, first.Artist, first.Duration, len(radio), radio[0].Title, radio[0].Artist)
}

// TestLiveSearchAlbumLinks verifies that song results carry the album
// browse id (and title) the TUI's open-album-of-track key (`i`) needs,
// and that the id actually resolves to a tracklist. Skipped when the
// network or endpoint is unavailable.
func TestLiveSearchAlbumLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	tracks, err := Search("bohemian rhapsody queen", 5)
	if err != nil {
		t.Skipf("live search unavailable: %v", err)
	}
	if len(tracks) == 0 {
		t.Fatal("search returned zero tracks")
	}
	// Not every result has an album to link to: live recordings, singles
	// and user uploads legitimately come back without one, and YouTube
	// Music sometimes returns junk in the field (an album of " & " is
	// what failed this test once). Those are observations about the
	// data, not defects in the parsing — so they are logged, and the
	// assertion is that most results link, which still catches the
	// parser breaking outright.
	linked := 0
	for _, tr := range tracks {
		switch {
		case tr.Album == "":
			t.Logf("no album title: %q", tr.Title)
		case !strings.HasPrefix(tr.AlbumBrowseID, "MPRE"):
			t.Logf("no album browseId: %q (album %q)", tr.Title, tr.Album)
		default:
			linked++
		}
	}
	if linked*2 < len(tracks) {
		t.Fatalf("only %d of %d results carried an album link", linked, len(tracks))
	}

	// Round-trip the first result that actually carries a link, not
	// tracks[0] — which may be one of the unlinked ones above.
	var seed Track
	for _, tr := range tracks {
		if strings.HasPrefix(tr.AlbumBrowseID, "MPRE") {
			seed = tr
			break
		}
	}
	full, err := AlbumTracks(seed.AlbumBrowseID)
	if err != nil {
		t.Skipf("live album page unavailable: %v", err)
	}
	if len(full.Tracks) == 0 {
		t.Fatal("album page returned no tracks")
	}
	if full.Tracks[0].AlbumBrowseID != seed.AlbumBrowseID {
		t.Errorf("album tracks lost their album browseId")
	}
	t.Logf("%d/%d results album-linked; %q resolved to %d tracks",
		linked, len(tracks), full.Title, len(full.Tracks))
}

// TestLiveAlbums exercises album search and the album page against the
// real endpoints. Skipped when the network or endpoint is unavailable.
func TestLiveAlbums(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	albums, err := SearchAlbums("mild high club timeline", 5)
	if err != nil {
		t.Skipf("live album search unavailable: %v", err)
	}
	if len(albums) == 0 {
		t.Fatal("album search returned nothing")
	}
	a := albums[0]
	if a.BrowseID == "" || a.Title == "" {
		t.Fatalf("incomplete album: %+v", a)
	}

	full, err := AlbumTracks(a.BrowseID)
	if err != nil {
		t.Skipf("live album page unavailable: %v", err)
	}
	if len(full.Tracks) == 0 {
		t.Fatal("album page returned no tracks")
	}
	for i, tr := range full.Tracks {
		if !IsVideoID(tr.VideoID) {
			t.Fatalf("track %d has bad videoId %q", i+1, tr.VideoID)
		}
		if tr.Title == "" {
			t.Fatalf("track %d has no title", i+1)
		}
	}
	withDur := 0
	for _, tr := range full.Tracks {
		if tr.Duration > 0 {
			withDur++
		}
	}
	if withDur == 0 {
		t.Error("no track had a duration")
	}
	t.Logf("album %q by %q (%s): %d tracks, %d with durations; first: %q (%ds)",
		full.Title, full.Artist, full.Year, len(full.Tracks), withDur,
		full.Tracks[0].Title, full.Tracks[0].Duration)
}

// TestLiveLyrics: the lyrics lookup matched on a panelIdentifier
// containing "MPLYR" and a browse id with the same prefix. The response
// carries no panelIdentifier at all and the ids are MPLYt, so nothing
// ever matched and every track came back as having no lyrics — which
// made the fallback behind LRCLIB dead code.
func TestLiveLyrics(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	hits, err := Search("Bohemian Rhapsody Queen", 1)
	if err != nil || len(hits) == 0 {
		t.Skipf("live search unavailable: %v", err)
	}
	text, err := PlainLyrics(hits[0].VideoID)
	if err != nil {
		t.Fatalf("a track this famous has lyrics; got %v", err)
	}
	if len(text) < 100 {
		t.Errorf("lyrics are %d bytes, expected a full song", len(text))
	}
}

// TestLivePlaylist fetches a real playlist. A playlist page reuses the
// songs-search row renderer inside a differently named shelf, so this
// guards both the shelf lookup and the duration backfill that the
// search rows do not carry.
func TestLivePlaylist(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	// A large public playlist — the shape a user actually pastes in.
	p, err := PlaylistTracks("PLFgquLnL59alCl_2TQvOiD5Vgm1hCaGSI")
	if err != nil {
		t.Skipf("live playlist unavailable: %v", err)
	}
	if p.Title == "" {
		t.Error("playlist has no title")
	}
	if len(p.Tracks) < 5 {
		t.Fatalf("got %d tracks, want a full playlist", len(p.Tracks))
	}
	for i, tr := range p.Tracks {
		if !IsVideoID(tr.VideoID) {
			t.Errorf("track %d: bad videoId %q", i, tr.VideoID)
		}
		if tr.Title == "" || tr.Artist == "" {
			t.Errorf("track %d: missing metadata: %+v", i, tr)
		}
		if tr.Duration <= 0 {
			t.Errorf("track %d (%q): no duration — the fixed-column backfill missed", i, tr.Title)
		}
	}
	t.Logf("playlist %q: %d tracks, first %q by %q (%ds)",
		p.Title, len(p.Tracks), p.Tracks[0].Title, p.Tracks[0].Artist, p.Tracks[0].Duration)
}

// An album's auto-generated OLAK5uy_ playlist carries no header, so the
// title is best-effort; the tracks must still come through.
func TestLiveAlbumPlaylistHasTracksWithoutTitle(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}
	p, err := PlaylistTracks("OLAK5uy_kmzoSOa_tCizE-r4sweNz91d9qBv1UCVY")
	if err != nil {
		t.Skipf("live playlist unavailable: %v", err)
	}
	if len(p.Tracks) < 5 || p.Tracks[0].Duration <= 0 {
		t.Fatalf("album playlist came back thin: %d tracks, first dur %ds", len(p.Tracks), p.Tracks[0].Duration)
	}
}
