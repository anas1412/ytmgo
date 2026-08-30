package queue

import "testing"

func mkQueue(n int) *Queue {
	q := New()
	for i := 0; i < n; i++ {
		q.Add(Track{ID: string(rune('a' + i)), Title: "t"})
	}
	return q
}

func TestNextLinear(t *testing.T) {
	q := mkQueue(3)
	for i := 0; i < 3; i++ {
		tr, ok := q.Next()
		if !ok {
			t.Fatalf("Next %d returned !ok", i)
		}
		if want := string(rune('a' + i)); tr.ID != want {
			t.Fatalf("Next %d = %s, want %s", i, tr.ID, want)
		}
	}
	if _, ok := q.Next(); ok {
		t.Fatal("Next past end should return !ok")
	}
}

func TestNextRepeatOne(t *testing.T) {
	q := mkQueue(2)
	q.Next() // now at index 0
	q.ToggleRepeat()
	for i := 0; i < 3; i++ {
		tr, ok := q.Next()
		if !ok || tr.ID != "a" {
			t.Fatalf("repeat-one Next = %s ok=%v, want a", tr.ID, ok)
		}
	}
}

func TestNextRepeatAllWraps(t *testing.T) {
	q := mkQueue(2)
	q.ToggleRepeatAll()
	ids := []string{}
	for i := 0; i < 4; i++ {
		tr, ok := q.Next()
		if !ok {
			t.Fatal("repeat-all should never exhaust")
		}
		ids = append(ids, tr.ID)
	}
	want := []string{"a", "b", "a", "b"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("sequence %v, want %v", ids, want)
		}
	}
}

func TestPrevClampsAtStart(t *testing.T) {
	q := mkQueue(3)
	q.Next() // index 0
	tr, ok := q.Prev()
	if !ok || tr.ID != "a" {
		t.Fatalf("Prev at start = %s ok=%v, want a", tr.ID, ok)
	}
}

func TestRemoveAdjustsCurrent(t *testing.T) {
	q := mkQueue(3)
	q.Next()
	q.Next() // current = index 1 ("b")
	q.Remove(0)
	if idx := q.CurrentIndex(); idx != 0 {
		t.Fatalf("current after removing earlier item = %d, want 0", idx)
	}
	cur, ok := q.Current()
	if !ok || cur.ID != "b" {
		t.Fatalf("current track = %s, want b", cur.ID)
	}
}

func TestRemoveOutOfRange(t *testing.T) {
	q := mkQueue(1)
	if q.Remove(5) || q.Remove(-1) {
		t.Fatal("out-of-range Remove should return false")
	}
}

func TestMoveUpDownFollowsCurrent(t *testing.T) {
	q := mkQueue(3)
	q.Next() // current = 0 ("a")
	q.MoveDown(0)
	if idx := q.CurrentIndex(); idx != 1 {
		t.Fatalf("current after MoveDown = %d, want 1", idx)
	}
	q.MoveUp(1)
	if idx := q.CurrentIndex(); idx != 0 {
		t.Fatalf("current after MoveUp = %d, want 0", idx)
	}
}

func TestShuffleCoversAllTracksOnce(t *testing.T) {
	q := mkQueue(5)
	q.ToggleShuffle()
	q.Next()
	seen := map[string]bool{}
	cur, _ := q.Current()
	seen[cur.ID] = true
	for {
		tr, ok := q.Next()
		if !ok {
			break
		}
		if seen[tr.ID] {
			t.Fatalf("shuffle repeated track %s before exhausting", tr.ID)
		}
		seen[tr.ID] = true
	}
	if len(seen) != 5 {
		t.Fatalf("shuffle played %d unique tracks, want 5", len(seen))
	}
}

func TestIsLastTrackAndPeekNext(t *testing.T) {
	q := mkQueue(2)
	q.Next() // at "a"
	if q.IsLastTrack() {
		t.Fatal("first of two tracks reported as last")
	}
	if next, ok := q.PeekNext(); !ok || next.ID != "b" {
		t.Fatalf("PeekNext = %s ok=%v, want b", next.ID, ok)
	}
	q.Next() // at "b"
	if !q.IsLastTrack() {
		t.Fatal("last track not reported as last")
	}
	if _, ok := q.PeekNext(); ok {
		t.Fatal("PeekNext past end should return !ok")
	}
	q.ToggleRepeatAll()
	if next, ok := q.PeekNext(); !ok || next.ID != "a" {
		t.Fatalf("PeekNext with repeat-all = %s ok=%v, want a", next.ID, ok)
	}
}

