package gui

import (
	"errors"
	"path/filepath"
	"slices"

	"ytmgo/internal/queue"
	"ytmgo/internal/ytmusic"
)

// radioSize is how many similar songs a radio asks for.
const radioSize = 25

var errNoRadio = errors.New("no similar songs found for this one")

// Radio is "Start radio": the song, then songs like it, for the page to
// play with PlayAll. It goes after what is playing rather than
// replacing the queue, so a long queue is not lost to one click.
//
// A song saved from a file has no YouTube id of its own to ask by; it
// is found first, by its artist and title.
func (a *App) Radio(t queue.Track) ([]queue.Track, error) {
	seed := t.ID
	if !ytmusic.IsVideoID(seed) {
		found, err := a.search(t.Artist+" "+t.Title, 1)
		if err != nil {
			return nil, problem(err)
		}
		if len(found) == 0 {
			return nil, problem(errNoRadio)
		}
		seed = found[0].ID
	}
	results, err := a.recommend(radioSize, []string{seed})
	if err != nil {
		return nil, problem(err)
	}
	out := []queue.Track{t}
	for _, r := range results {
		if c := r.ToTrack(); c.ID != t.ID && c.ID != seed {
			out = append(out, c)
		}
	}
	if len(out) == 1 {
		return nil, problem(errNoRadio)
	}
	return a.mark(out), nil
}

// ShowInFolder opens the folder a library song's file is in. Only a
// file in the library, as DeleteFile, so the page cannot open anything
// else.
func (a *App) ShowInFolder(path string) error {
	a.mu.Lock()
	known := slices.ContainsFunc(a.lib, func(t queue.Track) bool { return t.FilePath == path })
	a.mu.Unlock()
	if path == "" || !known {
		return problem(errNoSuchFile)
	}
	return problem(a.open(filepath.Dir(path)))
}

var errNoSuchFile = errors.New("that file is not in the library")
