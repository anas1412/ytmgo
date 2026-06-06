package tidal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a TIDAL HiFi proxy API client with automatic proxy fallback
// for search, track info, and recommendations. Streaming URL resolution
// is handled by yt-dlp (see internal/ytresolve/).
type Client struct {
	BaseURL   string
	Quality   string
	http      *http.Client
	proxyList []string // full list of known proxies for fallback
}

// New creates a new TIDAL client.
// baseURL is the proxy server URL (e.g. "https://hifi.geeked.wtf").
// quality is one of QualityLow, QualityHigh, QualityLossless, QualityHiRes,
// QualityHiResLossless. Empty quality defaults to LOSSLESS.
func New(baseURL, quality string) *Client {
	// Ensure baseURL has a scheme
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	if quality == "" {
		quality = QualityLossless
	}
	return &Client{
		BaseURL:   baseURL,
		Quality:   quality,
		proxyList: KnownProxyURLs,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SearchTracks searches TIDAL's track catalog.
// query is the search term, limit is max results (default 25), offset for pagination.
func (c *Client) SearchTracks(query string, limit, offset int) ([]TrackResult, error) {
	params := url.Values{}
	params.Set("s", query)
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	if offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", offset))
	}

	var resp SearchResponse
	if err := c.get("/search/", params, &resp); err != nil {
		return nil, fmt.Errorf("tidal search: %w", err)
	}
	return resp.Data.Items, nil
}

// GetTrackInfo returns detailed metadata for a track.
func (c *Client) GetTrackInfo(trackID int) (*TrackInfoResponse, error) {
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", trackID))
	var resp TrackInfoResponse
	if err := c.get("/info/", params, &resp); err != nil {
		return nil, fmt.Errorf("tidal track info: %w", err)
	}
	return &resp, nil
}

// FetchRecommendations returns recommended tracks seeded from listening history.
// For each seed track ID, it fetches recommendations from TIDAL's engine,
// then also collects TRACK_MIX UUIDs from those results for further exploration.
// Results are blended, deduplicated, and limited to limit entries.
func (c *Client) FetchRecommendations(limit int, historyTrackIDs []int) ([]TrackResult, error) {
	var results []TrackResult
	seen := make(map[int]bool)

	// Phase 1: Seed from listening history — fetch recommendations for recent tracks
	mixIDs := make(map[string]bool) // collect TRACK_MIX UUIDs for Phase 2
	seedsPerTrack := 5              // how many recs to fetch per seed track
	if len(historyTrackIDs) > 0 {
		seedCount := 0
		for _, trackID := range historyTrackIDs {
			if seedCount >= 5 { // limit to 5 seed tracks to avoid excessive API calls
				break
			}
			tracks, err := c.GetRecommendationsForTrack(trackID, seedsPerTrack)
			if err != nil {
				continue
			}
			seedCount++
			for _, t := range tracks {
				if seen[t.ID] {
					continue
				}
				seen[t.ID] = true
				results = append(results, t)
				// Track mixes for Phase 2
				if t.Mixes != nil && t.Mixes.TrackMix != "" {
					mixIDs[t.Mixes.TrackMix] = true
				}
				if len(results) >= limit {
					return results[:limit], nil
				}
			}
		}
	}

	// Phase 2: Explore TRACK_MIX UUIDs collected from recommendations
	for mixID := range mixIDs {
		tracks, err := c.GetMixTracks(mixID)
		if err != nil {
			continue
		}
		for _, t := range tracks {
			if seen[t.ID] {
				continue
			}
			seen[t.ID] = true
			results = append(results, t)
			if len(results) >= limit {
				return results[:limit], nil
			}
		}
	}

	// Phase 3: Fall back to a trending search if everything above fails
	if len(results) < limit {
		needed := limit - len(results)
		tracks, err := c.SearchTracks("trending", needed, 0)
		if err == nil {
			for _, t := range tracks {
				if seen[t.ID] {
					continue
				}
				results = append(results, t)
				if len(results) >= limit {
					break
				}
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// GetMixTracks returns tracks from a TIDAL mix.
func (c *Client) GetMixTracks(mixID string) ([]TrackResult, error) {
	params := url.Values{}
	params.Set("id", mixID)
	var resp MixResponse
	if err := c.get("/mix/", params, &resp); err != nil {
		return nil, fmt.Errorf("tidal mix: %w", err)
	}
	return resp.Items, nil
}

// GetRecommendationsForTrack returns track recommendations seeded from a given track ID.
// Returns up to limit similar tracks from TIDAL's recommendation engine.
func (c *Client) GetRecommendationsForTrack(trackID int, limit int) ([]TrackResult, error) {
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", trackID))
	if limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", limit))
	}
	var resp RecommendationsResponse
	if err := c.get("/recommendations/", params, &resp); err != nil {
		return nil, fmt.Errorf("tidal recommendations: %w", err)
	}
	// Extract tracks from the recommendation envelope
	var tracks []TrackResult
	for _, item := range resp.Data.Items {
		tracks = append(tracks, item.Track)
	}
	if len(tracks) > limit {
		tracks = tracks[:limit]
	}
	return tracks, nil
}

// HealthCheck verifies the proxy is reachable.
func (c *Client) HealthCheck() error {
	resp, err := c.http.Get(c.BaseURL + "/")
	if err != nil {
		return fmt.Errorf("tidal health check: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tidal health check: status %d", resp.StatusCode)
	}
	return nil
}

// --- internal helpers ---

func (c *Client) get(endpoint string, params url.Values, dest interface{}) error {
	// Try the current proxy first, then fall through to the proxy list
	proxies := []string{c.BaseURL}
	for _, p := range c.proxyList {
		if p != c.BaseURL {
			proxies = append(proxies, p)
		}
	}

	for _, proxy := range proxies {
		reqURL := strings.TrimRight(proxy, "/") + endpoint
		if len(params) > 0 {
			reqURL += "?" + params.Encode()
		}
		resp, err := c.http.Get(reqURL)
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil || resp.StatusCode != http.StatusOK {
			continue
		}
		// Success — update BaseURL to this working proxy
		c.BaseURL = proxy
		return json.Unmarshal(body, dest)
	}

	return fmt.Errorf("tidal: all proxies failed for %s", endpoint)
}
