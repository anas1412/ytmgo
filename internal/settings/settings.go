// Package settings defines the Settings struct and defaults.
// Persistence is handled by the db package.
package settings

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


