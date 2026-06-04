package queue

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
)

// Track represents a music track
type Track struct {
	ID          string `json:"id"`           // YouTube video ID
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Duration    string `json:"duration"`     // human readable e.g. "3:45"
	DurationSec int    `json:"duration_sec"` // seconds
	FilePath    string `json:"file_path"`    // local path once downloaded
	Downloaded  bool   `json:"downloaded"`
	URL         string `json:"url"`          // original youtube URL/query
}

// PlayURL returns the source that playback should use for this track:
// the local file path when the track is already downloaded and the
// file exists on disk, falling back to the original streaming URL
// otherwise. Centralising this logic here prevents the bug class
// where one call site uses t.URL directly and bypasses the local
// file even when t.FilePath is set.
func (t Track) PlayURL() string {
	if t.Downloaded && t.FilePath != "" {
		return t.FilePath
	}
	return t.URL
}

// Queue manages the ordered list of tracks
type Queue struct {
	mu           sync.RWMutex
	tracks       []Track
	currentIndex int // -1 means nothing playing
	shuffle      bool
	repeat       bool   // repeat current track
	repeatAll    bool   // repeat queue
	shuffleOrder []int  // shuffled indices when shuffle is on
}

func New() *Queue {
	return &Queue{currentIndex: -1}
}

func (q *Queue) Add(t Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append(q.tracks, t)
	if q.shuffle {
		q.rebuildShuffleOrder()
	}
}

func (q *Queue) AddFront(t Track) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = append([]Track{t}, q.tracks...)
	if q.currentIndex >= 0 {
		q.currentIndex++ // shift current
	}
	if q.shuffle {
		q.rebuildShuffleOrder()
	}
}

func (q *Queue) Remove(index int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.tracks) {
		return false
	}
	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)
	if q.currentIndex > index {
		q.currentIndex--
	} else if q.currentIndex == index {
		// removed current, will need to re-evaluate
		if q.currentIndex >= len(q.tracks) {
			q.currentIndex = len(q.tracks) - 1
		}
	}
	if q.shuffle {
		q.rebuildShuffleOrder()
	}
	return true
}

func (q *Queue) MoveUp(index int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index <= 0 || index >= len(q.tracks) {
		return
	}
	q.tracks[index], q.tracks[index-1] = q.tracks[index-1], q.tracks[index]
	if q.currentIndex == index {
		q.currentIndex--
	} else if q.currentIndex == index-1 {
		q.currentIndex++
	}
}

func (q *Queue) MoveDown(index int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if index < 0 || index >= len(q.tracks)-1 {
		return
	}
	q.tracks[index], q.tracks[index+1] = q.tracks[index+1], q.tracks[index]
	if q.currentIndex == index {
		q.currentIndex++
	} else if q.currentIndex == index+1 {
		q.currentIndex--
	}
}

func (q *Queue) Clear() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = nil
	q.currentIndex = -1
	q.shuffleOrder = nil
}

func (q *Queue) Tracks() []Track {
	q.mu.RLock()
	defer q.mu.RUnlock()
	cp := make([]Track, len(q.tracks))
	copy(cp, q.tracks)
	return cp
}

func (q *Queue) Current() (Track, bool) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if q.currentIndex < 0 || q.currentIndex >= len(q.tracks) {
		return Track{}, false
	}
	return q.tracks[q.currentIndex], true
}

func (q *Queue) CurrentIndex() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.currentIndex
}

func (q *Queue) SetCurrentIndex(i int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if i >= 0 && i < len(q.tracks) {
		q.currentIndex = i
	}
}

// Next advances queue and returns the next track. Returns ok=false if nothing next.
func (q *Queue) Next() (Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tracks) == 0 {
		return Track{}, false
	}
	if q.repeat && q.currentIndex >= 0 {
		return q.tracks[q.currentIndex], true
	}
	if q.shuffle && len(q.shuffleOrder) > 0 {
		// find current position in shuffle order
		for si, ti := range q.shuffleOrder {
			if ti == q.currentIndex {
				next := si + 1
				if next >= len(q.shuffleOrder) {
					if q.repeatAll {
						next = 0
					} else {
						return Track{}, false
					}
				}
				q.currentIndex = q.shuffleOrder[next]
				return q.tracks[q.currentIndex], true
			}
		}
	}
	next := q.currentIndex + 1
	if next >= len(q.tracks) {
		if q.repeatAll {
			next = 0
		} else {
			return Track{}, false
		}
	}
	q.currentIndex = next
	return q.tracks[q.currentIndex], true
}

