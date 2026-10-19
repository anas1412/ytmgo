package deps

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// pin is one exact build of a tool that Fetch may install.
type pin struct {
	url    string
	sha256 string
	// files maps a path suffix inside a zip to the file it is installed
	// as. Nil means the download is the tool itself.
	files map[string]string
}

// Fetch downloads the named tool into BinDir, where Path finds it next.
// The download is checked against a pinned SHA-256 before anything is
// installed; on any failure nothing is left behind. progress, if not
// nil, is called as bytes arrive (total is -1 when the server does not
// say).
func Fetch(ctx context.Context, name string, progress func(done, total int64)) error {
	p, ok := pins[name]
	if !ok {
		return fmt.Errorf("ytmgo cannot download %s on %s", name, runtime.GOOS)
	}
	dir := BinDir()
	if dir == "" {
		return fmt.Errorf("cannot determine where to install %s", name)
	}
	if err := fetch(ctx, p, name, dir, progress); err != nil {
		return err
	}
	// Path caches its answers; the next call must see the new copy.
	mu.Lock()
	clear(resolved)
	mu.Unlock()
	return nil
}

func fetch(ctx context.Context, p pin, name, dir string, progress func(done, total int64)) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, name+"-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s: %s", name, resp.Status)
	}

	sum := sha256.New()
	var body io.Reader = resp.Body
	if progress != nil {
		body = &progressReader{r: body, total: resp.ContentLength, fn: progress}
	}
	if _, err := io.Copy(io.MultiWriter(tmp, sum), body); err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	if got := hex.EncodeToString(sum.Sum(nil)); got != p.sha256 {
		return fmt.Errorf("downloaded %s does not match its checksum (got %s), so it was not installed", name, got)
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if p.files == nil {
		return install(tmp.Name(), filepath.Join(dir, fileName(name)))
	}
	return installFromZip(tmp.Name(), dir, p.files)
}

// installFromZip extracts every wanted member, and only installs them
// once all are out, so a broken archive installs none of them.
func installFromZip(archive, dir string, files map[string]string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("opening download: %w", err)
	}
	defer zr.Close()

	staged := map[string]string{} // staged temp file -> final path
	defer func() {
		for tmp := range staged {
			os.Remove(tmp)
		}
	}()
	for suffix, target := range files {
		f := findMember(zr, suffix)
		if f == nil {
			return fmt.Errorf("download has no %s", suffix)
		}
		tmp, err := extract(f, dir)
		if err != nil {
			return err
		}
		staged[tmp] = filepath.Join(dir, target)
	}
	for tmp, final := range staged {
		if err := install(tmp, final); err != nil {
			return err
		}
	}
	return nil
}

func findMember(zr *zip.ReadCloser, suffix string) *zip.File {
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, suffix) {
			return f
		}
	}
	return nil
}

func extract(f *zip.File, dir string) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	out, err := os.CreateTemp(dir, "extract-*.part")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(out.Name())
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(out.Name())
		return "", err
	}
	return out.Name(), nil
}

// install moves a finished file into place, executable.
func install(tmp, final string) error {
	if err := os.Chmod(tmp, 0755); err != nil {
		return err
	}
	return os.Rename(tmp, final)
}

type progressReader struct {
	r     io.Reader
	done  int64
	total int64
	fn    func(done, total int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	p.fn(p.done, p.total)
	return n, err
}
