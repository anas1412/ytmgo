package gui

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"ytmgo/internal/downloader"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

// fakeDL is a downloader that runs nothing: it records what it was
// asked to download, and the test sends progress itself.
type fakeDL struct {
	mu       sync.Mutex
	enqueued []downloader.Job
	pending  map[string]bool
	onDisk   map[string]bool
	progress chan downloader.ProgressEvent
}

func newFakeDL() *fakeDL {
	return &fakeDL{pending: map[string]bool{}, onDisk: map[string]bool{}, progress: make(chan downloader.ProgressEvent)}
}

func (f *fakeDL) HasPendingJob(id string) bool { return f.pending[id] }
func (f *fakeDL) IsDownloaded(id, _, _, _ string) bool {
	return f.onDisk[id]
}
func (f *fakeDL) Enqueue(id, title, uploader, url, dir, cover string) {
	f.EnqueueAs(id, title, uploader, url, dir, cover, "")
}
func (f *fakeDL) EnqueueAs(id, title, uploader, url, dir, cover, stem string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueued = append(f.enqueued, downloader.Job{TrackID: id, Title: title, Uploader: uploader, URL: url, OutDir: dir, NameStem: stem})
}
func (f *fakeDL) SetFormat(string) {}
func (f *fakeDL) Jobs() []*downloader.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*downloader.Job{}
	for _, j := range f.enqueued {
		out = append(out, &j)
	}
	return out
}
func (f *fakeDL) Progress() <-chan downloader.ProgressEvent { return f.progress }
func (f *fakeDL) Close()                                    {}

func dlApp(t *testing.T) (*App, *fakeDL, chan string) {
	t.Helper()
	f := newFakeDL()
	s := settings.Defaults()
	s.DownloadDir = t.TempDir()
	events := make(chan string, 10)
	a := &App{
		settings: s, q: queue.New(),
		saveQueue:     func([]queue.Track, int, bool, bool, bool) error { return nil },
		saveSettings:  func(*settings.Settings) error { return nil },
		newDownloader: func(string, string) downloads { return f },
	}
	Attach(a, Host{Emit: func(name string, _ any) { events <- name }})
	return a, f, events
}

func TestDownloadSaysWhatItDid(t *testing.T) {
	a, f, _ := dlApp(t)
	f.pending["p"], f.onDisk["d"] = true, true
	for id, want := range map[string]string{"p": AlreadyQueued, "d": AlreadyThere, "n": Queued} {
		if got, _ := a.Download(queue.Track{ID: id, Title: "T", Artist: "A"}); got != want {
			t.Errorf("Download(%s) = %q, want %q", id, got, want)
		}
	}
	if len(f.enqueued) != 1 || f.enqueued[0].TrackID != "n" || f.enqueued[0].OutDir != a.settings.DownloadDir {
		t.Errorf("enqueued %+v, want only n, into the download folder", f.enqueued)
	}
}

// The first download waits for ffmpeg; one that cannot be had stops the
// download, rather than queueing one bound to fail.
func TestDownloadWaitsForFFmpeg(t *testing.T) {
	a, f, events := dlApp(t)
	a.fetchTool = func(name string, progress func(done, total int64)) error {
		progress(50, 100)
		return errors.New("dial tcp: lookup www.gyan.dev: no such host")
	}
	_, err := a.Download(queue.Track{ID: "n"})
	if p, ok := err.(*Problem); !ok || p.Kind != KindOffline {
		t.Errorf("err = %v, want an offline problem", err)
	}
	if <-events != "tool" || len(f.enqueued) != 0 {
		t.Errorf("enqueued %v without ffmpeg", f.enqueued)
	}
}

// An album goes into its own folder, numbered in album order: not the
// root of the download folder, where the TUI puts it.
func TestDownloadAlbumNumbersIntoItsFolder(t *testing.T) {
	a, f, _ := dlApp(t)
	a.album = func(string) (ytmusic.Album, error) {
		return ytmusic.Album{Title: "Disc", Artist: "Band", Tracks: []ytmusic.Track{{VideoID: "v1", Title: "One"}, {VideoID: "v2", Title: "Two"}}}, nil
	}
	n, err := a.DownloadAlbum("MPREb_x")
	if err != nil || n != 2 {
		t.Fatalf("DownloadAlbum = %d, %v", n, err)
	}
	dir := filepath.Join(a.settings.DownloadDir, "Band - Disc")
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("album folder not made: %v", err)
	}
	if j := f.enqueued[1]; j.OutDir != dir || j.NameStem != "2 - Two" || j.Uploader != "Band" || !strings.Contains(j.URL, "v2") {
		t.Errorf("second track enqueued as %+v", j)
	}
}

