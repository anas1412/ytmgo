package gui

import (
	"errors"

	"ytmgo/internal/queue"
)

// QueueState is everything the page shows of the queue: the tracks,
// which one is playing (-1 for none), and the modes. Every change to the
// queue answers with the whole of it, so the page never has to keep its
// own copy in step.
type QueueState struct {
	Tracks  []queue.Track `json:"tracks"`
	Current int           `json:"current"`
	Shuffle bool          `json:"shuffle"`
	Repeat  string        `json:"repeat"` // "off", "one" or "all"
	// Next is what plays after the current track, as the TUI's ⏭ shows
	// it: none under repeat-one, at the end, or with nothing playing.
	Next *queue.Track `json:"next"`
	// Upcoming and Played are the queue as it plays (queue.Order):
	// indices into Tracks of what is to come, in play order, shuffle
	// included, and of what has played. The panel draws Current first,
	// then Upcoming, then Played, as Spotify's queue does.
	Upcoming []int `json:"upcoming"`
	Played   []int `json:"played"`
	// Landed is where the song just added went, or the first of several;
	// -1 when nothing was added.
	Landed int `json:"landed"`
	// Version counts changes. A state reaches the page two ways, as a
	// call's answer and as a "queue" event, and an event sent just
	// before a change can arrive after that change's answer; the page
	// keeps the newest and drops what is older.
	Version int `json:"version"`
}

// Step is the queue after moving to another track. Moved is false when
// there was nowhere to go: the end of the queue, with repeat off.
type Step struct {
	QueueState
	Moved bool `json:"moved"`
}

// state reads the queue. Callers hold a.mu.
func (a *App) state() QueueState {
	repeat := "off"
	switch {
	case a.q.IsRepeat():
		repeat = "one"
	case a.q.IsRepeatAll():
		repeat = "all"
	}
	s := QueueState{Tracks: a.q.Tracks(), Current: a.q.CurrentIndex(), Shuffle: a.q.IsShuffle(), Repeat: repeat, Landed: -1, Version: a.qver}
	_, s.Upcoming, s.Played = a.q.Order()
	if next, ok := a.q.PeekNext(); ok {
		s.Next = &next
	}
	return s
}

// changed saves the queue, as the TUI does after every change, and
// returns it for the page. Callers hold a.mu. A save that fails leaves
// the queue as it is in memory; the next change tries again.
func (a *App) changed() QueueState {
	a.qver++
	s := a.state()
	a.saveQueue(s.Tracks, s.Current, a.q.IsShuffle(), a.q.IsRepeat(), a.q.IsRepeatAll())
	return s
}

// Queue is the queue as it stands, for the page to draw on startup.
func (a *App) Queue() QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state()
}

// PlayNow makes t the playing track. A track already in the queue is
// jumped to rather than added twice; a new one goes straight after the
// one playing, so what was coming next still comes next.
func (a *App) PlayNow(t queue.Track) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	tracks := a.q.Tracks()
	for i, have := range tracks {
		if have.ID == t.ID {
			a.q.SetCurrentIndex(i)
			return a.changed()
		}
	}
	at := a.q.CurrentIndex() + 1
	tracks = append(tracks[:at], append([]queue.Track{t}, tracks[at:]...)...)
	a.q.LoadData(tracks, a.q.IsShuffle(), a.q.IsRepeat(), a.q.IsRepeatAll())
	a.q.SetCurrentIndex(at)
	return a.changed()
}

// Add puts t next, after the songs added before it (queue.AddNext), as
// Spotify's "Add to queue"; Landed says where it went.
func (a *App) Add(t queue.Track) QueueState {
	return a.AddAll([]queue.Track{t})
}

// AddAll puts every track next, in order, saving once: the TUI's e,
// which queues the whole list on screen. Landed is the first of them.
func (a *App) AddAll(ts []queue.Track) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.addAllLocked(ts)
}

func (a *App) addAllLocked(ts []queue.Track) QueueState {
	landed := -1
	for i, t := range ts {
		if at := a.q.AddNext(t); i == 0 {
			landed = at
		}
	}
	s := a.changed()
	s.Landed = landed
	return s
}

// PlayAll puts the tracks next and plays the first of them: an album's
// or a playlist's Play.
func (a *App) PlayAll(ts []queue.Track) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	s := a.addAllLocked(ts)
	if len(ts) == 0 {
		return s
	}
	a.q.SetCurrentIndex(s.Landed)
	landed := s.Landed
	s = a.changed()
	s.Landed = landed
	return s
}

// Jump makes the track at i the playing one.
func (a *App) Jump(i int) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.q.SetCurrentIndex(i)
	return a.changed()
}

