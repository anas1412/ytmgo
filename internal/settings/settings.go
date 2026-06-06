// Package settings defines the Settings struct and defaults.
// Persistence is handled by the db package.
package settings

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
	PlaybackMode      int    `json:"playback_mode"`        // 0=Stream, 1=Hybrid, 2=Offline
	DefaultVolume     int    `json:"default_volume"`       // 0-100
	SearchLimit       int    `json:"search_limit"`         // results per search / recommendation batch
	DownloadDir       string `json:"download_dir"`         // relative or absolute path for downloads
	TidalProxyURL     string `json:"tidal_proxy_url"`      // TIDAL API proxy URL
	DownloadFormat    string `json:"download_format"`      // m4a or mp3
	ShowQuotes        bool   `json:"show_quotes"`          // fetch internet quotes
	DiscordRPCEnabled bool   `json:"discord_rpc_enabled"`  // enable Discord Rich Presence
}

// Defaults returns a Settings with sane defaults.
func Defaults() *Settings {
	return &Settings{
		PlaybackMode:      PlaybackStream,
		DefaultVolume:     80,
		SearchLimit:       20,
		DownloadDir:       "downloads",
		TidalProxyURL:     "https://eu-central.monochrome.tf",
		DownloadFormat:    FormatM4A,
		ShowQuotes:        true,
		DiscordRPCEnabled: true,
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


