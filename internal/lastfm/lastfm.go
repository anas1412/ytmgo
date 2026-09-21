// Package lastfm scrobbles to Last.fm.
//
// Two calls do the work: track.updateNowPlaying when a track starts, and
// track.scrobble once it has been heard long enough to count. Both need
// a session key, which the user grants once: ytmgo asks for a token,
// the user approves it on last.fm in a browser, and the token is traded
// for a key that never expires. No password ever passes through here.
//
// The API key and secret identify ytmgo to Last.fm. They are baked in,
// as every open-source scrobbler bakes them in; the secret only signs
// requests, and the credential that matters — the session key — is the
// user's own and stays on their machine.
package lastfm

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ytmgo's own Last.fm API account. If these are ever emptied, Configured
// reports false and the Settings row says so rather than failing later.
const (
	apiKey    = "5dccbd49d803b8805b7ad0add8579568"
	apiSecret = "22c811072e947bf9b04d56f12d6dc660"
)

// endpoint is a variable so a test can point it at a local server.
var endpoint = "https://ws.audioscrobbler.com/2.0/"

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Configured reports whether this build carries an API key at all.
func Configured() bool { return apiKey != "" && apiSecret != "" }

// ErrNotAuthorized means the user has not yet approved the token on
// last.fm — the one error worth telling apart, since the fix is to go
// and click the button.
var ErrNotAuthorized = errors.New("lastfm: token not yet approved")

// Session is what auth.getSession hands back.
type Session struct {
	User string
	Key  string
}

// Track is what a scrobble needs to know about a song.
type Track struct {
	Artist   string
	Title    string
	Album    string
	Duration int // seconds; 0 when unknown
}

// ShouldScrobble applies Last.fm's rule: a track longer than 30 seconds
// counts once it has played for half its length or four minutes,
// whichever comes first.
func ShouldScrobble(position, duration float64) bool {
	if duration <= 30 {
		return false
	}
	return position >= duration/2 || position >= 240
}

// GetToken starts the authorisation: a token the user must approve.
func GetToken() (string, error) {
	var out struct {
		Token string `json:"token"`
	}
	if err := call("GET", map[string]string{"method": "auth.getToken"}, &out); err != nil {
		return "", err
	}
	if out.Token == "" {
		return "", errors.New("lastfm: no token in response")
	}
	return out.Token, nil
}

// AuthURL is where the user approves the token.
func AuthURL(token string) string {
	return "https://www.last.fm/api/auth/?api_key=" + url.QueryEscape(apiKey) + "&token=" + url.QueryEscape(token)
}

// GetSession trades an approved token for a permanent session key.
func GetSession(token string) (Session, error) {
	var out struct {
		Session struct {
			Name string `json:"name"`
			Key  string `json:"key"`
		} `json:"session"`
	}
	if err := call("GET", map[string]string{"method": "auth.getSession", "token": token}, &out); err != nil {
		return Session{}, err
	}
	if out.Session.Key == "" {
		return Session{}, errors.New("lastfm: no session key in response")
	}
	return Session{User: out.Session.Name, Key: out.Session.Key}, nil
}

// NowPlaying tells Last.fm what is playing right now.
func NowPlaying(sessionKey string, t Track) error {
	return call("POST", trackParams("track.updateNowPlaying", sessionKey, t, time.Time{}), nil)
}

// Scrobble records a play that began at startedAt.
func Scrobble(sessionKey string, t Track, startedAt time.Time) error {
	return call("POST", trackParams("track.scrobble", sessionKey, t, startedAt), nil)
}

func trackParams(method, sessionKey string, t Track, startedAt time.Time) map[string]string {
	p := map[string]string{
		"method": method,
		"sk":     sessionKey,
		"artist": t.Artist,
		"track":  t.Title,
	}
	if t.Album != "" {
		p["album"] = t.Album
	}
	if t.Duration > 0 {
		p["duration"] = strconv.Itoa(t.Duration)
	}
	if !startedAt.IsZero() {
		p["timestamp"] = strconv.FormatInt(startedAt.Unix(), 10)
	}
	return p
}

// sign produces api_sig: every parameter sorted by name, name and value
// run together, the secret on the end, MD5'd. format and callback are
// left out, as the spec says.
func sign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "format" || k == "callback" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params[k])
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// apiError is the shape of a failure response.
type apiError struct {
	Code    int    `json:"error"`
	Message string `json:"message"`
}

// call signs and sends one request, decoding the JSON body into out
// when out is not nil.
func call(httpMethod string, params map[string]string, out interface{}) error {
	if !Configured() {
		return errors.New("lastfm: no API key in this build")
	}
	return callWith(apiKey, apiSecret, httpMethod, params, out)
}

// callWith is call with the credentials passed in, so a test can drive
// it against a local server without a real key in the build.
func callWith(key, secret, httpMethod string, params map[string]string, out interface{}) error {
	params["api_key"] = key
	params["api_sig"] = sign(params, secret)
	params["format"] = "json"

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}

	var req *http.Request
	var err error
	if httpMethod == "GET" {
		req, err = http.NewRequest("GET", endpoint+"?"+form.Encode(), nil)
	} else {
		req, err = http.NewRequest("POST", endpoint, strings.NewReader(form.Encode()))
		if req != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return fmt.Errorf("lastfm: %w", err)
	}
	req.Header.Set("User-Agent", "ytmgo (https://github.com/anas1412/ytmgo)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("lastfm: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("lastfm: %w", err)
	}

	var apiErr apiError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Code != 0 {
		if apiErr.Code == 14 {
			return ErrNotAuthorized
		}
		return fmt.Errorf("lastfm: %s (error %d)", apiErr.Message, apiErr.Code)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("lastfm: HTTP %d", resp.StatusCode)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("lastfm: decode: %w", err)
		}
	}
	return nil
}
