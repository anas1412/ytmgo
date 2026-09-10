package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"ytmgo/internal/settings"
	"ytmgo/internal/tui"
	"ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// setupLogging routes the default logger to ytmgo.log in the data dir.
// The default destination is stderr, which corrupts the TUI while the
// alternate screen is active; when no log file can be opened, discard.
func setupLogging() *os.File {
	log.SetOutput(io.Discard)
	// Beside the database, under the platform data dir — a log is state,
	// not configuration, and it used to land in a literal ~/.config.
	base, err := settings.UserDataDir()
	if err != nil {
		return nil
	}
	dir := filepath.Join(base, "ytmgo")
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
	flag.Usage = func() { fmt.Print(cliUsage) }
	flag.Parse()
	if *versionFlag {
		fmt.Println("ytmgo", version.Full())
		os.Exit(0)
	}

	// Headless subcommands (search / play / download) run without the
	// TUI; anything else falls through and opens the player.
	if handled, code := runCLI(flag.Args()); handled {
		os.Exit(code)
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
