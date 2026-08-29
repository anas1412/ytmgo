package downloader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		`AC/DC - Back In Black`: `AC_DC - Back In Black`,
		`What? "Why" <A>|B:C*D`: `What_ _Why_ _A__B_C_D`,
		`ラブ・ストーリーは突然に`:          `ラブ・ストーリーは突然に`,
	}
	for in, want := range cases {
		if got := sanitizeFilename(in); got != want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Artist - Song sZxzPcT1Meg.m4a")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if got := findExisting(dir, "sZxzPcT1Meg"); got != path {
		t.Fatalf("findExisting = %q, want %q", got, path)
	}
	if got := findExisting(dir, "missing1234"); got != "" {
		t.Fatalf("findExisting for absent id = %q, want empty", got)
	}
	if got := findExisting(filepath.Join(dir, "nope"), "x"); got != "" {
		t.Fatalf("findExisting on missing dir = %q, want empty", got)
	}
}
