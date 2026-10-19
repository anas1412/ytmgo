// Package deps locates the external tools ytmgo runs: yt-dlp, ffmpeg,
// ffprobe and mpv.
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
// of asking the user to reorder their PATH. Windows has the same need
// with a different shape: nothing is on PATH at all, so the tools live
// beside the app (put there by the installer) or in ytmgo's own bin
// directory (fetched later).
package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"

	"ytmgo/internal/settings"
)

var (
	mu       sync.Mutex
	resolved = map[string]string{}
)

// Path returns the copy of the named tool to execute ("yt-dlp",
// "ffmpeg", ...; no ".exe"). In order: beside the ytmgo binary, in
// ytmgo's bin directory under the data dir, then whatever PATH offers,
// and otherwise the bare name so exec reports the failure in its own
// words.
func Path(name string) string {
	mu.Lock()
	defer mu.Unlock()
	if p, ok := resolved[name]; ok {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	p := resolve(name, exe, BinDir())
	resolved[name] = p
	return p
}

// Bundled reports whether Path found ytmgo's own copy of the tool rather
// than the one PATH offers. Callers that have to tell another program
// which copy to use — mpv resolves its own yt-dlp — only need to
// override when the answer differs from what that program would pick.
func Bundled(name string) bool {
	p := Path(name)
	return filepath.IsAbs(p) && p != lookPath(name)
}

// BinDir is where ytmgo keeps tools it fetched itself:
// %LOCALAPPDATA%\ytmgo\bin on Windows, ~/.local/share/ytmgo/bin on Linux.
// Empty when the data dir cannot be determined.
func BinDir() string {
	base, err := settings.UserDataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "ytmgo", "bin")
}

// resolve picks the tool for a given ytmgo executable path and bin dir.
// Split out from Path so the choice can be tested without moving the
// test binary around.
func resolve(name, exe, binDir string) string {
	var dirs []string
	if exe != "" {
		// Through symlinks: ~/.local/bin/ytmgo may point elsewhere, and
		// the tool we care about sits next to the real file.
		if real, err := filepath.EvalSymlinks(exe); err == nil {
			exe = real
		}
		dirs = append(dirs, filepath.Dir(exe))
	}
	if binDir != "" {
		dirs = append(dirs, binDir)
	}
	for _, dir := range dirs {
		if candidate := filepath.Join(dir, fileName(name)); isExecutable(candidate) {
			return candidate
		}
	}
	if p := lookPath(name); p != "" {
		return p
	}
	return name
}

// fileName is the tool's file name on this platform.
func fileName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func lookPath(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// isExecutable reports whether path is a file that can be run. Windows
// has no execute bit — the .exe name is what makes a file runnable — so
// there a regular file is enough.
func isExecutable(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || fi.Mode().Perm()&0111 != 0
}
