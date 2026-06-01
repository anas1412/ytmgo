package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a reusable HTTP client for the YouTube Music InnerTube API.
// It maintains connection reuse and shared configuration.
type Client struct {
	hc      *http.Client
	baseURL string
	apiKey  string
}

// NewClient creates a new InnerTube client with sensible defaults.
func NewClient() *Client {
	return &Client{
		hc: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        maxIdleConns,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		},
		baseURL: baseURL,
		apiKey:  apiKey,
	}
}

// ─── Request helpers ──────────────────────────────────────────────────

// context creates the API context block sent with every request.
func contextBlock() map[string]interface{} {
	return map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    clientName,
				"clientVersion": clientVersion,
				"hl":            defaultHL,
				"gl":            defaultGL,
			},
		},
	}
}

// post sends a POST request to the given endpoint with the provided body
// merged with the standard API context. Returns the parsed JSON response.
func (c *Client) post(ctx context.Context, endpoint string, extraBody map[string]interface{}) (map[string]interface{}, error) {
	resp, err := c.doRequest(ctx, endpoint, extraBody)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("innertube: decode response: %w", err)
	}
	return result, nil
}

// postRaw sends a POST request and returns the raw response body ReadCloser
// for streaming JSON decoding. The caller MUST close the returned body.
func (c *Client) postRaw(ctx context.Context, endpoint string, extraBody map[string]interface{}) (io.ReadCloser, error) {
	resp, err := c.doRequest(ctx, endpoint, extraBody)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// doRequest builds, signs, and executes the InnerTube POST request.
// Returns the raw HTTP response. The caller MUST close resp.Body.
func (c *Client) doRequest(ctx context.Context, endpoint string, extraBody map[string]interface{}) (*http.Response, error) {
	body := contextBlock()
	for k, v := range extraBody {
		body[k] = v
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("innertube: marshal body: %w", err)
	}

	url := fmt.Sprintf("%s/%s?key=%s", c.baseURL, endpoint, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("innertube: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://music.youtube.com")

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("innertube: request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("innertube: %s returned %d: %s", endpoint, resp.StatusCode, string(errBody))
	}

	return resp, nil
}

// ─── JSON navigation helpers ──────────────────────────────────────────

// getIn walks a JSON object tree following the given keys.
// Returns nil if any step is missing or the wrong type.
func getIn(obj interface{}, keys ...string) interface{} {
	current := obj
	for _, key := range keys {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

// getString extracts a string from a JSON object at the given path.
func getString(obj interface{}, keys ...string) string {
	if v := getIn(obj, keys...); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getRunsText extracts the text from a "runs" array (or falls back to simpleText).
func getRunsText(obj interface{}) string {
	m, ok := obj.(map[string]interface{})
	if !ok {
		return ""
	}
	if runs, ok := m["runs"].([]interface{}); ok && len(runs) > 0 {
		if first, ok := runs[0].(map[string]interface{}); ok {
			if text, ok := first["text"].(string); ok {
				return text
			}
		}
	}
	if s, ok := m["simpleText"].(string); ok {
		return s
	}
	return ""
}
