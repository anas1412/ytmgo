// Package ytdlp locates the yt-dlp binary ytmgo should run.
//
// It exists because PATH order is the user's, not ours. install.sh puts
// an upstream yt-dlp next to the ytmgo binary, since a distro-packaged
// one freezes and cannot self-update — Debian and Ubuntu still carry
// builds from 2023 alongside the current one, and a stale yt-dlp cannot
// open a single track. But a packaged copy in /usr/bin usually sits
// earlier on PATH than ~/.local/bin, so the stale one would still win
// and the installer could only warn about it.
//
// Preferring our own copy by absolute path removes that problem instead
// of asking the user to reorder their PATH.
package ytdlp

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

const binary = "yt-dlp"

var (
	once     sync.Once
	resolved string
)

// Path returns the yt-dlp to execute: the copy beside the ytmgo binary
// when there is one, otherwise whatever PATH offers, otherwise the bare
// name so exec reports the failure in its own words.
func Path() string {
	once.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			exe = ""
		}
		resolved = resolveBeside(exe)
	})
	return resolved
}

// Bundled reports whether Path found a copy installed alongside ytmgo,
// rather than falling back to PATH. Callers that have to tell another
// program which yt-dlp to use — mpv resolves its own — only need to
// override when the answer differs from what that program would pick.
func Bundled() bool {
	return filepath.IsAbs(Path()) && filepath.Base(Path()) == binary &&
		Path() != lookPath()
}

// resolveBeside picks the yt-dlp for a given ytmgo executable path.
// Split out from Path so the choice can be tested without moving the
// test binary around.
func resolveBeside(exe string) string {
	if exe != "" {
		// Through symlinks: ~/.local/bin/ytmgo may point elsewhere, and
		// the yt-dlp we care about sits next to the real file.
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		candidate := filepath.Join(filepath.Dir(exe), binary)
		if isExecutable(candidate) {
			return candidate
		}
	}
	if p := lookPath(); p != "" {
		return p
	}
	return binary
}

func lookPath() string {
	p, err := exec.LookPath(binary)
	if err != nil {
		return ""
	}
	return p
}

func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0
}
