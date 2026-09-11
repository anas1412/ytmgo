// Package clipboard copies text to the system clipboard.
//
// Nothing is linked in: the clipboard belongs to the display server, and
// every platform already ships a command that talks to it. Shelling out
// to that command keeps ytmgo free of cgo and of a dependency that would
// have to know about Wayland, X11 and macOS separately.
package clipboard

import (
	"errors"
	"os/exec"
	"strings"
)

// ErrNoTool means no clipboard command is installed. Callers should say
// so rather than fail silently — a copy that quietly does nothing is
// worse than one that explains itself.
var ErrNoTool = errors.New("no clipboard tool found")

// tools are tried in order. wl-copy first: a Wayland session usually
// also answers to xclip through Xwayland, but that writes to the X11
// clipboard, which Wayland applications never read.
var tools = []struct {
	bin  string
	args []string
}{
	{"wl-copy", nil},
	{"xclip", []string{"-selection", "clipboard"}},
	{"xsel", []string{"--clipboard", "--input"}},
	{"pbcopy", nil}, // macOS
}

// Copy writes s to the system clipboard.
func Copy(s string) error {
	for _, t := range tools {
		path, err := exec.LookPath(t.bin)
		if err != nil {
			continue
		}
		cmd := exec.Command(path, t.args...)
		cmd.Stdin = strings.NewReader(s)
		if err := cmd.Run(); err != nil {
			return err
		}
		return nil
	}
	return ErrNoTool
}

// InstallHint names the package that provides a clipboard tool, so a
// missing one is a one-line fix rather than a dead end. Mirrors the
// visualizer's hint for cava.
func InstallHint() string {
	for _, pm := range []struct{ bin, cmd string }{
		{"pacman", "sudo pacman -S wl-clipboard"},
		{"apt", "sudo apt install wl-clipboard"},
		{"dnf", "sudo dnf install wl-clipboard"},
		{"zypper", "sudo zypper install wl-clipboard"},
		{"apk", "sudo apk add wl-clipboard"},
	} {
		if _, err := exec.LookPath(pm.bin); err == nil {
			return pm.cmd + " (or xclip on X11)"
		}
	}
	return "install wl-clipboard, xclip or xsel"
}
