// Package settings manages persistent TUI configuration stored as JSON.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Playback mode constants.
const (
	PlaybackStream  = 0 // play via URL, no download
	PlaybackHybrid  = 1 // play while downloading in background
	PlaybackOffline = 2 // download first, then play locally
)

// Settings holds all user-configurable values.
type Settings struct {
	PlaybackMode  int    `json:"playback_mode"`  // 0=Stream, 1=Hybrid, 2=Offline
	DefaultVolume int    `json:"default_volume"` // 0-100
	SearchLimit   int    `json:"search_limit"`   // results per search / recommendation batch
	DownloadDir   string `json:"download_dir"`   // relative or absolute path for downloads
	CookieBrowser string `json:"cookie_browser"` // "brave", "firefox", "chrome", or ""
	UserAgent     string `json:"user_agent"`     // custom UA for yt-dlp (empty = yt-dlp default)
	ShowQuotes    bool   `json:"show_quotes"`    // fetch internet quotes (falls back to local pool when off or offline)
}

// Defaults returns a Settings with sane defaults.
func Defaults() *Settings {
	return &Settings{
		PlaybackMode:  PlaybackStream,
		DefaultVolume: 80,
		SearchLimit:   20,
		DownloadDir:   "downloads",
		CookieBrowser: "brave",
		ShowQuotes:    true,
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

// configPath returns the path to the JSON settings file.
func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home dir: %w", err)
	}
	dir := filepath.Join(home, ".config", "ytmgo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config dir %s: %w", dir, err)
	}
	return filepath.Join(dir, "settings.json"), nil
}

// Load reads settings from disk. If the file doesn't exist or is corrupt,
// it returns Defaults and a non-nil error (callers may log/save).
func Load() (*Settings, error) {
	path, err := configPath()
	if err != nil {
		return Defaults(), err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Defaults(), nil // first run — no error, will save on first write
		}
		return Defaults(), fmt.Errorf("reading settings: %w", err)
	}

	s := Defaults()
	if err := json.Unmarshal(data, s); err != nil {
		return Defaults(), fmt.Errorf("parsing settings: %w", err)
	}
	return s, nil
}

// Save writes settings to disk as JSON.
func (s *Settings) Save() error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}
	return nil
}
