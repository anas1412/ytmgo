package innertube

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
)

// Search performs a YouTube Music search for the given query.
// Returns up to limit song results. Non-song results (albums, artists,
// playlists, videos) are filtered out.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	body := map[string]interface{}{
		"query": query,
	}

	resp, err := c.post(ctx, "search", body)
	if err != nil {
		return nil, err
	}

	return parseSearchResults(resp, limit)
}

// SearchStream performs a YouTube Music search and sends Result items to ch
// incrementally as the HTTP response body is decoded. Results appear one at a
// time rather than all at once after a single batch decode.
func (c *Client) SearchStream(ctx context.Context, query string, limit int, ch chan<- Result) error {
	body := map[string]interface{}{
		"query": query,
	}

	reader, err := c.postRaw(ctx, "search", body)
	if err != nil {
		return fmt.Errorf("innertube stream: %w", err)
	}
	defer reader.Close()

	return streamParseSearchResults(reader, limit, ch)
}

// streamParseSearchResults uses json.NewDecoder to walk the response body
// token-by-token, extracting song results from the sectionListRenderer.contents
// array as each element arrives from the network. Each result is sent to ch
// immediately after its section is decoded, so the UI can display results
// incrementally instead of waiting for the entire response.
func streamParseSearchResults(r io.Reader, limit int, ch chan<- Result) error {
	dec := json.NewDecoder(r)

	// Walk to the sectionListRenderer.contents[] array, skipping everything
	// before it. The search response path is:
	//   contents → tabbedSearchResultsRenderer → tabs[0] → tabRenderer
	//   → content → sectionListRenderer → contents
	if !seekObjectKey(dec, "contents") {
		return nil
	}
	if !seekObjectKey(dec, "tabbedSearchResultsRenderer") {
		return nil
	}
	if !seekObjectKey(dec, "tabs") {
		return nil
	}
	if !seekArrayIndex(dec, 0) {
		return nil
	}
	if !seekObjectKey(dec, "tabRenderer") {
		return nil
	}
	if !seekObjectKey(dec, "content") {
		return nil
	}
	if !seekObjectKey(dec, "sectionListRenderer") {
		return nil
	}
	if !seekObjectKey(dec, "contents") {
		return nil
	}

	// Consume opening '[' of the contents array.
	_, err := dec.Token()
	if err != nil {
		return nil
	}

	// Iterate over each element in the contents array.
	sent := 0
	for dec.More() {
		if limit > 0 && sent >= limit {
			break
		}

		var section map[string]interface{}
		if err := dec.Decode(&section); err != nil {
			return nil
		}

		// Check for itemSectionRenderer (individual search results).
		if items := getIn(section, "itemSectionRenderer", "contents"); items != nil {
			if list, ok := items.([]interface{}); ok {
				for _, item := range list {
					r := extractSongResult(item)
					if r.ID != "" {
						ch <- r
						sent++
					}
					if limit > 0 && sent >= limit {
						break
					}
				}
			}
		}

		// Check for musicCardShelfRenderer (top result).
		if items := getIn(section, "musicCardShelfRenderer", "contents"); items != nil {
			if list, ok := items.([]interface{}); ok {
				for _, item := range list {
					r := extractSongResult(item)
					if r.ID != "" {
						ch <- r
						sent++
					}
					if limit > 0 && sent >= limit {
						break
					}
				}
			}
		}
	}

	return nil
}

// seekObjectKey reads tokens from dec until it finds an object with the given
// key. It skips over sibling keys and their values using json.RawMessage.
// Returns false if the key is not found or the token stream ends.
func seekObjectKey(dec *json.Decoder, target string) bool {
	// Expect opening '{'
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	if tok != json.Delim('{') {
		return false
	}

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return false
		}
		key, ok := keyTok.(string)
		if !ok {
			return false
		}
		if key == target {
			return true
		}
		// Skip this key's value entirely.
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false
		}
	}
	return false
}

// seekArrayIndex reads tokens from dec until it finds the element at the given
// index in an array. It skips earlier elements using json.RawMessage.
// Returns false if the index is out of bounds or the token stream ends.
func seekArrayIndex(dec *json.Decoder, target int) bool {
	// Expect opening '['
	tok, err := dec.Token()
	if err != nil {
		return false
	}
	if tok != json.Delim('[') {
		return false
	}

	index := 0
	for dec.More() {
		if index == target {
			return true
		}
		// Skip this element.
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return false
		}
		index++
	}
	return false
}

// parseSearchResults extracts song results from a YouTube Music search response.
//
// The WEB_REMIX client returns this structure:
//
//	contents → tabbedSearchResultsRenderer → tabs[0] → tabRenderer
//	  → content → sectionListRenderer → contents[]
//
// Each element in contents is one of:
//   - musicCardShelfRenderer   — index 0: "Top Result" with related songs
//   - itemSectionRenderer      — indices 1+: each wraps one musicResponsiveListItemRenderer
//
// Song info inside musicResponsiveListItemRenderer:
//
//	title:   flexColumns[0].text.runs[0].text
//	artist:  flexColumns[1].text.runs[2].text  (run[0]="Song", run[1]=" • ", run[2]=artist)
//	videoId: overlay.musicItemThumbnailOverlayRenderer.content.musicPlayButtonRenderer
//	           .playNavigationEndpoint.watchEndpoint.videoId
//
// Duration is NOT included in WEB_REMIX search responses.
func parseSearchResults(resp map[string]interface{}, limit int) ([]Result, error) {
	// Navigate through tabs array (getIn can't index into arrays).
	tabs := getIn(resp, "contents", "tabbedSearchResultsRenderer", "tabs")
	if tabs != nil {
		if arr, ok := tabs.([]interface{}); ok && len(arr) > 0 {
			if tab0, ok := arr[0].(map[string]interface{}); ok {
				if contents := getIn(tab0, "tabRenderer", "content", "sectionListRenderer", "contents"); contents != nil {
					return parseSections(contents, limit), nil
				}
			}
		}
	}

	// Try alternative path (no tabbed results).
	if contents := getIn(resp, "contents", "sectionListRenderer", "contents"); contents != nil {
		return parseSections(contents, limit), nil
	}
	return nil, nil
}

