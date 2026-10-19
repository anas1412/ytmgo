package settings

import (
	"os"
	"testing"
)

// On Windows everything ytmgo keeps lives under %LOCALAPPDATA%, never in
// a Unix-style ~/.local/share inside the user's profile.
func TestUserDataDirIsLocalAppData(t *testing.T) {
	want := os.Getenv("LOCALAPPDATA")
	if want == "" {
		t.Skip("LOCALAPPDATA not set")
	}
	got, err := UserDataDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("UserDataDir() = %q, want %q", got, want)
	}
}
