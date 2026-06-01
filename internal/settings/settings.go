// Package settings manages persistent TUI configuration stored as JSON.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings holds all user-configurable values.
type Settings struct {
	StreamMode    bool   `json:"stream_mode"`     // play via URL instead of forcing download
	AutoDownload  bool   `json:"auto_download"`   // auto-download queued tracks for offline
	DefaultVolume int    `json:"default_volume"`  // 0-100
	SearchLimit   int    `json:"search_limit"`    // results per search / recommendation batch
	DownloadDir   string `json:"download_dir"`    // relative or absolute path for downloads
	CookieBrowser string `json:"cookie_browser"`  // "brave", "firefox", "chrome", or ""
}

// Defaults returns a Settings with sane defaults.
func Defaults() *Settings {
	return &Settings{
		StreamMode:    true,
		AutoDownload:  false,
		DefaultVolume: 80,
		SearchLimit:   20,
		DownloadDir:   "downloads",
		CookieBrowser: "brave",
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
