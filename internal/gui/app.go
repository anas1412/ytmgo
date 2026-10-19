// Package gui is the Go side of ytmgo's desktop app: the methods its
// frontend calls, built on the same internal packages as the TUI.
//
// It does not import Wails. cmd/ytmgo-gui binds an App to the window,
// and every exported method here becomes callable from the frontend.
// Keeping Wails out means this package builds and tests anywhere,
// without cgo or a WebView.
package gui

import "ytmgo/internal/version"

// App is what the frontend talks to.
type App struct{}

// New returns the App to bind.
func New() *App { return &App{} }

// Version is the build's version, e.g. "v1.4.0" or "dev".
func (a *App) Version() string { return version.Version }
