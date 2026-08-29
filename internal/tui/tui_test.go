package tui

import (
	"strings"
	"testing"
	"time"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"

	"github.com/charmbracelet/lipgloss"
)

// TestTruncateCJK guards the width-aware truncation: never wider than
// asked, never a broken rune (the old byte-slicing corrupted Japanese
// titles).
func TestTruncateCJK(t *testing.T) {
	s := "ラブ・ストーリーは突然に - Love Story"
	for w := 1; w <= 30; w++ {
		got := truncate(s, w)
		if lipgloss.Width(got) > w {
			t.Fatalf("truncate width %d produced width %d (%q)", w, lipgloss.Width(got), got)
		}
		if !strings.Contains(got, "�") {
			continue
		}
		t.Fatalf("truncate width %d produced replacement char: %q", w, got)
	}
	if got := truncate("short", 40); got != "short" {
		t.Fatalf("no-op truncate changed string: %q", got)
	}
	if got := truncate("anything", 0); got != "" {
		t.Fatalf("zero width = %q, want empty", got)
	}
}

func TestNormalizeForMatch(t *testing.T) {
	cases := map[string]string{
		"Queen - Topic":                      "queen",
		"Song (Official Video)":              "song",
		"  Song (Lyrics)  ":                  "song",
		"ラブ・ストーリーは突然に":                       "ラブ・ストーリーは突然に",
		"Bohemian Rhapsody (Official Audio)": "bohemian rhapsody",
	}
	for in, want := range cases {
		if got := normalizeForMatch(in); got != want {
			t.Errorf("normalizeForMatch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindLibraryMatch(t *testing.T) {
	lib := []queue.Track{
		{Title: "Song", Artist: "Queen", FilePath: "/m/Queen - Song.m4a"},
		{Title: "NoFile", Artist: "X", FilePath: ""},
	}
	got, ok := findLibraryMatch(lib, queue.Track{Title: "Song (Official Video)", Artist: "Queen - Topic"})
	if !ok || got.FilePath != "/m/Queen - Song.m4a" {
		t.Fatalf("match = %+v ok=%v, want library file", got, ok)
	}
	if _, ok := findLibraryMatch(lib, queue.Track{Title: "NoFile", Artist: "X"}); ok {
		t.Fatal("matched a library entry without a file path")
	}
}

func TestFormatTotalDuration(t *testing.T) {
	cases := map[int]string{
		0:    "0:00",
		59:   "0:59",
		65:   "1:05",
		3600: "1:00:00",
		3725: "1:02:05",
	}
	for in, want := range cases {
		if got := formatTotalDuration(in); got != want {
			t.Errorf("formatTotalDuration(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestScrollIndicator(t *testing.T) {
	if got := scrollIndicator(0, 0, 1, 5); got != "" {
		t.Fatalf("no-scroll indicator = %q, want empty", got)
	}
	got := scrollIndicator(2, 3, 4, 10)
	for _, want := range []string{"2 above", "3 below", "4/10"} {
		if !strings.Contains(got, want) {
			t.Fatalf("indicator %q missing %q", got, want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Now().UTC()
	cases := map[string]string{
		now.Add(-30 * time.Second).Format(time.RFC3339):   "just now",
		now.Add(-5 * time.Minute).Format(time.RFC3339):    "5 min ago",
		now.Add(-2 * time.Hour).Format(time.RFC3339):      "2 hours ago",
		now.Add(-3 * 24 * time.Hour).Format(time.RFC3339): "3 days ago",
		"garbage": "garbage",
	}
	for in, want := range cases {
		if got := relativeTime(in); got != want {
			t.Errorf("relativeTime(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestClearSearchRestoresRecommendations covers the clear-search flow:
// a cached batch restores instantly; with no cache a fetch is issued.
func TestClearSearchRestoresRecommendations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := InitialModel()
	m.recommendations = []search.Result{{ID: "aaaaaaaaaaa", Title: "Rec"}}
	m.showingRecommendations = false
	m.results = []search.Result{{ID: "bbbbbbbbbbb", Title: "SearchHit"}}

	if cmd := m.showRecommendations(); cmd != nil {
		t.Fatal("cached recommendations should restore without a fetch")
	}
	if !m.showingRecommendations || len(m.results) != 1 || m.results[0].Title != "Rec" {
		t.Fatalf("recommendations not restored: %+v", m.results)
	}

	m2 := InitialModel()
	m2.showingRecommendations = false
	if cmd := m2.showRecommendations(); cmd == nil {
		t.Fatal("no cache: expected a fetch command")
	}
}

// TestStaleSearchResultsDropped: a search reply that lands after the
// user cleared the search must not overwrite the recommendations.
func TestStaleSearchResultsDropped(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := InitialModel()
	m.showingRecommendations = true
	m.results = []search.Result{{ID: "aaaaaaaaaaa", Title: "Rec"}}

	nm, _ := m.handleSearchResults(SearchResultsMsg{Results: []search.Result{{ID: "bbbbbbbbbbb", Title: "Late"}}})
	m = nm.(Model)
	if len(m.results) != 1 || m.results[0].Title != "Rec" {
		t.Fatal("stale search results overwrote recommendations")
	}
}

// TestSettingDefsInvariants makes sure every settings row declares the
// handlers its kind requires, so a future row can't reintroduce the
// silent index-mismatch class of bug.
func TestSettingDefsInvariants(t *testing.T) {
	seen := map[string]bool{}
	for i, def := range settingDefs {
		if def.label == "" {
			t.Fatalf("row %d has no label", i)
		}
		if seen[def.label] {
			t.Fatalf("duplicate settings label %q", def.label)
		}
		seen[def.label] = true
		if def.value == nil || def.desc == nil {
			t.Fatalf("%s: missing value/desc renderer", def.label)
		}
		switch def.kind {
		case settingToggle, settingCycle:
			if def.activate == nil {
				t.Fatalf("%s: %v row without activate", def.label, def.kind)
			}
		case settingNumber:
			if def.adjust == nil {
				t.Fatalf("%s: number row without adjust", def.label)
			}
		case settingString:
			if def.editGet == nil || def.editSet == nil {
				t.Fatalf("%s: string row without edit handlers", def.label)
			}
		}
	}
}
