package deps

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func serve(t *testing.T, body []byte) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// files lists what is left in dir, so a failed fetch can be shown to
// leave nothing — no .part, no half-installed tool.
func files(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestFetchInstallsAVerifiedTool(t *testing.T) {
	body := []byte("the tool")
	dir := t.TempDir()
	p := pin{url: serve(t, body), sha256: sum(body)}

	var seen int64
	if err := fetch(context.Background(), p, "yt-dlp", dir, func(done, _ int64) { seen = done }); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, fileName("yt-dlp")))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Errorf("installed %q, want %q", got, body)
	}
	if seen != int64(len(body)) {
		t.Errorf("progress reported %d bytes, want %d", seen, len(body))
	}
	if left := files(t, dir); len(left) != 1 {
		t.Errorf("dir holds %v, want only the tool", left)
	}
	// Path's lookup accepts it: on Unix that needs the execute bit.
	if got := resolve("yt-dlp", "", dir); got != filepath.Join(dir, fileName("yt-dlp")) {
		t.Errorf("Path would not use the fetched copy: resolved %q", got)
	}
}

// A download that does not match its pin is never installed, and the
// partial file is gone too.
func TestFetchRejectsAChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	p := pin{url: serve(t, []byte("tampered")), sha256: sum([]byte("the tool"))}

	if err := fetch(context.Background(), p, "yt-dlp", dir, nil); err == nil {
		t.Fatal("a mismatched download was accepted")
	}
	if left := files(t, dir); len(left) != 0 {
		t.Errorf("a failed fetch left %v behind", left)
	}
}

func zipOf(t *testing.T, members map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range members {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(content))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

var ffmpegFiles = map[string]string{
	"/bin/ffmpeg.exe":  "ffmpeg.exe",
	"/bin/ffprobe.exe": "ffprobe.exe",
}

// One ffmpeg archive installs both tools, wherever the build puts its
// top-level folder.
func TestFetchExtractsEveryMember(t *testing.T) {
	archive := zipOf(t, map[string]string{
		"ffmpeg-9.0.2-essentials_build/bin/ffmpeg.exe":  "ffmpeg",
		"ffmpeg-9.0.2-essentials_build/bin/ffprobe.exe": "ffprobe",
		"ffmpeg-9.0.2-essentials_build/README.txt":      "ignored",
	})
	dir := t.TempDir()
	p := pin{url: serve(t, archive), sha256: sum(archive), files: ffmpegFiles}

	if err := fetch(context.Background(), p, "ffmpeg", dir, nil); err != nil {
		t.Fatal(err)
	}
	for target, want := range map[string]string{"ffmpeg.exe": "ffmpeg", "ffprobe.exe": "ffprobe"} {
		got, err := os.ReadFile(filepath.Join(dir, target))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s holds %q, want %q", target, got, want)
		}
	}
	if left := files(t, dir); len(left) != 2 {
		t.Errorf("dir holds %v, want only the two tools", left)
	}
}

// An archive missing one tool installs neither: ffmpeg without ffprobe
// would pass the lookup and then fail on the first local file.
func TestFetchInstallsNothingFromAnIncompleteArchive(t *testing.T) {
	archive := zipOf(t, map[string]string{"x/bin/ffmpeg.exe": "ffmpeg"})
	dir := t.TempDir()
	p := pin{url: serve(t, archive), sha256: sum(archive), files: ffmpegFiles}

	if err := fetch(context.Background(), p, "ffmpeg", dir, nil); err == nil {
		t.Fatal("an archive without ffprobe was accepted")
	}
	if left := files(t, dir); len(left) != 0 {
		t.Errorf("a failed fetch left %v behind", left)
	}
}
