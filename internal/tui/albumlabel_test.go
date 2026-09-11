package tui

import (
	"strings"
	"testing"

	"ytmgo/internal/player"
	"ytmgo/internal/queue"
)

// The player bar's second row is the album. A play count is not one:
// rows saved before the artist page read its columns correctly carry
// "35M plays" in that field, and they live in the database, so they
// outlast the parser fix that stopped writing them.
func TestAlbumLabelRejectsCounts(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Utaisarishi Hana", "Utaisarishi Hana"},
		{"", ""},
		{"35M plays", ""},
		{"1.2B plays", ""},
		{"49M views", ""},
		{"  568K plays  ", ""},
		// Not everything with a number in it is a count.
		{"20 Jazz Funk Greats", "20 Jazz Funk Greats"},
		{"Plays Duke Ellington", "Plays Duke Ellington"},
	} {
		if got := albumLabel(tc.in); got != tc.want {
			t.Errorf("albumLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// And the bar itself shows nothing rather than the count.
func TestPlayerBarHidesACountInTheAlbumSlot(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.queue.Clear()
	m.queue.Add(queue.Track{
		ID: "65-UZY0mN3o", Title: "Velonica", Artist: "Aqua Timez", Album: "35M plays",
	})
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying

	bar := m.renderPlayerBar()
	if strings.Contains(bar, "35M plays") {
		t.Errorf("the player bar prints a play count where the album goes:\n%s", bar)
	}
	if !strings.Contains(bar, "Velonica") {
		t.Errorf("the track itself went missing:\n%s", bar)
	}

	// A real album still shows.
	m.queue.Clear()
	m.queue.Add(queue.Track{
		ID: "65-UZY0mN3o", Title: "Velonica", Artist: "Aqua Timez", Album: "Utaisarishi Hana",
	})
	m.queue.SetCurrentIndex(0)
	m.playerState = player.StatePlaying
	if bar := m.renderPlayerBar(); !strings.Contains(bar, "Utaisarishi Hana") {
		t.Errorf("a real album name is no longer shown:\n%s", bar)
	}
}
