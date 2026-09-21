package lastfm

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The signature is the one part of the protocol that fails silently —
// a wrong sig is just "invalid method signature" from the server — so
// pin it to a vector computed by hand: md5("api_keykmethodauth.getTokens").
func TestSign(t *testing.T) {
	got := sign(map[string]string{"method": "auth.getToken", "api_key": "k"}, "s")
	if want := "fcb68e4c03131d77e8851e889687a441"; got != want {
		t.Fatalf("sign = %s, want %s", got, want)
	}
	// format and callback are never part of the signature.
	with := sign(map[string]string{"method": "auth.getToken", "api_key": "k", "format": "json", "callback": "x"}, "s")
	if with != got {
		t.Fatal("format/callback leaked into the signature")
	}
}

// Last.fm's rule, at its edges.
func TestShouldScrobble(t *testing.T) {
	for _, tc := range []struct {
		pos, dur float64
		want     bool
		why      string
	}{
		{20, 30, false, "30s or shorter never counts"},
		{100, 200, true, "halfway counts"},
		{99, 200, false, "just under halfway does not"},
		{240, 1000, true, "four minutes counts however long the track"},
		{239, 1000, false, "just under four minutes does not"},
		{15.5, 31, true, "half of 31 is 15.5"},
	} {
		if got := ShouldScrobble(tc.pos, tc.dur); got != tc.want {
			t.Errorf("ShouldScrobble(%v, %v) = %v: %s", tc.pos, tc.dur, got, tc.why)
		}
	}
}

// Error 14 is the one a user can fix themselves, so it must come back
// as ErrNotAuthorized and not as a string to squint at.
func TestNotAuthorizedIsDistinct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":14,"message":"This token has not been authorized"}`))
	}))
	defer srv.Close()
	old := endpoint
	endpoint = srv.URL
	defer func() { endpoint = old }()

	// call refuses to run without a key; a test key is fine, the fake
	// server does not check it.
	if Configured() {
		t.Skip("real key compiled in; this test drives a fake server")
	}
	err := callWith("k", "s", "GET", map[string]string{"method": "auth.getSession", "token": "t"}, nil)
	if !errors.Is(err, ErrNotAuthorized) {
		t.Fatalf("got %v, want ErrNotAuthorized", err)
	}
}