// A finished download marks the queued track, so it plays from the file
// from then on, and the page hears about both.
func TestFinishedDownloadMarksTheQueue(t *testing.T) {
	a, f, events := dlApp(t)
	a.Add(queue.Track{ID: "n", Title: "T"})
	a.Download(queue.Track{ID: "n", Title: "T"})
	<-events // the job, queued
	f.progress <- downloader.ProgressEvent{TrackID: "n", Status: downloader.StatusDone, FilePath: "/music/T.m4a"}
	for _, want := range []string{"queue", "downloads"} {
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("event %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("no %q event", want)
		}
	}
	if got := a.Queue().Tracks[0]; !got.Downloaded || got.FilePath != "/music/T.m4a" {
		t.Errorf("queued track is %+v", got)
	}
}

// A downloaded file plays from disk through Media; anything else is a
// stream. Media serves only what PlaySource handed out.
func TestPlaySourcePrefersTheFile(t *testing.T) {
	a, _, _ := dlApp(t)
	file := filepath.Join(t.TempDir(), "song.m4a")
	os.WriteFile(file, []byte("0123456789"), 0644)
	a.streamURL = func(string) (string, error) { return "https://stream", nil }

	if src, _ := a.PlaySource(queue.Track{ID: "s", URL: "https://watch"}); src != "https://stream" {
		t.Errorf("a track not downloaded plays from %q", src)
	}
	src, err := a.PlaySource(queue.Track{ID: "d", Downloaded: true, FilePath: file, URL: "https://watch"})
	if err != nil || src != "/local/d" {
		t.Fatalf("a downloaded track plays from %q, %v", src, err)
	}

	h := Media(a)
	req := httptest.NewRequest("GET", src, nil)
	req.Header.Set("Range", "bytes=2-4")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 206 || rec.Body.String() != "234" {
		t.Errorf("ranged read: %d %q", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/local/s", nil))
	if rec.Code != 404 {
		t.Errorf("a file never handed out: %d", rec.Code)
	}
}

// A file deleted since it was downloaded falls back to the stream.
func TestPlaySourceStreamsAGoneFile(t *testing.T) {
	a, _, _ := dlApp(t)
	a.streamURL = func(string) (string, error) { return "https://stream", nil }
	src, _ := a.PlaySource(queue.Track{ID: "d", Downloaded: true, FilePath: "/no/such.m4a", URL: "https://watch"})
	if src != "https://stream" {
		t.Errorf("plays from %q", src)
	}
}

// finish ends the fake's job for id as the downloader would.
func (f *fakeDL) finish(t *testing.T, id string, path string, err error) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		f.mu.Lock()
		for i := range f.enqueued {
			if f.enqueued[i].TrackID == id {
				f.enqueued[i].Status, f.enqueued[i].FilePath, f.enqueued[i].Err = downloader.StatusDone, path, err
				if err != nil {
					f.enqueued[i].Status = downloader.StatusFailed
				}
				f.mu.Unlock()
				return
			}
		}
		f.mu.Unlock()
		if time.Now().After(deadline) {
			t.Fatalf("%s was never downloaded", id)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func modeApp(t *testing.T, mode int) (*App, *fakeDL) {
	a, f, _ := dlApp(t)
	a.settings.PlaybackMode = mode
	a.pollEvery = time.Millisecond
	a.streamURL = func(string) (string, error) { return "https://stream", nil }
	return a, f
}

func TestPlaybackModes(t *testing.T) {
	song := queue.Track{ID: "s", Title: "T", Artist: "A", URL: "https://watch"}

	a, f := modeApp(t, settings.PlaybackStream)
	if src, _ := a.PlaySource(song); src != "https://stream" || len(f.Jobs()) != 0 {
		t.Errorf("stream only: %q, %d downloads", src, len(f.Jobs()))
	}

	a, f = modeApp(t, settings.PlaybackHybrid)
	if src, _ := a.PlaySource(song); src != "https://stream" {
		t.Errorf("download while playing plays from %q", src)
	}
	f.finish(t, "s", "", nil) // it was downloaded too

	// Download first waits for the file, and plays it.
	a, f = modeApp(t, settings.PlaybackOffline)
	path := filepath.Join(t.TempDir(), "A - T.m4a")
	os.WriteFile(path, []byte("audio"), 0644)
	got := make(chan string)
	go func() {
		src, _ := a.PlaySource(song)
		got <- src
	}()
	f.finish(t, "s", path, nil)
	if src := <-got; src != "/local/s" {
		t.Errorf("download first plays from %q", src)
	}

	// A download that fails says why, rather than streaming anyway.
	a, f = modeApp(t, settings.PlaybackOffline)
	errs := make(chan error)
	go func() {
		_, err := a.PlaySource(song)
		errs <- err
	}()
	f.finish(t, "s", "", errors.New("ERROR: Video unavailable"))
	if p, ok := (<-errs).(*Problem); !ok || p.Kind != KindUnavailable {
		t.Errorf("failed download: %v", p)
	}
}
