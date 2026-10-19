//go:build windows || (linux && webkit2_41)

// Command ytmgo-gui is ytmgo's desktop app for Windows, built on the same
// internal packages as the terminal app. See win_migration.md.
//
// It builds on Windows, where Wails needs no cgo, and on Linux only with
// -tags webkit2_41 for development against WebKitGTK 4.1. The ordinary
// Linux build, vet and tests never see it, so they need neither cgo nor
// WebKit headers.
package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"ytmgo/internal/gui"
)

//go:embed all:frontend
var frontend embed.FS

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	app := gui.New()
	err = wails.Run(&options.App{
		Title:     "ytmgo",
		Width:     1200,
		Height:    760,
		MinWidth:  800,
		MinHeight: 520,
		// The page's own background, so the window does not flash white
		// before the first paint.
		BackgroundColour: &options.RGBA{R: 0x11, G: 0x11, B: 0x18, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		Bind:             []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}
