// Package settings defines the Settings struct and defaults.
// Persistence is handled by the db package.
package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// VisualizerOn and LyricsOn remember whether each pane was left open,
	// so hiding one sticks across restarts instead of coming back next
	// launch. Both default on.
	VisualizerOn bool `json:"visualizer_on"`
	LyricsOn     bool `json:"lyrics_on"`
	// CopyMusicLinks picks which host u writes to the clipboard. Off by
	// default: a youtube.com link opens anywhere, including for people
	// without YouTube Music, and still plays the same recording.
	CopyMusicLinks bool `json:"copy_music_links"`
	// LastFMSessionKey is the permanent key Last.fm hands over once the
	// user approves ytmgo; empty means scrobbling is off. LastFMUser is
	// the account it belongs to, kept only so the Settings row can say
	// who is connected.
	LastFMSessionKey string `json:"lastfm_session_key"`
	LastFMUser       string `json:"lastfm_user"`
	// LibraryDirs is the user's own music, as they typed it: folders
	// separated by commas, ~ allowed. The downloads folder is always
	// part of the library and is not listed here.
	LibraryDirs string `json:"library_dirs"`
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
		VisualizerOn:      true,
		LyricsOn:          true,
		CopyMusicLinks:    false,
	}
}

// CopyLinkLabel names the host u copies to the clipboard.
func CopyLinkLabel(musicHost bool) string {
	if musicHost {
		return "music.youtube.com — opens in YouTube Music"
	}
	return "youtube.com — opens anywhere"
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

// PlaybackModeLabel returns a human-readable label for the playback
// mode. The labels say what you get rather than what the code does:
// "Hybrid" named the implementation and left the reader to guess, and
// the guess that matters is whether a track is kept on disk.
func PlaybackModeLabel(mode int) string {
	switch mode {
	case PlaybackHybrid:
		return "Download while playing"
	case PlaybackOffline:
		return "Download first, then play"
	default: // PlaybackStream, and anything unrecognised
		return "Stream only — nothing is saved"
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

// UserDataDir returns the platform base directory for everything ytmgo
// keeps between runs: the database, the log, and downloads. Exported so
// the db package resolves the same base rather than keeping a second
// copy of the platform rules that could drift from this one.
func UserDataDir() (string, error) { return userDataDir() }

// userDataDir returns the platform base directory for app data.
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

// ResolveLibraryDirs turns the typed list into paths: split on commas,
// trimmed, ~ expanded, blanks dropped. Nothing is checked to exist —
// the scanner treats a missing folder as an empty one.
func (s *Settings) ResolveLibraryDirs() []string {
	var out []string
	home, _ := os.UserHomeDir()
	for _, part := range strings.Split(s.LibraryDirs, ",") {
		p := strings.TrimSpace(part)
		// A folder dropped onto the terminal arrives however the
		// terminal likes to paste it: quoted, as a file:// URL, or with
		// spaces backslash-escaped. Take all three.
		p = strings.TrimPrefix(p, "file://")
		p = strings.Trim(p, `"'`)
		p = strings.ReplaceAll(p, `\ `, " ")
		if p == "" {
			continue
		}
		if p == "~" || strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
		out = append(out, filepath.Clean(p))
	}
	return out
}
