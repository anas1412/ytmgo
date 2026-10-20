package gui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"ytmgo/internal/queue"
	"ytmgo/internal/search"
)

// libApp is an App whose library is files, the one given and nothing
// else, with the scan's arguments kept.
func libApp(t *testing.T, files ...queue.Track) (*App, *[]string) {
	t.Helper()
	a, _, _ := dlApp(t)
	var scanned []string
	a.scanLibrary = func(downloads string, extra []string) ([]queue.Track, error) {
		scanned = append([]string{downloads}, extra...)
		return append([]queue.Track{}, files...), nil
	}
	return a, &scanned
}

func file(t *testing.T, title, artist string) queue.Track {
	path := filepath.Join(t.TempDir(), artist+" - "+title+".m4a")
	os.WriteFile(path, []byte("audio"), 0644)
	return queue.Track{ID: path, Title: title, Artist: artist, FilePath: path, Downloaded: true}
}

// A song queued from a search before the scan plays from its file once
// the scan finds it, and search results show ✓ for it.
func TestLibraryMatchesSongsToFiles(t *testing.T) {
	f := file(t, "Song", "Queen")
	a, _ := libApp(t, f)
	a.streamURL = func(string) (string, error) { return "https://stream", nil }
	a.search = func(string, int) ([]search.Result, error) {
		return []search.Result{{ID: "yt1", Title: "Song (Official Video)", Uploader: "Queen - Topic", URL: "https://watch"}}, nil
	}
	a.Add(queue.Track{ID: "yt1", Title: "Song", Artist: "Queen", URL: "https://watch"})
	if _, err := a.Rescan(); err != nil {
		t.Fatal(err)
	}
	if got := a.Queue().Tracks[0]; got.FilePath != f.FilePath {
		t.Errorf("queued song has file %q, want %q", got.FilePath, f.FilePath)
	}
	found, _ := a.Search("queen")
	if r := found.Tracks[0]; !r.Downloaded || r.FilePath != f.FilePath {
		t.Errorf("search result not marked: %+v", r)
	}
	if src, _ := a.PlaySource(found.Tracks[0]); src != "/local/yt1" {
		t.Errorf("plays from %q", src)
	}
}

// d deletes the file, and only a library file: the page cannot name
// any path on disk. Queued copies go back to streaming.
func TestDeleteFileOnlyInTheLibrary(t *testing.T) {
	f := file(t, "Song", "Queen")
	a, _ := libApp(t, f)
	a.Library()
	a.Add(f)

	other := filepath.Join(t.TempDir(), "keep.txt")
	os.WriteFile(other, []byte("x"), 0644)
	a.DeleteFile(other)
	if _, err := os.Stat(other); err != nil {
		t.Fatal("deleted a file outside the library")
	}

	lib, err := a.DeleteFile(f.FilePath)
	if err != nil || len(lib) != 0 {
		t.Fatalf("library %v, %v", lib, err)
	}
	if _, err := os.Stat(f.FilePath); !os.IsNotExist(err) {
		t.Error("file still there")
	}
	if q := a.Queue().Tracks[0]; q.Downloaded || q.FilePath != "" {
		t.Errorf("queued copy still points at the file: %+v", q)
	}
}

// Folders are kept as the TUI keeps them, and the scan reads them all,
// the download folder first.
func TestLibraryFolders(t *testing.T) {
	a, scanned := libApp(t)
	// Adding and removing a folder rescan in the background. Each scan
	// makes the download folder again (ResolveDownloadDir), so the test
	// waits for all three, or one would make it while the temp folders
	// are being deleted.
	scans := make(chan struct{}, 3)
	scan := a.scanLibrary
	a.scanLibrary = func(downloads string, extra []string) ([]queue.Track, error) {
		defer func() { scans <- struct{}{} }()
		return scan(downloads, extra)
	}
	defer func() {
		for range 3 {
			select {
			case <-scans:
			case <-time.After(5 * time.Second):
				t.Fatal("a rescan never ran")
			}
		}
		a.scanMu.Lock() // and the last has returned
		a.scanMu.Unlock()
	}()
	picked := t.TempDir()
	a.host.PickFolder = func(string) (string, error) { return picked, nil }
	got, err := a.AddLibraryFolder()
	if err != nil || len(got) != 1 || got[0] != picked {
		t.Fatalf("folders %v, %v", got, err)
	}
	if a.AddLibraryFolder(); len(a.folders()) != 1 {
		t.Error("the same folder was added twice")
	}
	a.Rescan()
	if len(*scanned) != 2 || (*scanned)[0] != a.settings.DownloadDir || (*scanned)[1] != picked {
		t.Errorf("scanned %v", *scanned)
	}
	if got := a.RemoveLibraryFolder(picked); len(got) != 0 {
		t.Errorf("left %v", got)
	}
}
