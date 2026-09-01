// Package settings defines the Settings struct and defaults.
// Persistence is handled by the db package.
package settings

import (
	"os"
	"path/filepath"
	"runtime"
)

// Playback mode constants.
const (
	PlaybackStream  = 0 // play via URL, no download
	PlaybackHybrid  = 1 // play while downloading in background
	PlaybackOffline = 2 // download first, then play locally
)

// DownloadFormat constants.
const (
	FormatM4A = "m4a" // AAC, best quality, direct stream copy
	FormatMP3 = "mp3" // MP3, broadest device compatibility
)

// Settings holds all user-configurable values.
type Settings struct {
	PlaybackMode      int    `json:"playback_mode"`       // 0=Stream, 1=Hybrid, 2=Offline
	DefaultVolume     int    `json:"default_volume"`      // 0-100
	SearchLimit       int    `json:"search_limit"`        // results per search / recommendation batch
	DownloadDir       string `json:"download_dir"`        // relative or absolute path for downloads
	DownloadFormat    string `json:"download_format"`     // m4a or mp3
	ShowQuotes        bool   `json:"show_quotes"`         // fetch internet quotes
	ShowHints         bool   `json:"show_hints"`          // inline [key] hints outside the footer
	DiscordRPCEnabled bool   `json:"discord_rpc_enabled"` // enable Discord Rich Presence
	AutoplayEnabled   bool   `json:"autoplay_enabled"`    // auto-queue related tracks when queue empties
	Theme             string `json:"theme"`               // auto, dark, light or terminal
}

// Defaults returns a Settings with sane defaults.
func Defaults() *Settings {
	return &Settings{
		PlaybackMode:      PlaybackStream,
		DefaultVolume:     80,
		SearchLimit:       20,
		DownloadDir:       "downloads",
		DownloadFormat:    FormatM4A,
		ShowQuotes:        true,
		ShowHints:         true,
		DiscordRPCEnabled: true,
		AutoplayEnabled:   true,
		Theme:             "terminal",
	}
}

// DownloadFormatLabel returns a human-readable label for the download format.
func DownloadFormatLabel(f string) string {
	switch f {
	case FormatM4A:
		return "M4A (AAC) — best quality, no re-encode"
	case FormatMP3:
		return "MP3 — broadest device compatibility"
	default:
		return "M4A (AAC) — best quality, no re-encode"
	}
}

// DownloadFormatHint returns a short hint shown in the settings description.
func DownloadFormatHint(f string) string {
	switch f {
	case FormatM4A:
		return "Recommended: copies AAC directly from YouTube (fast, lossless)"
	case FormatMP3:
		return "Transcodes to MP3 (slower, slight quality loss)"
	default:
		return ""
	}
}

// PlaybackModeLabel returns a human-readable label for the playback mode.
func PlaybackModeLabel(mode int) string {
	switch mode {
	case PlaybackStream:
		return "Stream"
	case PlaybackHybrid:
		return "Hybrid"
	case PlaybackOffline:
		return "Offline"
	default:
		return "Hybrid"
	}
}

// ─── Paths ──────────────────────────────────────────────────────────

// ResolveDownloadDir returns the directory downloaded tracks are stored
// in, creating it if needed. Shared by the TUI and the CLI subcommands
// so both write to the same place.
//
// Resolution order:
//  1. A custom path set on the Settings page.
//  2. The platform user-data dir (XDG_DATA_HOME/ytmgo/downloads on
//     Linux, ~/Library/Application Support/ytmgo/downloads on macOS).
//
// The legacy default "downloads" counts as unset, so upgrading users get
// the XDG location instead of a stray folder next to the binary.
func (s *Settings) ResolveDownloadDir() string {
	if dir := s.DownloadDir; dir != "" && dir != "downloads" {
		os.MkdirAll(dir, 0755)
		return dir
	}
	base, err := userDataDir()
	if err != nil {
		return "downloads" // last-ditch fallback
	}
	dir := filepath.Join(base, "ytmgo", "downloads")
	os.MkdirAll(dir, 0755)
	return dir
}

// userDataDir returns the platform base directory for app data (NOT
// configuration — that lives beside the database in ~/.config/ytmgo).
func userDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return xdg, nil
	}
	return filepath.Join(home, ".local", "share"), nil
}
