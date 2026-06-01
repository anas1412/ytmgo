package main

import (
	"flag"
	"fmt"
	"os"

	"ytmgo/internal/tui"
	"ytmgo/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *versionFlag {
		fmt.Println("ytmgo", version.Full())
		os.Exit(0)
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
