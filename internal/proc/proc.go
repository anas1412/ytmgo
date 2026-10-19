// Package proc builds the commands ytmgo runs for external tools
// (yt-dlp, ffmpeg), so every one of them is started and cancelled the
// same way on each platform.
//
// Cancelling has to kill the whole tree, not just the process started:
// yt-dlp spawns ffmpeg children that a plain Process.Kill would orphan.
// How that is done is the platform-specific part, in proc_unix.go and
// proc_windows.go.
package proc
