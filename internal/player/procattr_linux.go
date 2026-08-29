package player

import "syscall"

// procAttr makes mpv die with ytmgo even on an ungraceful exit
// (SIGKILL, crash, closed terminal): the kernel delivers SIGTERM to
// mpv when its parent goes away, so no idle mpv is ever orphaned.
func procAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}
