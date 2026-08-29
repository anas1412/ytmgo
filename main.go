package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"ytmgo/internal/tui"
	"ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// setupLogging routes the default logger to ~/.config/ytmgo/ytmgo.log.
// The default destination is stderr, which corrupts the TUI while the
// alternate screen is active; when no log file can be opened, discard.
func setupLogging() *os.File {
	log.SetOutput(io.Discard)
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(home, ".config", "ytmgo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil
	}
	f, err := os.OpenFile(filepath.Join(dir, "ytmgo.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil
	}
	log.SetOutput(f)
	return f
}

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println("ytmgo", version.Full())
		os.Exit(0)
	}

	if f := setupLogging(); f != nil {
		defer f.Close()
	}

	m := tui.InitialModel()
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	final, err := p.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	// Clean up background processes on any exit path
	if m, ok := final.(tui.Model); ok {
		m.Shutdown()
	}
	if err != nil {
		os.Exit(1)
	}
}
