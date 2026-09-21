package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLibraryDirs(t *testing.T) {
	home, _ := os.UserHomeDir()
	s := &Settings{LibraryDirs: " ~/Music ,, /mnt/more/ , ~ "}
	got := s.ResolveLibraryDirs()
	want := []string{filepath.Join(home, "Music"), "/mnt/more", home}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dir %d = %q, want %q", i, got[i], want[i])
		}
	}
	if n := len((&Settings{}).ResolveLibraryDirs()); n != 0 {
		t.Errorf("empty setting resolved to %d dirs", n)
	}
}

// Dragging a folder onto the terminal pastes its path the way the
// terminal likes — quoted, escaped, or as a URL — and each must land as
// the plain path.
func TestResolveLibraryDirsAcceptsDroppedPaths(t *testing.T) {
	for in, want := range map[string]string{
		`'/home/me/My Music'`:        "/home/me/My Music",
		`"/home/me/My Music"`:        "/home/me/My Music",
		`/home/me/My\ Music`:         "/home/me/My Music",
		`file:///home/me/My Music`:   "/home/me/My Music",
		`  file:///home/me/Music/  `: "/home/me/Music",
	} {
		got := (&Settings{LibraryDirs: in}).ResolveLibraryDirs()
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s -> %v, want [%s]", in, got, want)
		}
	}
}
