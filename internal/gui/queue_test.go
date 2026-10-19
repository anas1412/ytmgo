package gui

import (
	"testing"

	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
)

// queueApp is an App with an empty queue whose saves are counted
// rather than written anywhere.
func queueApp(t *testing.T) (*App, *int) {
	t.Helper()
	saves := 0
	return &App{
		settings: settings.Defaults(),
		q:        queue.New(),
		saveQueue: func([]queue.Track, int, bool, bool, bool) error {
			saves++
			return nil
		},
	}, &saves
}

func tr(id string) queue.Track { return queue.Track{ID: id, Title: id} }

func ids(s QueueState) []string {
	out := make([]string, len(s.Tracks))
	for i, t := range s.Tracks {
		out[i] = t.ID
	}
	return out
}

func eq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Playing a new track puts it straight after the one playing, so what
// was coming next still comes next; one already queued is jumped to,
// never added twice.
func TestPlayNow(t *testing.T) {
	a, saves := queueApp(t)
	a.Add(tr("a"))
	a.Add(tr("b"))
	a.Jump(0)

	s := a.PlayNow(tr("x"))
	if want := []string{"a", "x", "b"}; !eq(ids(s), want) || s.Current != 1 {
		t.Fatalf("queue %v at %d, want %v at 1", ids(s), s.Current, want)
	}
	s = a.PlayNow(tr("b"))
	if !eq(ids(s), []string{"a", "x", "b"}) || s.Current != 2 {
		t.Errorf("replaying a queued track: queue %v at %d", ids(s), s.Current)
	}
	if *saves != 5 {
		t.Errorf("%d saves, want one per change (5)", *saves)
	}
}

// Into an empty queue, or with nothing playing yet (a restored queue),
// the new track goes first and plays.
func TestPlayNowWithNothingPlaying(t *testing.T) {
	a, _ := queueApp(t)
	a.q.LoadData([]queue.Track{tr("a"), tr("b")}, false, false, false) // as restored: current -1
	s := a.PlayNow(tr("x"))
	if !eq(ids(s), []string{"x", "a", "b"}) || s.Current != 0 {
		t.Errorf("queue %v at %d, want [x a b] at 0", ids(s), s.Current)
	}
}

// Repeat-one holds a track that ends by itself, but the next button
// still moves on; at the end of the queue there is nowhere to go.
func TestEndedSkipAndTheEnd(t *testing.T) {
	a, _ := queueApp(t)
	a.Add(tr("a"))
	a.Add(tr("b"))
	a.Jump(0)

	a.CycleRepeat() // one
	if s := a.Ended(); !s.Moved || s.Current != 0 {
		t.Errorf("repeat-one ended: at %d moved %v, want 0 and true", s.Current, s.Moved)
	}
	if s := a.Skip(); !s.Moved || s.Current != 1 {
		t.Errorf("repeat-one skip: at %d moved %v, want 1 and true", s.Current, s.Moved)
	}
	a.CycleRepeat() // all
	a.CycleRepeat() // off
	if s := a.Ended(); s.Moved || s.Current != 1 {
		t.Errorf("end of the queue: at %d moved %v, want 1 and false", s.Current, s.Moved)
	}
	if s := a.Back(); !s.Moved || s.Current != 0 {
		t.Errorf("back: at %d, want 0", s.Current)
	}
}

func TestCycleRepeat(t *testing.T) {
	a, _ := queueApp(t)
	var got []string
	for i := 0; i < 4; i++ {
		got = append(got, a.CycleRepeat().Repeat)
	}
	if want := []string{"one", "all", "off", "one"}; !eq(got, want) {
		t.Errorf("repeat cycled %v, want %v", got, want)
	}
}

// Removing a track before the playing one keeps the same track playing.
func TestRemoveKeepsTheTrackPlaying(t *testing.T) {
	a, _ := queueApp(t)
	for _, id := range []string{"a", "b", "c"} {
		a.Add(tr(id))
	}
	a.Jump(2)
	s := a.Remove(0)
	if s.Tracks[s.Current].ID != "c" {
		t.Errorf("playing %q after removing an earlier track, want c", s.Tracks[s.Current].ID)
	}
}

// Queueing a whole list keeps its order and saves once, not per track.
func TestAddAllSavesOnce(t *testing.T) {
	a, saves := queueApp(t)
	s := a.AddAll([]queue.Track{tr("a"), tr("b"), tr("c")})
	if !eq(ids(s), []string{"a", "b", "c"}) || *saves != 1 {
		t.Errorf("queue %v after %d saves, want [a b c] after 1", ids(s), *saves)
	}
}

func playing(s QueueState) string {
	if s.Current < 0 {
		return ""
	}
	return s.Tracks[s.Current].ID
}

// Dragging a row moves it and slides the rest along, in either
// direction, and whatever was playing still is.
func TestMove(t *testing.T) {
	a, _ := queueApp(t)
	a.AddAll([]queue.Track{tr("a"), tr("b"), tr("c"), tr("d")})
	a.Jump(1) // b

	s := a.Move(0, 2)
	if !eq(ids(s), []string{"b", "c", "a", "d"}) || playing(s) != "b" {
		t.Errorf("down: %v playing %q", ids(s), playing(s))
	}
	s = a.Move(3, 0)
	if !eq(ids(s), []string{"d", "b", "c", "a"}) || playing(s) != "b" {
		t.Errorf("up: %v playing %q", ids(s), playing(s))
	}
	s = a.Move(1, 3) // the playing track itself
	if !eq(ids(s), []string{"d", "c", "a", "b"}) || playing(s) != "b" {
		t.Errorf("moving the playing track: %v playing %q", ids(s), playing(s))
	}
}

// A song dropped into the queue lands where it was dropped; one dropped
// above the playing track does not change what is playing.
func TestInsert(t *testing.T) {
	a, _ := queueApp(t)
	a.AddAll([]queue.Track{tr("a"), tr("b"), tr("c")})
	a.Jump(1) // b

	s := a.Insert(tr("x"), 0)
	if !eq(ids(s), []string{"x", "a", "b", "c"}) || playing(s) != "b" {
		t.Errorf("above: %v playing %q", ids(s), playing(s))
	}
	s = a.Insert(tr("y"), 99) // past the end means last
	if !eq(ids(s), []string{"x", "a", "b", "c", "y"}) {
		t.Errorf("past the end: %v", ids(s))
	}
}

// Dropping a song that is already queued moves it, never duplicates it.
func TestInsertMovesAQueuedSong(t *testing.T) {
	a, _ := queueApp(t)
	a.AddAll([]queue.Track{tr("a"), tr("b"), tr("c"), tr("d")})
	if s := a.Insert(tr("a"), 3); !eq(ids(s), []string{"b", "c", "a", "d"}) {
		t.Errorf("dropped between c and d: %v", ids(s))
	}
	if s := a.Insert(tr("d"), 0); !eq(ids(s), []string{"d", "b", "c", "a"}) {
		t.Errorf("dropped first: %v", ids(s))
	}
}

// With nothing playing (a restored queue), a drop starts nothing.
func TestInsertWithNothingPlaying(t *testing.T) {
	a, _ := queueApp(t)
	a.q.LoadData([]queue.Track{tr("a")}, false, false, false)
	if s := a.Insert(tr("x"), 0); s.Current != -1 {
		t.Errorf("a drop started track %d", s.Current)
	}
}