// Move puts the track at from at position to, sliding the tracks in
// between along: dragging a row in the queue. The playing track stays
// the playing track wherever it ends up.
func (a *App) Move(from, to int) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.moveLocked(from, to) {
		return a.state()
	}
	return a.changed()
}

// moveLocked moves one track by swapping it along, which keeps the
// queue's playing index right at every step. Callers hold a.mu.
func (a *App) moveLocked(from, to int) bool {
	n := a.q.Len()
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		return false
	}
	for ; from < to; from++ {
		a.q.MoveDown(from)
	}
	for ; from > to; from-- {
		a.q.MoveUp(from)
	}
	return true
}

// Insert puts t at position at, from 0 (first) to the queue's length
// (last): a song dropped into the queue. A song already queued is moved
// there rather than added twice. What is playing keeps playing.
func (a *App) Insert(t queue.Track, at int) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	tracks := a.q.Tracks()
	at = min(max(at, 0), len(tracks))
	for i, have := range tracks {
		if have.ID == t.ID {
			// Dropped below itself, it lands one higher once it has
			// left its old place.
			if at > i {
				at--
			}
			a.moveLocked(i, min(at, len(tracks)-1))
			return a.changed()
		}
	}
	cur := a.q.CurrentIndex()
	tracks = append(tracks[:at], append([]queue.Track{t}, tracks[at:]...)...)
	a.q.LoadData(tracks, a.q.IsShuffle(), a.q.IsRepeat(), a.q.IsRepeatAll())
	if cur >= 0 {
		if at <= cur {
			cur++
		}
		a.q.SetCurrentIndex(cur)
	}
	return a.changed()
}

// Remove takes the track at i out of the queue.
func (a *App) Remove(i int) QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.q.Remove(i)
	return a.changed()
}

// Clear empties the queue.
func (a *App) Clear() QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.q.Clear()
	return a.changed()
}

// Ended moves on when a track finishes by itself. Repeat-one plays the
// same track again.
func (a *App) Ended() Step {
	return a.step(a.q.Next)
}

// Skip moves on because the user asked to. Repeat-one governs what
// happens when a track ends, not whether the user may move on, so it
// moves even then.
func (a *App) Skip() Step {
	return a.step(a.q.Skip)
}

// Back goes to the track before.
func (a *App) Back() Step {
	return a.step(a.q.Prev)
}

func (a *App) step(move func() (queue.Track, bool)) Step {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, moved := move()
	return Step{QueueState: a.changed(), Moved: moved}
}

// ToggleShuffle turns shuffle on or off.
func (a *App) ToggleShuffle() QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.q.ToggleShuffle()
	return a.changed()
}

// CycleRepeat goes off → one → all → off, as the TUI's r does.
func (a *App) CycleRepeat() QueueState {
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case a.q.IsRepeat():
		a.q.ToggleRepeat()
		a.q.ToggleRepeatAll()
	case a.q.IsRepeatAll():
		a.q.ToggleRepeatAll()
	default:
		a.q.ToggleRepeat()
	}
	return a.changed()
}

// Autoplay is whether the queue keeps going by itself when it runs out:
// the TUI's autoplay, on by default.
func (a *App) Autoplay() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.AutoplayEnabled
}

// ToggleAutoplay turns autoplay on or off and saves it, as the TUI's c
// does.
func (a *App) ToggleAutoplay() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.AutoplayEnabled = !a.settings.AutoplayEnabled
	a.saveSettings(a.settings)
	return a.settings.AutoplayEnabled
}

// AutoplayAdd appends one song related to what was played lately, for
// when the queue is about to run out. Unlike the TUI's, it skips songs
// already in the queue, so autoplay does not bring back what is there.
// Finding nothing new is an error the page can say.
func (a *App) AutoplayAdd() (QueueState, error) {
	seeds, err := a.seeds()
	if err != nil {
		return a.Queue(), problem(err)
	}
	if len(seeds) == 0 {
		return a.Queue(), problem(errNothingToGoOn)
	}
	// More than one, so there is something left once the queued songs
	// are skipped.
	results, err := a.recommend(autoplayAsk, seeds)
	if err != nil {
		return a.Queue(), problem(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	queued := map[string]bool{}
	for _, t := range a.q.Tracks() {
		queued[t.ID] = true
	}
	for _, r := range results {
		if !queued[r.ID] {
			a.q.Add(r.ToTrack())
			return a.changed(), nil
		}
	}
	return a.state(), problem(errNothingNew)
}

// autoplayAsk is how many suggestions AutoplayAdd asks for, of which it
// adds one.
const autoplayAsk = 10

var (
	errNothingToGoOn = errors.New("autoplay: nothing has been played yet to base suggestions on")
	errNothingNew    = errors.New("autoplay: every suggestion is already in the queue")
)
