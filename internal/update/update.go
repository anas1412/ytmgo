// Package update finds ytmgo's latest release, for both apps, and
// fetches the Windows installer from it.
package update

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// releases is a var so the tests can point it at a server of their own.
var releases = "https://github.com/anas1412/ytmgo/releases"

// Latest is the latest release's tag, "v1.6.0", read from where
// /releases/latest redirects: no API, no key, no rate limit.
func Latest() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		// The Location header is the answer; following it is not needed.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Get(releases + "/latest")
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently || loc == "" {
		return "", fmt.Errorf("latest release: %s", resp.Status)
	}
	return path.Base(loc), nil // ".../tag/v1.6.0" → "v1.6.0"
}

// Newer reports whether latest is a version to update to from current.
// A local build ("dev") never is.
func Newer(current, latest string) bool {
	return current != "dev" && current != "" && latest != "" && latest != current
}

// Installer is the release's Windows installer.
const Installer = "ytmgo-gui-setup.exe"

// FetchInstaller downloads tag's installer into dir and checks it
// against the checksum published beside it, reporting progress as a
// fraction. It returns the installer's path.
func FetchInstaller(tag, dir string, progress func(float64)) (string, error) {
	base := releases + "/download/" + tag + "/" + Installer
	client := &http.Client{Timeout: 10 * time.Minute}

	resp, err := client.Get(base + ".sha256")
	if err != nil {
		return "", err
	}
	sum, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer checksum: %s %v", resp.Status, err)
	}
	want := strings.ToLower(strings.Fields(string(sum) + " ")[0])

	resp, err = client.Get(base)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("installer: %s", resp.Status)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dst := filepath.Join(dir, Installer)
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), &counter{r: resp.Body, total: resp.ContentLength, progress: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = fmt.Errorf("installer does not match its checksum")
	}
	if err != nil {
		os.Remove(dst)
		return "", err
	}
	return dst, nil
}

// counter reports how much of the download has arrived.
type counter struct {
	r        io.Reader
	n, total int64
	progress func(float64)
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.progress != nil && c.total > 0 {
		c.progress(float64(c.n) / float64(c.total))
	}
	return n, err
}
