package gui

import (
	"errors"
	"testing"

	"ytmgo/internal/lyrics"
	"ytmgo/internal/queue"
)

// lyricsApp has a cache in a map and a fetch that answers with l, err.
func lyricsApp(l *lyrics.Lyrics, err error) (*App, map[string]string, *int) {
	cache := map[string]string{}
	fetches := 0
	a := &App{
		cachedLyrics: func(id string) (string, bool, bool, error) {
			text, ok := cache[id]
			return text, true, ok, nil
		},
		cacheLyrics: func(id, text string, _ bool) error { cache[id] = text; return nil },
		fetchLyrics: func(string, string, string, int) (*lyrics.Lyrics, error) { fetches++; return l, err },
	}
	return a, cache, &fetches
}

func TestLyricsAreCachedAndServedFromTheCache(t *testing.T) {
	a, cache, fetches := lyricsApp(lyrics.FromText("[00:01.50]one\n[00:04.00]two", true), nil)
	v, err := a.Lyrics(queue.Track{ID: "x"})
	if err != nil || !v.Found || !v.Synced || len(v.Lines) != 2 || v.Lines[0].Time != 1.5 || v.Lines[1].Text != "two" {
		t.Fatalf("got %+v, %v", v, err)
	}
	if cache["x"] == "" {
		t.Fatal("not cached")
	}
	if v, _ := a.Lyrics(queue.Track{ID: "x"}); *fetches != 1 || len(v.Lines) != 2 {
		t.Errorf("second ask fetched again (%d) or lost lines: %+v", *fetches, v)
	}
}

// A song with no lyrics is remembered as such; a failure is not, so the
// next time asks again.
func TestLyricsMissIsCachedFailureIsNot(t *testing.T) {
	a, cache, _ := lyricsApp(nil, lyrics.ErrNotFound)
	if v, err := a.Lyrics(queue.Track{ID: "none"}); err != nil || v.Found || v.Lines == nil {
		t.Errorf("miss: %+v, %v", v, err)
	}
	if _, ok := cache["none"]; !ok {
		t.Error("miss not cached")
	}
	a, cache, _ = lyricsApp(nil, errors.New("dial tcp: lookup lrclib.net: no such host"))
	if _, err := a.Lyrics(queue.Track{ID: "off"}); err == nil {
		t.Error("a failure came back as no lyrics")
	}
	if _, ok := cache["off"]; ok {
		t.Error("a failure was cached")
	}
}
