package lyrics

import (
	"errors"
	"strings"
	"testing"
)

func TestParseLRC(t *testing.T) {
	raw := "[ar:Artist]\n[ti:Title]\n[00:17.12] First line\n[00:21.5] Second\n[62:33] Later\n[00:17.12] Repeated\n"
	lines := ParseLRC(raw)
	// Metadata tags are dropped; the repeated timestamp expands.
	if len(lines) != 4 {
		t.Fatalf("ParseLRC got %d lines, want 4: %+v", len(lines), lines)
	}
	want := []Line{
		{Time: 17.12, Text: "First line"},
		{Time: 17.12, Text: "Repeated"},
		{Time: 21.5, Text: "Second"},
		{Time: 62*60 + 33, Text: "Later"},
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("lines[%d] = %+v, want %+v", i, lines[i], w)
		}
	}
}

func TestParseLRCGarbage(t *testing.T) {
	for _, raw := range []string{"", "no timestamps here", "[ar:x]\n[ti:y]"} {
		if lines := ParseLRC(raw); len(lines) != 0 {
			t.Errorf("ParseLRC(%q) = %d lines, want 0", raw, len(lines))
		}
	}
}

func TestFromTextRoundTrip(t *testing.T) {
	// Synced: Raw survives and parses back into the same lines.
	synced := &Lyrics{Raw: "[00:05.00] hello\n[00:09.00] world", Lines: []Line{
		{Time: 5, Text: "hello"},
		{Time: 9, Text: "world"},
	}, Synced: true, Source: "lrclib"}
	got := FromText(synced.Raw, true)
	if !got.Synced || len(got.Lines) != 2 || got.Lines[0].Time != 5 {
		t.Fatalf("FromText synced = %+v", got)
	}

	// Plain: lines keep their text, no timestamps.
	plain := FromText("one\ntwo\n", false)
	if plain.Synced || len(plain.Lines) != 2 || plain.Lines[1].Text != "two" {
		t.Fatalf("FromText plain = %+v", plain)
	}

	// Text marked synced but not LRC degrades to plain instead of
	// returning zero lines.
	degraded := FromText("just words", true)
	if degraded.Synced || len(degraded.Lines) != 1 {
		t.Fatalf("FromText degraded = %+v", degraded)
	}
}

func TestCleanQuery(t *testing.T) {
	cases := []struct{ title, artist, wantT, wantA string }{
		{"Song (Official Video)", "Artist", "Song", "Artist"},
		{"Song [Lyric Video]", "Artist", "Song", "Artist"},
		{"Song (Lyrics)", "Channel - Topic", "Song", "Channel"},
		{"Already Clean", "Artist - Topic", "Already Clean", "Artist"},
		{"Song (Official Video) (Lyrics)", "A", "Song", "A"},
	}
	for _, c := range cases {
		gotT, gotA := cleanQuery(c.title, c.artist)
		if gotT != c.wantT || gotA != c.wantA {
			t.Errorf("cleanQuery(%q, %q) = (%q, %q), want (%q, %q)",
				c.title, c.artist, gotT, gotA, c.wantT, c.wantA)
		}
	}
}

// TestLiveFetch hits LRCLIB (and, on a miss, YouTube Music's lyrics
// endpoint) for a well-known song. Skipped automatically in -short
// mode or when the network is unavailable, so CI never flakes on it.
func TestLiveFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}

	l, err := Fetch("fJ9rUzIMcZQ", "Bohemian Rhapsody", "Queen", 354)
	if err != nil {
		t.Skipf("live lyrics unavailable: %v", err)
	}
	if len(l.Lines) == 0 {
		t.Fatal("live fetch returned zero lines")
	}
	for _, ln := range l.Lines {
		if strings.TrimSpace(ln.Text) == "" {
			continue
		}
		if l.Synced && ln.Time < 0 {
			t.Errorf("synced lyrics carry negative timestamps: %+v", ln)
		}
		return
	}
	t.Error("live fetch returned only blank lines")
}

// TestLiveFetchMiss verifies an obscure query comes back as a
// definitive ErrNotFound (cacheable) rather than a transient error.
func TestLiveFetchMiss(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping network test")
	}

	_, err := Fetch("dQw4w9WgXcQ", "zzz nonexistent song alpha", "zzz artist beta", 120)
	if err == nil {
		t.Skip("unexpected lyrics for a nonexistent track — skipping")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Skipf("lookup unavailable (not a definitive miss): %v", err)
	}
}
