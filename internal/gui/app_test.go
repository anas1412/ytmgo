package gui

import (
	"testing"

	"ytmgo/internal/search"
	"ytmgo/internal/settings"
)

// fake records what App asked the search for.
type fake struct {
	query string
	limit int
	calls int
}

func (f *fake) search(query string, limit int) ([]search.Result, error) {
	f.query, f.limit = query, limit
	f.calls++
	return []search.Result{{ID: "abc", Title: "Song"}}, nil
}

func app(limit int) (*App, *fake) {
	f := &fake{}
	s := settings.Defaults()
	s.SearchLimit = limit
	return &App{settings: s, search: f.search}, f
}

func TestSearchTrimsAndUsesTheUsersLimit(t *testing.T) {
	a, f := app(7)
	got, err := a.Search("  night drive \n")
	if err != nil {
		t.Fatal(err)
	}
	if f.query != "night drive" || f.limit != 7 {
		t.Errorf("searched %q with limit %d, want %q with 7", f.query, f.limit, "night drive")
	}
	if len(got) != 1 || got[0].ID != "abc" || got[0].Title != "Song" {
		t.Errorf("got %v", got)
	}
}

// A blank box is not a search: nothing goes to YouTube Music, and the
// page gets an empty list rather than null.
func TestBlankQueryAsksNobody(t *testing.T) {
	a, f := app(7)
	got, err := a.Search("   ")
	if err != nil {
		t.Fatal(err)
	}
	if f.calls != 0 {
		t.Error("a blank query reached the search")
	}
	if got == nil {
		t.Error("got nil, want an empty list")
	}
}

// A limit of zero in an old or hand-edited database falls back to the
// default instead of asking for no results.
func TestUnsetLimitFallsBackToDefault(t *testing.T) {
	a, f := app(0)
	a.Search("x")
	if want := settings.Defaults().SearchLimit; f.limit != want {
		t.Errorf("limit %d, want the default %d", f.limit, want)
	}
}
