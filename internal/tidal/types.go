// Package tidal provides a client for the TIDAL HiFi proxy API.
//
// The HiFi proxy (e.g. hifi.geeked.wtf) exposes TIDAL's internal API as
// simple HTTP endpoints. This package wraps those endpoints for search,
// track metadata, streaming URLs, and recommendations.
package tidal

import (
	"fmt"
	"strings"
)

// TrackResult is a single track from a TIDAL search or mix response.
type TrackResult struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Duration   int    `json:"duration"`    // seconds
	Popularity int    `json:"popularity"`  // 0–100
	Artist     *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`           // pointer so null is valid (mix endpoint returns null)
	Artists []ArtistInfo `json:"artists,omitempty"` // fallback when artist is null
	Album struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		Cover string `json:"cover"` // UUID for cover art
	} `json:"album"`
	Mixes *TrackMixes `json:"mixes,omitempty"` // optional mix UUIDs for recommendations
}

// ArtistInfo represents an artist in the artists array (plural).
type ArtistInfo struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// TrackMixes holds mix UUIDs returned by the TIDAL proxy for track-based recommendations.
type TrackMixes struct {
	TrackMix string `json:"TRACK_MIX"` // UUID for this track's radio mix
}

// CoverURL returns the URL for the album cover art at the given size.
// The UUID stored in Album.Cover needs dashes replaced with slashes.
func (t *TrackResult) CoverURL(width, height int) string {
	if t.Album.Cover == "" {
		return ""
	}
	slashed := strings.ReplaceAll(t.Album.Cover, "-", "/")
	return fmt.Sprintf("https://resources.tidal.com/images/%s/%dx%d.jpg", slashed, width, height)
}

// ArtistName returns the artist name, checking both singular artist and plural artists array.
func (t *TrackResult) ArtistName() string {
	if t.Artist != nil {
		return t.Artist.Name
	}
	if len(t.Artists) > 0 {
		return t.Artists[0].Name
	}
	return ""
}

// SearchResponse is the top-level JSON body from GET /search/.
type SearchResponse struct {
	Version string `json:"version"`
	Data    struct {
		Limit             int           `json:"limit"`
		Offset            int           `json:"offset"`
		TotalNumberOfItems int          `json:"totalNumberOfItems"`
		Items             []TrackResult `json:"items"`
	} `json:"data"`
}

// TrackInfoResponse is the response from GET /info/.
type TrackInfoResponse struct {
	Version string `json:"version"`
	Data    struct {
		ID       int    `json:"id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"`
		Artist   struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"artist"`
		Album struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
			Cover string `json:"cover"`
		} `json:"album"`
		ISRC    string `json:"isrc"`
		Popularity int `json:"popularity"`
		TrackNumber int `json:"trackNumber"`
		VolumeNumber int `json:"volumeNumber"`
	} `json:"data"`
}

// MixResponse is the response from GET /mix/?id=...
// Returns tracks in a TIDAL-generated mix.
// Note: items are at the top level (not nested under "data").
type MixResponse struct {
	Version string        `json:"version"`
	Items   []TrackResult `json:"items"`
}

// RecommendationsResponse is the response from GET /recommendations/?id=...
// Each item wraps a track inside a "track" envelope.
type RecommendationsResponse struct {
	Version string `json:"version"`
	Data    struct {
		Limit             int                     `json:"limit"`
		Offset            int                     `json:"offset"`
		TotalNumberOfItems int                    `json:"totalNumberOfItems"`
		Items             []RecommendationItem    `json:"items"`
	} `json:"data"`
}

// RecommendationItem wraps a track with source metadata.
type RecommendationItem struct {
	Track   TrackResult `json:"track"`
	Sources []string    `json:"sources"`
}

// Quality levels supported by the TIDAL proxy API.
const (
	QualityLow           = "LOW"            // AAC 320kbps
	QualityHigh          = "HIGH"           // AAC 320kbps
	QualityLossless      = "LOSSLESS"       // FLAC 16-bit 44.1kHz
	QualityHiRes         = "HI_RES"         // FLAC 24-bit 96kHz
	QualityHiResLossless = "HI_RES_LOSSLESS" // MQA
)

// DefaultProxyURL is the default TIDAL proxy instance for search.
// All public proxies are community-run and may go down at any time.
const DefaultProxyURL = "https://eu-central.monochrome.tf"

// KnownProxyURLs is a list of publicly known TIDAL proxy instances
// used for search/info/recommendations. The proxy list is used as
// built-in fallback when the configured proxy fails.
// Note: Streaming URL resolution (/track/) is not available on any
// public proxy — playback uses yt-dlp instead (see internal/ytresolve/).
var KnownProxyURLs = []string{
	"https://eu-central.monochrome.tf",
	"https://us-west.monochrome.tf",
	"https://api.monochrome.tf",
}