func TestLoadDataResetsCurrent(t *testing.T) {
	q := New()
	q.LoadData([]Track{{ID: "x"}}, false, false, false)
	if idx := q.CurrentIndex(); idx != -1 {
		t.Fatalf("current after LoadData = %d, want -1", idx)
	}
}

func TestPlayURLPrefersLocalFile(t *testing.T) {
	tr := Track{URL: "https://example.com", FilePath: "/tmp/x.m4a", Downloaded: true}
	if got := tr.PlayURL(); got != "/tmp/x.m4a" {
		t.Fatalf("PlayURL = %s, want local path", got)
	}
	tr.Downloaded = false
	if got := tr.PlayURL(); got != "https://example.com" {
		t.Fatalf("PlayURL = %s, want URL", got)
	}
}

// TestSkipMovesUnderRepeatOne: pressing next must advance even with
// repeat-one on. Sharing Next with the end-of-track handler made the
// next key look dead for as long as repeat-one was enabled, while prev
// (which ignored repeat) kept working — the asymmetry users hit.
func TestSkipMovesUnderRepeatOne(t *testing.T) {
	q := New()
	for _, id := range []string{"a", "b", "c"} {
		q.Add(Track{ID: id})
	}
	q.SetCurrentIndex(0)
	q.ToggleRepeat()

	// End-of-track still repeats the same one.
	if got, ok := q.Next(); !ok || got.ID != "a" {
		t.Errorf("Next under repeat-one = %q/%v, want a/true", got.ID, ok)
	}
	// An explicit skip moves on.
	if got, ok := q.Skip(); !ok || got.ID != "b" {
		t.Errorf("Skip under repeat-one = %q/%v, want b/true", got.ID, ok)
	}
	if q.CurrentIndex() != 1 {
		t.Errorf("Skip left currentIndex at %d, want 1", q.CurrentIndex())
	}
	// And prev comes back to where it was: the two must mirror.
	if got, ok := q.Prev(); !ok || got.ID != "a" {
		t.Errorf("Prev = %q/%v, want a/true", got.ID, ok)
	}
}

// TestPrevFollowsShuffleOrder: prev walked the raw queue while next
// walked the shuffle order, so going back in shuffle mode landed on a
// track that had never played.
func TestPrevFollowsShuffleOrder(t *testing.T) {
	q := New()
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		q.Add(Track{ID: id})
	}
	q.SetCurrentIndex(0)
	q.ToggleShuffle()
	// Repeat-all so the walk never runs off the end of the shuffle
	// order, whichever slot the starting track landed in.
	q.ToggleRepeatAll()

	var played []string
	if cur, ok := q.Current(); ok {
		played = append(played, cur.ID)
	}
	for i := 0; i < 3; i++ {
		t2, ok := q.Skip()
		if !ok {
			t.Fatalf("Skip %d failed early", i)
		}
		played = append(played, t2.ID)
	}
	// Walking back must retrace the same path.
	for i := len(played) - 2; i >= 0; i-- {
		got, ok := q.Prev()
		if !ok {
			t.Fatalf("Prev failed at step %d", i)
		}
		if got.ID != played[i] {
			t.Fatalf("Prev returned %q, want %q (played order %v)", got.ID, played[i], played)
		}
	}
}

// TestPrevWrapsWithRepeatAll mirrors Next's wraparound.
func TestPrevWrapsWithRepeatAll(t *testing.T) {
	q := New()
	for _, id := range []string{"a", "b", "c"} {
		q.Add(Track{ID: id})
	}
	q.SetCurrentIndex(0)

	// Without repeat-all, prev at the top stays put.
	if got, _ := q.Prev(); got.ID != "a" {
		t.Errorf("Prev at top = %q, want a", got.ID)
	}
	q.ToggleRepeatAll()
	if got, ok := q.Prev(); !ok || got.ID != "c" {
		t.Errorf("Prev at top with repeat-all = %q/%v, want c/true", got.ID, ok)
	}
}
