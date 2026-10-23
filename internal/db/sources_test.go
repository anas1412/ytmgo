package db

import "testing"

// Newest first; playing one again moves it to the front, updated.
func TestRecentSources(t *testing.T) {
	d := openTestDB(t)
	for _, s := range []Source{
		{Kind: "album", ID: "MPREb_a", Title: "A"},
		{Kind: "playlist", ID: "VLb", Title: "B"},
		{Kind: "album", ID: "MPREb_a", Title: "A, again"},
	} {
		if err := d.RecordSource(s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := d.LoadSources(10)
	if err != nil || len(got) != 2 || got[0].ID != "MPREb_a" || got[0].Title != "A, again" || got[1].ID != "VLb" {
		t.Fatalf("%+v, %v", got, err)
	}
	if err := d.RecordSource(Source{Kind: "album"}); err == nil {
		t.Error("a source with no id was kept")
	}
}

// Only the newest sourcesKept are kept.
func TestRecentSourcesTrimmed(t *testing.T) {
	d := openTestDB(t)
	for i := range sourcesKept + 5 {
		if err := d.RecordSource(Source{Kind: "album", ID: string(rune('a'+i%26)) + string(rune('a'+i/26))}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := d.LoadSources(1000)
	if len(got) != sourcesKept {
		t.Errorf("kept %d, want %d", len(got), sourcesKept)
	}
}
