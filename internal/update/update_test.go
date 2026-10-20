package update

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		cur, latest string
		want        bool
	}{{"v1.5.0", "v1.6.0", true}, {"v1.6.0", "v1.6.0", false}, {"dev", "v1.6.0", false}, {"v1.5.0", "", false}} {
		if got := Newer(c.cur, c.latest); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.cur, c.latest, got)
		}
	}
}

// The installer is kept only when it matches the checksum beside it.
func TestFetchInstallerChecksum(t *testing.T) {
	body := []byte("MZ installer")
	sum := sha256.Sum256(body)
	good := hex.EncodeToString(sum[:])
	published := good
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/download/v9.9.9/" + Installer:
			w.Write(body)
		case "/download/v9.9.9/" + Installer + ".sha256":
			w.Write([]byte(published + "  " + Installer + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	releases = srv.URL

	var last float64
	path, err := FetchInstaller("v9.9.9", t.TempDir(), func(f float64) { last = f })
	if err != nil || last != 1 {
		t.Fatalf("good: %v, progress %v", err, last)
	}
	if got, _ := os.ReadFile(path); string(got) != string(body) {
		t.Errorf("wrote %q", got)
	}

	published = hex.EncodeToString(make([]byte, 32))
	dir := t.TempDir()
	if _, err := FetchInstaller("v9.9.9", dir, nil); err == nil {
		t.Fatal("a mismatching installer was accepted")
	}
	if left, _ := os.ReadDir(dir); len(left) != 0 {
		t.Errorf("left behind: %v", left)
	}
}