// parseSections extracts results from a sectionListRenderer.contents array.
func parseSections(sectionList interface{}, limit int) []Result {
	sections, ok := sectionList.([]interface{})
	if !ok {
		return nil
	}

	var results []Result
	for _, section := range sections {
		if limit > 0 && len(results) >= limit {
			break
		}

		if items := getIn(section, "itemSectionRenderer", "contents"); items != nil {
			results = appendSongResults(results, items, limit)
			continue
		}

		if items := getIn(section, "musicCardShelfRenderer", "contents"); items != nil {
			results = appendSongResults(results, items, limit)
			continue
		}
	}
	return results
}

// appendSongResults appends song results from a list of InnerTube items.
func appendSongResults(results []Result, items interface{}, limit int) []Result {
	list, ok := items.([]interface{})
	if !ok {
		return results
	}

	for _, item := range list {
		if limit > 0 && len(results) >= limit {
			break
		}

		r := extractSongResult(item)
		if r.ID != "" {
			results = append(results, r)
		}
	}

	return results
}

// extractSongResult attempts to extract a song Result from a JSON item.
// The item is expected to contain musicResponsiveListItemRenderer.
func extractSongResult(item interface{}) Result {
	renderer := getIn(item, "musicResponsiveListItemRenderer")
	if renderer == nil {
		return Result{}
	}

	title := extractSearchTitle(renderer)
	artist := extractSearchArtist(renderer)
	videoID := extractVideoID(renderer)

	if videoID == "" || title == "" {
		return Result{}
	}

	return Result{
		ID:       videoID,
		Title:    title,
		Uploader: artist,
		Duration: 0, // duration not available in WEB_REMIX search responses
		URL:      "https://www.youtube.com/watch?v=" + videoID,
	}
}

// flexColumn returns the flexColumns[index] element. Unlike getIn, this
// correctly handles the fact that flexColumns is a JSON ARRAY, not an object.
func flexColumn(renderer interface{}, index int) interface{} {
	v := getIn(renderer, "flexColumns")
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok || index >= len(arr) {
		return nil
	}
	return arr[index]
}

// extractSearchTitle reads the title from flexColumns[0].
// Tries both musicResponsiveListItemFlexColumnRenderer and any other
// column renderer type, since different search queries may return
// different column structures.
func extractSearchTitle(renderer interface{}) string {
	col := flexColumn(renderer, 0)
	if col == nil {
		return ""
	}
	return extractFlexColumnText(col)
}

// extractSearchArtist reads the artist name from flexColumns[1].
// Structure: flex[1].runs = [{"text": "Song"}, {"text": " • "}, {"text": "Artist Name"}, ...]
func extractSearchArtist(renderer interface{}) string {
	col := flexColumn(renderer, 1)
	if col == nil {
		return ""
	}
	return extractArtistFromColumn(col)
}

// extractFlexColumnText extracts the text from a flex column object,
// trying every renderer type key it finds (not just the hardcoded one).
func extractFlexColumnText(col interface{}) string {
	m, ok := col.(map[string]interface{})
	if !ok {
		return ""
	}
	// Try each renderer key in the column object.
	for _, v := range m {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if text, ok := vm["text"]; ok {
			if s := getRunsText(text); s != "" {
				return s
			}
		}
	}
	return ""
}

// extractArtistFromColumn extracts the artist name from a flex column,
// looking for the third text run across any renderer type.
func extractArtistFromColumn(col interface{}) string {
	m, ok := col.(map[string]interface{})
	if !ok {
		return ""
	}
	// Try each renderer key in the column object.
	for _, v := range m {
		vm, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		text, ok := vm["text"]
		if !ok {
			continue
		}
		tm, ok := text.(map[string]interface{})
		if !ok {
			continue
		}
		runs, ok := tm["runs"].([]interface{})
		if !ok || len(runs) < 3 {
			continue
		}
		if third, ok := runs[2].(map[string]interface{}); ok {
			if s, ok := third["text"].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// extractVideoID finds the watch endpoint video ID from a renderer.
// Tries overlay play button path first, then flex[0] navigation endpoint.
func extractVideoID(renderer interface{}) string {
	// Primary: overlay → play button → watch endpoint.
	id := getString(renderer,
		"overlay", "musicItemThumbnailOverlayRenderer",
		"content", "musicPlayButtonRenderer",
		"playNavigationEndpoint", "watchEndpoint", "videoId",
	)
	if id != "" {
		return id
	}

	// Fallback: flexColumns[0] → navigation endpoint → watch endpoint.
	if col := flexColumn(renderer, 0); col != nil {
		if m, ok := col.(map[string]interface{}); ok {
			for _, v := range m {
				if vm, ok := v.(map[string]interface{}); ok {
					if text, ok := vm["text"]; ok {
						if tm, ok := text.(map[string]interface{}); ok {
							if runs, ok := tm["runs"].([]interface{}); ok && len(runs) > 0 {
								if first, ok := runs[0].(map[string]interface{}); ok {
									if nav, ok := first["navigationEndpoint"].(map[string]interface{}); ok {
										if we, ok := nav["watchEndpoint"].(map[string]interface{}); ok {
											if s, ok := we["videoId"].(string); ok && s != "" {
												return s
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}
