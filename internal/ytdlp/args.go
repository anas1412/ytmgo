// Package ytdlp builds argument slices for yt-dlp subprocess calls.
package ytdlp

import (
	"fmt"
	"os"
	"path/filepath"
)

// CookiesArg returns --cookies-from-browser for the given browser name.
// Supported: "brave", "firefox", "chrome", "chromium", "edge".
// If browser is empty, returns "".
// For brave, it resolves the config path (Brave-Nightly, Brave-Browser, etc.).
func CookiesArg(browser string) string {
	if browser == "" {
		return ""
	}

	switch browser {
	case "brave":
		return braveCookiesArg()
	case "firefox", "chrome", "chromium", "edge":
		return fmt.Sprintf("--cookies-from-browser=%s", browser)
	default:
		// Treat as custom path or name — pass directly
		return fmt.Sprintf("--cookies-from-browser=%s", browser)
	}
}

// UserAgentArg returns --user-agent if ua is non-empty, else "".
func UserAgentArg(ua string) string {
	if ua == "" {
		return ""
	}
	return fmt.Sprintf("--user-agent=%s", ua)
}

// braveCookiesArg tries common Brave config directories.
func braveCookiesArg() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "--cookies-from-browser=brave"
	}
	candidates := []string{
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Origin-Nightly"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Nightly"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Beta"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Dev"),
		filepath.Join(home, "snap", "brave", "current", ".config", "BraveSoftware", "Brave-Browser"),
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return fmt.Sprintf("--cookies-from-browser=brave:%s", p)
		}
	}
	// Fallback: let yt-dlp resolve it
	return "--cookies-from-browser=brave"
}
