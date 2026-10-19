package downloader

import (
	"path/filepath"
	"testing"
)

func TestAlbumDir(t *testing.T) {
	for _, c := range []struct{ artist, album, want string }{
		{"Mild High Club", "Skiptracing", "Mild High Club - Skiptracing"},
		{"", "Skiptracing", "Skiptracing"},
		{"AC/DC", "Who Made Who?", "AC_DC - Who Made Who_"},
	} {
		if got := AlbumDir("/music", c.artist, c.album); got != filepath.Join("/music", c.want) {
			t.Errorf("AlbumDir(%q, %q) = %q, want %q", c.artist, c.album, got, c.want)
		}
	}
}

func TestAlbumStem(t *testing.T) {
	if got := AlbumStem(3, 9, "Homage"); got != "3 - Homage" {
		t.Errorf("of 9: %q", got)
	}
	if got := AlbumStem(3, 12, "Homage"); got != "03 - Homage" {
		t.Errorf("of 12: %q", got)
	}
}

// The cases the CLI's own copy of this was tested with, before the CLI,
// the TUI and the GUI came to share it.
func TestSafeSegment(t *testing.T) {
	cases := map[string]string{
		"Mild High Club - Timeline": "Mild High Club - Timeline",
		"AC/DC - Back In Black":     "AC_DC - Back In Black",
		`Hi: "there" <x>|y*z?`:      `Hi_ _there_ _x__y_z_`,
		"  padded  ":                "padded",
		"ラブ・ストーリーは突然に":              "ラブ・ストーリーは突然に",
	}
	for in, want := range cases {
		if got := safeSegment(in); got != want {
			t.Errorf("safeSegment(%q) = %q, want %q", in, got, want)
		}
	}
}
