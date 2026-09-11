package downloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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

// newIdle builds a downloader whose worker can never actually run
// yt-dlp: an empty PATH makes the exec fail at once, so a job that
// reaches the worker just fails instead of hitting the network.
func newIdle(t *testing.T, outDir, format string) *Downloader {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
	d := New(outDir, format)
	t.Cleanup(d.Close)
	return d
}

func TestStemFor(t *testing.T) {
	cases := []struct{ uploader, title, stem, want string }{
		{"Mild High Club", "Homage", "", "Mild High Club - Homage"},
		// An album track's number has to survive into the filename, or
		// the folder stops sorting in album order.
		{"Mild High Club", "Homage", "01 - Homage", "01 - Homage"},
		{"AC/DC", "Back In Black", "", "AC_DC - Back In Black"},
		{"Artist", "A/B", "03 - A/B", "03 - A_B"},
	}
	for _, c := range cases {
		if got := stemFor(c.uploader, c.title, c.stem); got != c.want {
			t.Errorf("stemFor(%q,%q,%q) = %q, want %q", c.uploader, c.title, c.stem, got, c.want)
		}
	}
}

// TestIsDownloadedFollowsTheFormat: switching m4a→mp3 in settings must
// make an already-downloaded track look absent again, so it can be
// re-fetched in the new format.
func TestIsDownloadedFollowsTheFormat(t *testing.T) {
	dir := t.TempDir()
	d := newIdle(t, dir, "m4a")
	if err := os.WriteFile(filepath.Join(dir, "Artist - Song.m4a"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if !d.IsDownloaded("zzz", "Song", "Artist", dir) {
		t.Error("the m4a on disk was not found")
	}
	d.SetFormat("mp3")
	if d.IsDownloaded("zzz", "Song", "Artist", dir) {
		t.Error("after switching to mp3 the m4a still counts as downloaded")
	}
}

// TestIsDownloadedFindsRenamedFiles: the trackID in the filename is the
// fallback for a file saved under a different artist/title spelling.
func TestIsDownloadedFindsRenamedFiles(t *testing.T) {
	dir := t.TempDir()
	d := newIdle(t, dir, "m4a")
	if err := os.WriteFile(filepath.Join(dir, "Whatever sZxzPcT1Meg.m4a"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if !d.IsDownloaded("sZxzPcT1Meg", "Different Title", "Different Artist", dir) {
		t.Error("the trackID fallback did not match")
	}
	if d.IsDownloaded("notthisone", "Different Title", "Different Artist", dir) {
		t.Error("an unrelated trackID matched")
	}
}

// TestEnqueueSkipsWhatIsAlreadyThere: an existing file must report
// Skipped and create no job, or pressing x twice queues a second
// download of a track already on disk.
func TestEnqueueSkipsWhatIsAlreadyThere(t *testing.T) {
	dir := t.TempDir()
	d := newIdle(t, dir, "m4a")
	path := filepath.Join(dir, "Artist - Song.m4a")
	if err := os.WriteFile(path, nil, 0644); err != nil {
		t.Fatal(err)
	}

	d.Enqueue("zzz", "Song", "Artist", "https://example/x", dir, "")
	select {
	case ev := <-d.Progress():
		if ev.Status != StatusSkipped {
			t.Errorf("status %v, want StatusSkipped", ev.Status)
		}
		if ev.FilePath != path {
			t.Errorf("file path %q, want %q", ev.FilePath, path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no progress event for an already-downloaded track")
	}
	if n := len(d.Jobs()); n != 0 {
		t.Errorf("an already-downloaded track created %d jobs, want 0", n)
	}
}

// TestHasPendingJob: the guard that stops x queueing the same track
// twice. Only pending and downloading count — a finished or failed job
// must not block a retry.
func TestHasPendingJob(t *testing.T) {
	dir := t.TempDir()
	d := newIdle(t, dir, "m4a")

	if d.HasPendingJob("abc") {
		t.Error("an empty downloader reports a pending job")
	}
	d.Enqueue("abc", "Song", "Artist", "https://example/x", dir, "")

	// The worker fails the job quickly (empty PATH), so poll for the
	// terminal state rather than racing it.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		jobs := d.Jobs()
		if len(jobs) == 1 && jobs[0].Status == StatusFailed {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	jobs := d.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	if jobs[0].Status != StatusFailed {
		t.Fatalf("job status %v, want StatusFailed (no yt-dlp on PATH)", jobs[0].Status)
	}
	if d.HasPendingJob("abc") {
		t.Error("a failed job still blocks a retry")
	}
}

// TestJobsAreSnapshots: the UI reads these fields every frame from
// another goroutine, so Jobs must hand back copies rather than aliases
// into the live jobs the worker is writing.
func TestJobsAreSnapshots(t *testing.T) {
	dir := t.TempDir()
	d := newIdle(t, dir, "m4a")
	d.Enqueue("abc", "Song", "Artist", "https://example/x", dir, "")

	first := d.Jobs()
	if len(first) != 1 {
		t.Fatalf("got %d jobs, want 1", len(first))
	}
	second := d.Jobs()
	if first[0] == second[0] {
		t.Error("two calls handed back the same pointer, so neither is a snapshot")
	}
}

// TestProgressPatternMatchesYtDlp: the regex drives the progress bar,
// and yt-dlp always prints a decimal.
func TestProgressPatternMatchesYtDlp(t *testing.T) {
	for _, c := range []struct {
		line string
		want string
	}{
		{"[download]   0.0% of 8.06MiB at 1.00MiB/s", "0.0"},
		{"[download]  45.3% of ~8.06MiB", "45.3"},
		{"[download] 100.0% of 8.06MiB in 00:03", "100.0"},
		{"[download] Destination: x.webm", ""},
		{"[ExtractAudio] Destination: x.m4a", ""},
	} {
		m := progressRe.FindStringSubmatch(c.line)
		got := ""
		if len(m) > 1 {
			got = m[1]
		}
		if got != c.want {
			t.Errorf("%q matched %q, want %q", c.line, got, c.want)
		}
	}
}
