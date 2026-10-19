// Command fetchdep copies a pinned tool into a folder, for packaging:
// the Windows release workflow runs `go run ./cmd/fetchdep yt-dlp
// cmd/ytmgo-gui/build/bin` so the installer ships yt-dlp.exe. It goes
// through deps.Fetch, so the release uses the same pinned build and
// checksum as the app does.
package main

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"

	"ytmgo/internal/deps"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: fetchdep <tool> <folder>")
	}
	name, dir := os.Args[1], os.Args[2]
	if err := deps.Fetch(context.Background(), name, nil); err != nil {
		log.Fatal(err)
	}
	src, err := os.Open(deps.Path(name))
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	dst, err := os.Create(filepath.Join(dir, filepath.Base(src.Name())))
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		log.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s -> %s", src.Name(), dst.Name())
}
