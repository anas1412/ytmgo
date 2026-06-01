package innertube

import "context"

// Home fetches the YouTube Music home feed and extracts song recommendations.
// Since the home feed contains mixed content (carousels of albums, playlists,
// mixes, and songs), we search for music instead for consistent song results.
// This is equivalent to what yt-dlp's ytsearch5:music does.
func (c *Client) Home(ctx context.Context, limit int) ([]Result, error) {
	// Use a broad music search to get recommendations, matching current
	// yt-dlp behavior (ytsearch5:music). YouTube Music search returns
	// more relevant music results than generic YouTube search.
	//
	// For authenticated users, FEmusic_home browse would return personalized
	// recommendations, but we don't support authentication yet.
	return c.Search(ctx, "music", limit)
}

// BrowseHome fetches the YouTube Music home feed via the browse endpoint.
// This is kept as a separate function for future use (e.g., when auth is
// added) but currently unused since Search gives better standalone results.
//
//nolint:unused
func (c *Client) BrowseHome(ctx context.Context) ([]Result, error) {
	body := map[string]interface{}{
		"browseId": "FEmusic_home",
	}

	resp, err := c.post(ctx, "browse", body)
	if err != nil {
		return nil, err
	}

	return parseBrowseResults(resp)
}

// parseBrowseResults extracts song results from a YouTube Music browse response.
//
// Response structure (singleColumnBrowseResultsRenderer):
//
//	contents → singleColumnBrowseResultsRenderer → tabs[0] → tabRenderer
//	  → content → sectionListRenderer → contents[]
//
// Each section may contain:
//   - musicCarouselShelfRenderer  — horizontal scroll (albums, playlists, songs)
//   - musicShelfRenderer          — vertical list (occasionally in home)
//   - musicImmersiveCarouselShelfRenderer — hero carousel
//   - gridRenderer                — grid layout
//
// For now, only musicShelfRenderer items are parsed (song lists).
func parseBrowseResults(resp map[string]interface{}) ([]Result, error) {
	sectionList := getIn(resp,
		"contents", "singleColumnBrowseResultsRenderer",
		"tabs", "0",
		"tabRenderer", "content",
		"sectionListRenderer", "contents",
	)
	if sectionList == nil {
		return nil, nil
	}

	sections, ok := sectionList.([]interface{})
	if !ok {
		return nil, nil
	}

	var results []Result
	for _, section := range sections {
		// Look for musicShelfRenderer (song lists).
		shelf := getIn(section, "musicShelfRenderer", "contents")
		if shelf == nil {
			continue
		}

		results = appendSongResults(results, shelf, 0)
	}

	return results, nil
}
