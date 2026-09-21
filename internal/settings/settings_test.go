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
