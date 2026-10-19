package gui

import (
	"errors"
	"os"
	"slices"
	"strings"

	"ytmgo/internal/library"
	"ytmgo/internal/queue"
)

// The library is the TUI's: every audio file in the download folder
// and the user's own folders, read with ffprobe, cached by mtime in the
// same table. A song found in it plays from the file wherever it shows
// up, matched by artist and title (library.Match), as in the TUI.

// Library lists the library, scanning it the first time.
func (a *App) Library() ([]queue.Track, error) {
	a.mu.Lock()
	lib := a.lib
	a.mu.Unlock()
	if lib != nil {
		return lib, nil
	}
	return a.Rescan()
}

// Rescan reads the folders again: the page's Rescan, and what follows a
// change of folders. Scans that overlap wait for each other rather than
// run twice.
func (a *App) Rescan() ([]queue.Track, error) {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	a.mu.Lock()
	downloads, extra := a.settings.ResolveDownloadDir(), a.settings.ResolveLibraryDirs()
	a.mu.Unlock()
	lib, err := a.scanLibrary(downloads, extra)
	if err != nil {
		return nil, problem(err)
	}
	a.mu.Lock()
	a.lib = lib
	// Songs queued before the scan finished, or before this file was
	// found, play from it too, as the TUI's backfill does.
	marked := false
	for _, t := range a.q.Tracks() {
		if t.Downloaded && t.FilePath != "" {
			continue
		}
		if f, ok := library.Match(lib, t); ok {
			a.q.UpdateTrack(t.ID, func(qt *queue.Track) { qt.Downloaded, qt.FilePath = true, f.FilePath })
			marked = true
		}
	}
	var s QueueState
	if marked {
		s = a.changed()
	}
	a.mu.Unlock()
	if marked {
		a.emit("queue", s)
	}
	a.emit("library", lib)
	return lib, nil
}

// mark sets Downloaded and FilePath on the songs the library has a file
// for, so lists show ✓ on them.
func (a *App) mark(ts []queue.Track) []queue.Track {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, t := range ts {
		if t.Downloaded {
			continue
		}
		if f, ok := library.Match(a.lib, t); ok {
			ts[i].Downloaded, ts[i].FilePath = true, f.FilePath
		}
	}
	return ts
}

// added puts a file just downloaded into the library, so the song plays
// from it from now on without a rescan. Callers hold a.mu.
func (a *App) added(t queue.Track) {
	if a.lib == nil || t.FilePath == "" {
		return // the first scan will find it
	}
	for _, have := range a.lib {
		if have.FilePath == t.FilePath {
			return
		}
	}
	a.lib = append(a.lib, t)
}

// DeleteFile deletes a library song's file from disk: the TUI's d,
// after the page has asked. Only a file in the library can be deleted,
// so the page cannot reach anything else on disk.
func (a *App) DeleteFile(path string) ([]queue.Track, error) {
	a.mu.Lock()
	i := slices.IndexFunc(a.lib, func(t queue.Track) bool { return t.FilePath == path })
	a.mu.Unlock()
	if i < 0 {
		return a.Library()
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, problem(err)
	}
	a.mu.Lock()
	a.lib = slices.DeleteFunc(a.lib, func(t queue.Track) bool { return t.FilePath == path })
	lib := a.lib
	// Queued copies stream again.
	queued := false
	for _, t := range a.q.Tracks() {
		if t.FilePath == path {
			a.q.UpdateTrack(t.ID, func(qt *queue.Track) { qt.Downloaded, qt.FilePath = false, "" })
			queued = true
		}
	}
	var s QueueState
	if queued {
		s = a.changed()
	}
	a.mu.Unlock()
	if queued {
		a.emit("queue", s)
	}
	return lib, nil
}

// folders are the user's own music folders, as Settings lists them.
func (a *App) folders() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string{}, a.settings.ResolveLibraryDirs()...)
}

// AddLibraryFolder asks for a folder with the native dialog, adds it,
// and rescans. Cancelling changes nothing.
func (a *App) AddLibraryFolder() ([]string, error) {
	if a.host.PickFolder == nil {
		return a.folders(), nil
	}
	dir, err := a.host.PickFolder("Add a music folder")
	if err != nil {
		return a.folders(), problem(err)
	}
	if dir == "" || slices.Contains(a.folders(), dir) {
		return a.folders(), nil
	}
	a.setFolders(append(a.folders(), dir))
	go a.Rescan()
	return a.folders(), nil
}

// RemoveLibraryFolder takes a folder out of the library. Its files stay
// on disk.
func (a *App) RemoveLibraryFolder(dir string) []string {
	a.setFolders(slices.DeleteFunc(a.folders(), func(d string) bool { return d == dir }))
	go a.Rescan()
	return a.folders()
}

// setFolders saves the list the way the TUI keeps it: one string,
// comma separated.
//
// ponytail: a folder whose name has a comma in it splits in two when
// read back; fixing it means changing how the TUI stores the list too.
func (a *App) setFolders(dirs []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.LibraryDirs = strings.Join(dirs, ", ")
	a.saveSettings(a.settings)
}
