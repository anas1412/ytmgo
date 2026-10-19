package downloader

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AlbumDir is the folder an album downloads into, under parent:
// "Artist - Album", or the album alone when the artist is unknown, with
// the characters a folder name cannot hold replaced.
func AlbumDir(parent, artist, album string) string {
	folder := album
	if artist != "" {
		folder = artist + " - " + album
	}
	return filepath.Join(parent, safeSegment(folder))
}

// AlbumStem names track n of count in an album's folder, "01 - Title",
// numbered to the width of the count so the folder sorts in album
// order: 9 tracks give "1", 10 or more "01".
func AlbumStem(n, count int, title string) string {
	width := len(fmt.Sprint(count))
	return fmt.Sprintf("%0*d - %s", width, n, title)
}

// safeSegment strips the characters awkward or forbidden in a folder
// name on Windows, macOS or Linux.
func safeSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
