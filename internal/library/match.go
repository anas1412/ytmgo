package library

import "strings"

// ─── Library matching ───────────────────────────────────────────────
//
// Search results and library entries describe the same song with
// different field shapes:
//
//	search:  ID=YouTube video ID  | Title=raw (e.g. "Song (Official Video)")
//	                                Artist=uploader (e.g. "Channel - Topic")
//	library: ID=file path         | Title=cleaned (e.g. "Song")
//	                                Artist=filename prefix (e.g. "Channel")
//
// YouTube IDs and file paths can never match, so library lookup is
// done via a normalized "artist|title" signature that strips the
// common YouTube decorations (" - Topic" channel suffix, "(Official
// Video)" title suffix, etc.) before comparing.

// normalizeForMatch lower-cases, trims, and strips the YouTube-specific
// decorations that differ between a search result and a library entry
// even when they refer to the same track.
func normalizeForMatch(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// YouTube auto-generated channels append " - Topic" to the channel
	// name. The library filename parser doesn't capture this suffix.
	s = strings.TrimSuffix(s, " - topic")
	// Common title decorations that the library filename parser strips
	// (see cleanTitle) but search results preserve.
	for _, suf := range []string{
		"(official music video)", "(official video)", "(official lyric video)",
		"(lyric video)", "(lyrics)", "(audio)", "(official audio)",
		"[official music video]", "[official video]", "[lyrics]",
	} {
		s = strings.TrimSuffix(s, suf)
	}
	return strings.TrimSpace(s)
}

// MatchKey returns the signature used for library lookups.
// Format: "artist|title" with both fields normalized.
func MatchKey(t Track) string {
	return normalizeForMatch(t.Artist) + "|" + normalizeForMatch(t.Title)
}

// Match searches the local library for a track whose
// signature matches t. Returns the matching library track and true,
// or a zero Track and false. Library entries with empty FilePath are
// skipped (they aren't actually playable).
func Match(library []Track, t Track) (Track, bool) {
	sig := MatchKey(t)
	for i := range library {
		lt := library[i]
		if lt.FilePath == "" {
			continue
		}
		if MatchKey(lt) == sig {
			return lt, true
		}
	}
	return Track{}, false
}
