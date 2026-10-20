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
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	goruntime "runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ytmgo/internal/db"
	"ytmgo/internal/gui"
)

//go:embed all:frontend
var frontend embed.FS

// The window icon on Linux, where development happens. On Windows the
// icon comes from build/windows/icon.ico, which wails build embeds in
// the exe.
//
//go:embed build/appicon.png
var icon []byte

func main() {
	assets, err := fs.Sub(frontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	d, err := db.Open()
	if err != nil {
		log.Fatal(err)
	}
	app := gui.New(d)
	if goruntime.GOOS == "windows" {
		// WebView2 hands the page's Media Session to Windows only with
		// this feature on: the media keys, and the overlay with the song
		// and its buttons. Kept with anything the user set there.
		args := os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS")
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", strings.TrimSpace(args+" --enable-features=HardwareMediaKeyHandling"))
	}
	// Downloaded and library songs, and their covers, over loopback HTTP
	// (gui.ServeMedia says why not through the asset server).
	if err := gui.ServeMedia(app); err != nil {
		log.Fatal(err)
	}
	var ctx context.Context // the window's, from OnStartup
	err = wails.Run(&options.App{
		Title:     "ytmgo",
		Width:     1200,
		Height:    760,
		MinWidth:  800,
		MinHeight: 520,
		// The page draws its own title bar, with Windows' buttons in its
		// corner (frontend: .titlebar). Windows keeps the resize edges,
		// snapping and the shadow of a framed window.
		Frameless: true,
		// The page's own background, so the window does not flash white
		// before the first paint.
		BackgroundColour: &options.RGBA{R: 0x11, G: 0x11, B: 0x18, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
		Bind:             []interface{}{app},
		// One ytmgo at a time, as one player: a second launch brings the
		// open window forward instead of playing over it.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "ytmgo-6f1c2a8e-gui",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				runtime.WindowUnminimise(ctx)
				runtime.Show(ctx)
			},
		},
		OnStartup: func(c context.Context) {
			ctx = c
			gui.Attach(app, gui.Host{
				Emit: func(event string, data any) { runtime.EventsEmit(ctx, event, data) },
				PickFolder: func(title string) (string, error) {
					return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: title})
				},
				Quit: func() { runtime.Quit(ctx) },
			})
		},
		// The database is opened here rather than in gui.New so that
		// closing it is not a method on App, which the page could call.
		OnShutdown: func(context.Context) {
			gui.Close(app)
			d.Close()
		},
		Linux: &linux.Options{Icon: icon},
	})
	if err != nil {
		log.Fatal(err)
	}
}
