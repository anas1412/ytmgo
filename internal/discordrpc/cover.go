package discordrpc

import (
	"strings"
	"sync"

	"ytmgo/internal/ytmusic"
)

// Discord shows only pictures on the web, and a library song has none:
// its cover is inside its file. So for a song without a cover link,
// YouTube Music is asked once for the song, in the background, and the
// answer, even "none", is kept for the rest of the run.
var (
	coverMu sync.Mutex
	covers  = map[string]string{} // "artist|title" → cover link, "" for none
	asking  = map[string]bool{}

	// findCover is YouTube Music's top result for the song; tests swap it.
	findCover = func(artist, title string) (string, error) {
		found, err := ytmusic.Search(strings.TrimSpace(artist+" "+title), 1)
		if err != nil || len(found) == 0 {
			return "", err
		}
		return found[0].CoverURL, nil
	}
)

// coverFor returns the cover kept for the song and true, or, the first
// time, false at once while it asks in the background; found then gets
// the answer. An error (offline, say) keeps nothing, so the next time
// asks again.
func coverFor(artist, title string, found func(string)) (string, bool) {
	key := strings.ToLower(artist + "|" + title)
	coverMu.Lock()
	defer coverMu.Unlock()
	if c, ok := covers[key]; ok {
		return c, true
	}
	if !asking[key] {
		asking[key] = true
		go func() {
			c, err := findCover(artist, title)
			coverMu.Lock()
			delete(asking, key)
			if err == nil {
				covers[key] = c
			}
			coverMu.Unlock()
			if err == nil && c != "" {
				found(c)
			}
		}()
	}
	return "", false
}
