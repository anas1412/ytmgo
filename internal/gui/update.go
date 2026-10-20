package gui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"

	"ytmgo/internal/proc"
	"ytmgo/internal/update"
	"ytmgo/internal/version"
)

// UpdateInfo says whether a newer ytmgo is out: the page shows Update
// in the title bar when it is.
type UpdateInfo struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
}

// CheckUpdate asks GitHub for the latest release. A local build never
// has one to update to.
func (a *App) CheckUpdate() (UpdateInfo, error) {
	info := UpdateInfo{Current: version.Version}
	if version.Version == "dev" {
		return info, nil
	}
	latest, err := a.latest()
	if err != nil {
		return info, problem(err)
	}
	info.Latest, info.Available = latest, update.Newer(version.Version, latest)
	return info, nil
}

// InstallUpdate is the Update button: it downloads the latest
// installer, checked against its published checksum, starts it
// silently and closes the app. The installer waits for the app to be
// gone, installs over it and opens it again (project.nsi). Songs,
// playlists and settings live apart from the app and stay.
//
// The page sends no version: what is installed is what GitHub says is
// latest, read again here.
func (a *App) InstallUpdate() error {
	if runtime.GOOS != "windows" {
		return problem(errNoInstaller)
	}
	tag, err := a.latest()
	if err != nil {
		return problem(err)
	}
	shown := -1
	path, err := a.fetchInstaller(tag, filepath.Join(os.TempDir(), "ytmgo-update"), func(f float64) {
		if p := int(f * 100); p != shown {
			shown = p
			a.emit("update", p)
		}
	})
	if err != nil {
		return problem(err)
	}
	// /D= is the folder this app runs from, so the update goes where
	// the user installed it, not to the default folder.
	exe, err := os.Executable()
	if err != nil {
		return problem(err)
	}
	if err := proc.Hand(path, "/S", "/D="+filepath.Dir(exe)); err != nil {
		return problem(err)
	}
	if a.host.Quit != nil {
		a.host.Quit()
	}
	return nil
}

var errNoInstaller = errors.New("updates install themselves on Windows only")