// Prev goes back one track
func (q *Queue) Prev() (Track, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.tracks) == 0 {
		return Track{}, false
	}
	prev := q.currentIndex - 1
	if prev < 0 {
		prev = 0
	}
	q.currentIndex = prev
	return q.tracks[q.currentIndex], true
}

func (q *Queue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.tracks)
}

func (q *Queue) ToggleShuffle() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.shuffle = !q.shuffle
	if q.shuffle {
		q.rebuildShuffleOrder()
	}
	return q.shuffle
}

func (q *Queue) ToggleRepeat() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.repeat = !q.repeat
	return q.repeat
}

func (q *Queue) ToggleRepeatAll() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.repeatAll = !q.repeatAll
	return q.repeatAll
}

func (q *Queue) IsShuffle() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.shuffle
}

func (q *Queue) IsRepeat() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.repeat
}

func (q *Queue) IsRepeatAll() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.repeatAll
}

func (q *Queue) UpdateTrack(id string, fn func(*Track)) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.tracks {
		if q.tracks[i].ID == id {
			fn(&q.tracks[i])
			return
		}
	}
}

// UpdateTrackByMatch applies fn to the first track whose match key
// (computed by the caller-provided keyFn) matches the target key.
// Used by the TUI layer to back-fill FilePath/Downloaded on tracks
// added from search before the library scan completed, since those
// tracks use YouTube video IDs that don't match the library's
// file-path IDs.
func (q *Queue) UpdateTrackByMatch(target string, keyFn func(Track) string, fn func(*Track)) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i := range q.tracks {
		if keyFn(q.tracks[i]) == target {
			fn(&q.tracks[i])
			return true
		}
	}
	return false
}

// ─── Queue state persistence ───────────────────────────────────────────

// queueStateJSON is the serializable form of a Queue.
type queueStateJSON struct {
	Tracks       []Track `json:"tracks"`
	CurrentIndex int     `json:"current_index"`
	Shuffle      bool    `json:"shuffle"`
	Repeat       bool    `json:"repeat"`
	RepeatAll    bool    `json:"repeat_all"`
}

// SaveState writes the queue state to a JSON file.
func (q *Queue) SaveState(path string) error {
	q.mu.RLock()
	defer q.mu.RUnlock()
	state := queueStateJSON{
		Tracks:       q.tracks,
		CurrentIndex: q.currentIndex,
		Shuffle:      q.shuffle,
		Repeat:       q.repeat,
		RepeatAll:    q.repeatAll,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding queue: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing queue: %w", err)
	}
	return nil
}

// LoadState reads the queue state from a JSON file.
// Returns ok=false and no error when the file doesn't exist (first run).
func (q *Queue) LoadState(path string) (ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // first run — silently empty
		}
		return false, fmt.Errorf("reading queue: %w", err)
	}
	var state queueStateJSON
	if err := json.Unmarshal(data, &state); err != nil {
		return false, fmt.Errorf("parsing queue: %w", err)
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = state.Tracks
	// Don't restore currentIndex — mpv isn't loaded with any track
	// after a restart. Reset to -1 so the UI doesn't show a phantom
	// "now playing" track that produces no audio.
	q.currentIndex = -1
	q.shuffle = state.Shuffle
	q.repeat = state.Repeat
	q.repeatAll = state.RepeatAll
	if q.shuffle && len(q.tracks) > 0 {
		q.rebuildShuffleOrder()
	}
	if q.tracks == nil {
		q.tracks = []Track{} // never nil
	}
	return true, nil
}

// LoadData restores the queue state from in-memory data (loaded from SQLite).
// currentIndex is always set to -1 — mpv isn't loaded after a restart, so
// we must not show a phantom "now playing" track.
func (q *Queue) LoadData(tracks []Track, shuffle, repeat, repeatAll bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.tracks = tracks
	q.currentIndex = -1
	q.shuffle = shuffle
	q.repeat = repeat
	q.repeatAll = repeatAll
	if q.shuffle && len(q.tracks) > 0 {
		q.rebuildShuffleOrder()
	}
	if q.tracks == nil {
		q.tracks = []Track{}
	}
}

func (q *Queue) rebuildShuffleOrder() {
	n := len(q.tracks)
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	rand.Shuffle(n, func(i, j int) { order[i], order[j] = order[j], order[i] })
	q.shuffleOrder = order
}
