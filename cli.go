package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ytmgo/internal/db"
	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
)

// Headless subcommands. Running ytmgo with no arguments opens the TUI;
// these let it be used as a plain CLI tool and scripted:
//
//	ytmgo search <query>     print matching tracks
//	ytmgo play   <query>     play the first match, block until it ends
//	ytmgo download <query>   download the first match
//
// Each command is self-contained: no TUI, no queue, no database writes
// beyond reading the user's settings so downloads land in the same
// folder and format the app is configured for.

const cliUsage = `ytmgo — YouTube Music from the terminal

Usage:
  ytmgo                    open the interactive player
  ytmgo search <query>     print matching tracks
  ytmgo play <query>       play the first match (ctrl+c to stop)
  ytmgo download <query>   download the first match
  ytmgo --version          print version

While the player is running, media keys and playerctl control it:
  playerctl -p ytmgo play-pause | next | previous | metadata
`

// runCLI dispatches a subcommand. Returns false when args name no
// subcommand, in which case the caller launches the TUI.
func runCLI(args []string) (handled bool, exitCode int) {
	if len(args) == 0 {
		return false, 0
	}
	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "search", "play", "download":
		query := joinQuery(rest)
		if query == "" {
			fmt.Fprintf(os.Stderr, "ytmgo %s: missing search query\n\nUsage: ytmgo %s <query>\n", cmd, cmd)
			return true, 2
		}
		var err error
		switch cmd {
		case "search":
			err = cliSearch(query)
		case "play":
			err = cliPlay(query)
		case "download":
			err = cliDownload(query)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "ytmgo: "+err.Error())
			return true, 1
		}
		return true, 0

	case "help", "-h", "--help":
		fmt.Print(cliUsage)
		return true, 0
	}
	return false, 0
}

// joinQuery rebuilds the query from argv so quoting is optional:
// `ytmgo play homage mild high club` works without quotes.
func joinQuery(args []string) string {
	out := ""
	for _, a := range args {
		if a == "" {
			continue
		}
		if out != "" {
			out += " "
		}
		out += a
	}
	return out
}

// ─── search ─────────────────────────────────────────────────────────

const cliSearchLimit = 10

func cliSearch(query string) error {
	results, err := search.Search(query, cliSearchLimit)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		return fmt.Errorf("no results for %q", query)
	}
	for i, r := range results {
		fmt.Printf("%2d. %s — %s  (%s)\n", i+1, r.Title, r.Uploader, fmtDur(r.Duration))
	}
	return nil
}

// ─── play ───────────────────────────────────────────────────────────

func cliPlay(query string) error {
	track, err := firstMatch(query)
	if err != nil {
		return err
	}
	fmt.Printf("▶  %s — %s  (%s)\n", track.Title, track.Uploader, fmtDur(track.Duration))

	p := player.New()
	defer p.Shutdown()
	if err := p.Play(track.URL); err != nil {
		return err
	}

	// Ctrl+C stops playback and exits cleanly (mpv is shut down by the
	// deferred Shutdown, so nothing is left running).
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	tty := isTTY()
	for {
		select {
		case <-p.Ended():
			if tty {
				fmt.Print("\r\033[K")
			}
			return nil
		case <-sig:
			if tty {
				fmt.Print("\r\033[K")
			}
			return nil
		case pos := <-p.Positions():
			if tty {
				fmt.Printf("\r   %s / %s\033[K", fmtDur(int(pos.Position)), fmtDur(track.Duration))
			}
		}
	}
}

// ─── download ───────────────────────────────────────────────────────

func cliDownload(query string) error {
	track, err := firstMatch(query)
	if err != nil {
		return err
	}
	set := loadSettings()
	dir := set.ResolveDownloadDir()
	fmt.Printf("⬇  %s — %s  →  %s\n", track.Title, track.Uploader, dir)

	d := downloader.New(dir, set.DownloadFormat)
	defer d.Close()
	d.Enqueue(track.ID, track.Title, track.Uploader, track.URL, dir, track.CoverURL)

	tty := isTTY()
	for evt := range d.Progress() {
		switch evt.Status {
		case downloader.StatusDownloading:
			if tty && evt.Progress > 0 {
				fmt.Printf("\r   %.0f%%\033[K", evt.Progress)
			}
		case downloader.StatusDone, downloader.StatusSkipped:
			if tty {
				fmt.Print("\r\033[K")
			}
			verb := "saved"
			if evt.Status == downloader.StatusSkipped {
				verb = "already downloaded"
			}
			fmt.Printf("✓  %s: %s\n", verb, evt.FilePath)
			return nil
		case downloader.StatusFailed:
			if tty {
				fmt.Print("\r\033[K")
			}
			if evt.Err != nil {
				return evt.Err
			}
			return fmt.Errorf("download failed")
		}
	}
	return fmt.Errorf("download ended without result")
}

// ─── shared helpers ─────────────────────────────────────────────────

// firstMatch returns the top search result for a query.
func firstMatch(query string) (search.Result, error) {
	results, err := search.Search(query, 1)
	if err != nil {
		return search.Result{}, err
	}
	if len(results) == 0 {
		return search.Result{}, fmt.Errorf("no results for %q", query)
	}
	return results[0], nil
}

// loadSettings reads the user's saved settings so CLI downloads share
// the app's directory and format. Falls back to defaults when the
// database is unavailable.
func loadSettings() *settings.Settings {
	database, err := db.Open()
	if err != nil {
		return settings.Defaults()
	}
	defer database.Close()
	s, err := database.LoadSettings()
	if err != nil {
		return settings.Defaults()
	}
	return s
}

// isTTY reports whether stdout is a terminal, so progress lines that
// use carriage returns are skipped when the output is piped to a file.
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func fmtDur(secs int) string {
	if secs <= 0 {
		return "0:00"
	}
	d := time.Duration(secs) * time.Second
	if h := int(d.Hours()); h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}
