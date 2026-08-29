//go:build !linux

package player

import "syscall"

// procAttr is a no-op off Linux (Pdeathsig is Linux-only); the normal
// Shutdown path still quits mpv cleanly.
func procAttr() *syscall.SysProcAttr { return nil }
