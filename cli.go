package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ytmgo/internal/db"
	"ytmgo/internal/discordrpc"
	"ytmgo/internal/downloader"
	"ytmgo/internal/player"
	"ytmgo/internal/search"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
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

Options:
  -a, --album              work on albums instead of single tracks
  -f, --format <m4a|mp3>   override the configured audio format   (download)
  -l, --location <dir>     save somewhere else, "." for here      (download)

An album download creates its own folder and numbers the tracks:
  ytmgo download -a timeline mild high club
  → "Mild High Club - Timeline/01 - Club Intro.m4a", …

While the player is running, your keyboard's media keys control it.
`

// Sentinel errors so the dispatcher can map a cause to an exit code:
// misuse exits 2, a genuine failure exits 1.
var (
	errNoQuery = errors.New("missing search query")
	errBadFlag = errors.New("invalid option")
)

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
		var err error
		switch cmd {
		case "search":
			err = cliSearch(rest)
		case "play":
			err = cliPlay(rest)
		case "download":
			err = cliDownload(rest)
		}
		switch {
		case errors.Is(err, errNoQuery):
			fmt.Fprintf(os.Stderr, "ytmgo %s: missing search query\n\nUsage: ytmgo %s <query>\n", cmd, cmd)
			return true, 2
		case errors.Is(err, errBadFlag):
			fmt.Fprintf(os.Stderr, "ytmgo %s: %v\n\n%s", cmd, err, cliUsage)
			return true, 2
		case err != nil:
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

// flagsWithValues names the options that consume the next argument, so
// hoistFlags knows "-f mp3" is one option and not an option plus a
// query word.
var flagsWithValues = map[string]bool{"f": true, "format": true, "l": true, "location": true}

// hoistFlags separates options from query words wherever they appear.
// Go's flag package stops parsing at the first non-flag argument, so
// without this "ytmgo download bebalee -f mp3" would silently ignore
// -f and fold "-f mp3" into the search text. Everything after a bare
// "--" is treated as literal query text.
func hoistFlags(args []string) (flags, query []string) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			query = append(query, args[i+1:]...)
			break
		}
		if len(a) > 1 && strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			name := strings.TrimLeft(a, "-")
			// "--format=mp3" carries its value already.
			if strings.Contains(name, "=") {
				continue
			}
			if flagsWithValues[name] && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		query = append(query, a)
	}
	return flags, query
}

// joinQuery rebuilds the query from argv so quoting is optional:
// `ytmgo play homage mild high club` works without quotes.
func joinQuery(args []string) string {
	out := ""
	for _, a := range args {
		// Our own listings print "Title — Artist", so a pasted-back line
		// arrives with a standalone dash that matches nothing upstream.
		if a == "" || a == "—" || a == "–" {
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

func cliSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var album bool
	fs.BoolVar(&album, "album", false, "search albums instead of tracks")
	fs.BoolVar(&album, "a", false, "shorthand for --album")
	flags, words := hoistFlags(args)
	if err := fs.Parse(flags); err != nil {
		return fmt.Errorf("%w: %v", errBadFlag, err)
	}
	query := joinQuery(words)
	if query == "" {
		return errNoQuery
	}

	if album {
		albums, err := ytmusic.SearchAlbums(query, cliSearchLimit)
		if err != nil {
			return err
		}
		if len(albums) == 0 {
			return fmt.Errorf("no albums for %q", query)
		}
		for i, a := range albums {
			year := ""
			if a.Year != "" {
				year = "  (" + a.Year + ")"
			}
			fmt.Printf("%2d. %s — %s%s\n", i+1, a.Title, a.Artist, year)
		}
		return nil
	}

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

func cliPlay(args []string) error {
	// play takes no options. Reject them rather than letting them fall
	// through as search text, where "-a album" would silently look for a
	// song literally called "-a album".
	flags, words := hoistFlags(args)
	if len(flags) > 0 {
		return fmt.Errorf("%w: play takes no options (got %s)", errBadFlag, flags[0])
	}
	query := joinQuery(words)
	if query == "" {
		return errNoQuery
	}
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

	// Show the track on Discord while it plays, same as the TUI does —
	// and honour the same Settings toggle. Discord not running (or the
	// feature disabled) is not an error; playback carries on regardless.
	// One Update is enough: it sends a start timestamp, so Discord
	// counts the elapsed time itself.
	if loadSettings().DiscordRPCEnabled {
		if err := discordrpc.Init(); err == nil {
			defer discordrpc.Close()
			discordrpc.Update(track.ToTrack(), player.StatePlaying, 0)
		}
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

// cliDownload parses its own flags so the format and destination can be
// overridden per invocation; both default to the app's saved settings.
//
//	ytmgo download -f mp3 -l . homage
func cliDownload(args []string) error {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // errors are reported by the dispatcher
	var format, location string
	var album bool
	fs.BoolVar(&album, "album", false, "download a whole album into its own folder")
	fs.BoolVar(&album, "a", false, "shorthand for --album")
	fs.StringVar(&format, "format", "", "audio format: m4a or mp3")
	fs.StringVar(&format, "f", "", "shorthand for --format")
	fs.StringVar(&location, "location", "", "directory to save into")
	fs.StringVar(&location, "l", "", "shorthand for --location")
	flags, words := hoistFlags(args)
	if err := fs.Parse(flags); err != nil {
		return fmt.Errorf("%w: %v", errBadFlag, err)
	}

	query := joinQuery(words)
	if query == "" {
		return errNoQuery
	}

	set := loadSettings()
	fmtOut, err := resolveFormat(format, set)
	if err != nil {
		return fmt.Errorf("%w: %v", errBadFlag, err)
	}
	dir, err := resolveLocation(location, set)
	if err != nil {
		return err
	}

	if album {
		return downloadAlbum(query, dir, fmtOut)
	}

	track, err := firstMatch(query)
	if err != nil {
		return err
	}
	fmt.Printf("⬇  %s — %s  →  %s\n", track.Title, track.Uploader, dir)

	d := downloader.New(dir, fmtOut)
	defer d.Close()
	d.Enqueue(track.ID, track.Title, track.Uploader, track.URL, dir, track.CoverURL)

	tty := isTTY()
	for evt := range d.Progress() {
		switch evt.Status {
		case downloader.StatusDownloading:
			if tty && evt.Progress > 0 {
				fmt.Printf("\r   %s %5.1f%%\033[K", progressBar(evt.Progress, 30), evt.Progress)
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

// downloadAlbum fetches an album's tracklist and downloads every track
// into "<parent>/<Artist> - <Album>/", numbered so the folder sorts in
// album order. One failed track is reported but does not abort the rest.
func downloadAlbum(query, parentDir, format string) error {
	albums, err := ytmusic.SearchAlbums(query, 1)
	if err != nil {
		return err
	}
	if len(albums) == 0 {
		return fmt.Errorf("no album found for %q — try `ytmgo search -a %s` to see what matches", query, query)
	}
	alb, err := ytmusic.AlbumTracks(albums[0].BrowseID)
	if err != nil {
		return err
	}

	folder := alb.Title
	if alb.Artist != "" {
		folder = alb.Artist + " - " + alb.Title
	}
	dir := filepath.Join(parentDir, sanitizePathSegment(folder))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create %s: %w", dir, err)
	}

	year := ""
	if alb.Year != "" {
		year = " (" + alb.Year + ")"
	}
	fmt.Printf("⬇  %s — %s%s · %d tracks  →  %s\n", alb.Title, alb.Artist, year, len(alb.Tracks), dir)

	d := downloader.New(dir, format)
	defer d.Close()

	tty := isTTY()
	// Number width follows the tracklist: 9 tracks -> "1", 10+ -> "01".
	width := len(fmt.Sprintf("%d", len(alb.Tracks)))
	var failed int

	for i, t := range alb.Tracks {
		n := i + 1
		stem := fmt.Sprintf("%0*d - %s", width, n, t.Title)
		label := fmt.Sprintf("[%d/%d] %s", n, len(alb.Tracks), t.Title)

		d.EnqueueAs(t.VideoID, t.Title, alb.Artist, ytmusic.WatchURL(t.VideoID), dir, t.CoverURL, stem)

		// One job at a time: consume events until this one settles.
		done := false
		for !done {
			evt, ok := <-d.Progress()
			if !ok {
				return fmt.Errorf("downloader closed unexpectedly")
			}
			switch evt.Status {
			case downloader.StatusDownloading:
				if tty && evt.Progress > 0 {
					fmt.Printf("\r   %s %s %5.1f%%\033[K", label, progressBar(evt.Progress, 20), evt.Progress)
				}
			case downloader.StatusDone, downloader.StatusSkipped:
				if tty {
					fmt.Print("\r\033[K")
				}
				mark := "✓"
				if evt.Status == downloader.StatusSkipped {
					mark = "·"
				}
				fmt.Printf("   %s %s\n", mark, label)
				done = true
			case downloader.StatusFailed:
				if tty {
					fmt.Print("\r\033[K")
				}
				failed++
				fmt.Fprintf(os.Stderr, "   ✗ %s: %v\n", label, evt.Err)
				done = true
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("%d of %d tracks failed (the rest are in %s)", failed, len(alb.Tracks), dir)
	}
	fmt.Printf("✓  album saved: %s\n", dir)
	return nil
}

// sanitizePathSegment strips characters that are awkward in a directory
// name. Mirrors the downloader's filename sanitising.
func sanitizePathSegment(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// resolveFormat validates a --format override, falling back to the
// configured format when the flag is absent.
func resolveFormat(f string, set *settings.Settings) (string, error) {
	if f == "" {
		return set.DownloadFormat, nil
	}
	switch strings.ToLower(f) {
	case settings.FormatM4A, settings.FormatMP3:
		return strings.ToLower(f), nil
	}
	return "", fmt.Errorf("unsupported format %q (use m4a or mp3)", f)
}

// resolveLocation turns a --location override into an absolute
// directory, creating it if needed. "." means the current directory and
// a leading "~" is expanded; empty falls back to the configured
// download directory.
func resolveLocation(loc string, set *settings.Settings) (string, error) {
	if loc == "" {
		return set.ResolveDownloadDir(), nil
	}
	if loc == "~" || strings.HasPrefix(loc, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot expand ~: %w", err)
		}
		loc = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(loc, "~"), "/"))
	}
	abs, err := filepath.Abs(loc)
	if err != nil {
		return "", fmt.Errorf("bad location %q: %w", loc, err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return "", fmt.Errorf("cannot use %s: %w", abs, err)
	}
	return abs, nil
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

// progressBar renders a fixed-width fill bar for the download percentage.
func progressBar(pct float64, width int) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct / 100 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
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
