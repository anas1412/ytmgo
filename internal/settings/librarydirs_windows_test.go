package settings

import (
	"reflect"
	"testing"
)

// On Windows the folders come from the GUI's folder picker as native
// paths, and must come back unchanged — backslashes included, even
// before a space.
func TestResolveLibraryDirsWindows(t *testing.T) {
	s := &Settings{LibraryDirs: ` C:\Users\me\My Music ,, D:\Music\ Live `}
	want := []string{`C:\Users\me\My Music`, `D:\Music\ Live`}
	if got := s.ResolveLibraryDirs(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
