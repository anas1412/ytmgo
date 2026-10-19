package library

import "testing"

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
	lib := []Track{
		{Title: "Song", Artist: "Queen", FilePath: "/m/Queen - Song.m4a"},
		{Title: "NoFile", Artist: "X", FilePath: ""},
	}
	got, ok := Match(lib, Track{Title: "Song (Official Video)", Artist: "Queen - Topic"})
	if !ok || got.FilePath != "/m/Queen - Song.m4a" {
		t.Fatalf("match = %+v ok=%v, want library file", got, ok)
	}
	if _, ok := Match(lib, Track{Title: "NoFile", Artist: "X"}); ok {
		t.Fatal("matched a library entry without a file path")
	}
}
