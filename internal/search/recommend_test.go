package search

import (
	"testing"

	"ytmgo/internal/ytmusic"
)

// TestRecommendationsStayRelated: recommendations are seeded from what
// has been played and there is no filler. A trending-search fallback
// used to pad short results, which put a globally charting track at the
// end of a queue of game soundtrack — the radio was fine, it just came
// up short once the tracks already played were filtered out.
//
// Hits the real endpoints, like the ytmusic live tests.
func TestRecommendationsStayRelated(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}

	hits, err := ytmusic.Search("Beneath the Mask Persona 5", 1)
	if err != nil || len(hits) == 0 {
		t.Skipf("live search unavailable: %v", err)
	}
	seed := hits[0].VideoID

	// Autoplay's case: one track wanted, seeded with what just played.
	// The radio opens with that track's neighbours, which are exactly
	// what gets filtered out, so asking for only `limit` left nothing.
	for i := 0; i < 3; i++ {
		recs, err := FetchRecommendations(1, []string{seed})
		if err != nil {
			t.Skipf("live radio unavailable: %v", err)
		}
		if len(recs) == 0 {
			t.Fatal("autoplay got nothing from a working radio — no headroom for dedupe")
		}
		if recs[0].ID == seed {
			t.Error("recommended the seed back")
		}
	}
}

// TestNoSeedsMeansNoRecommendations: with nothing played there is
// nothing to recommend from, and the answer is none rather than
// whatever happens to be charting.
func TestNoSeedsMeansNoRecommendations(t *testing.T) {
	got, err := FetchRecommendations(10, nil)
	if err != nil {
		t.Fatalf("no seeds should not be an error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d recommendations from no seeds (first: %q) — filler is back",
			len(got), got[0].Title)
	}
}
