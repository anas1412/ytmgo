package gui

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"

	"ytmgo/internal/downloader"
	"ytmgo/internal/queue"
	"ytmgo/internal/settings"
	"ytmgo/internal/ytmusic"
)

// Host is what only the window can do, handed over by cmd/ytmgo-gui
// once it has one: telling the page something happened, and asking the
// user for a folder. Attach sets it; until then both do nothing.
type Host struct {
	// Emit sends an event the page listens for: "downloads" with the
	// job list, "queue" with the queue when a download marks a track.
	Emit func(event string, data any)
	// PickFolder shows the native folder dialog. An empty path is the
	// user cancelling.
	PickFolder func(title string) (string, error)
}

// Attach gives the app its window. A function, not a method, so the
// page cannot call it.
func Attach(a *App, h Host) { a.host = h }

// Close stops the downloads, killing a yt-dlp that is running, so quitting
// does not leave a half-written file behind.
func Close(a *App) {
	if a.quit != nil {
		a.quit()
	}
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	if a.dl != nil {
		a.dl.Close()
	}
}

func (a *App) emit(event string, data any) {
	if a.host.Emit != nil {
		a.host.Emit(event, data)
	}
}

// downloads is what App needs of a downloader.Downloader, so the tests
// can stand one in that runs nothing.
type downloads interface {
	HasPendingJob(trackID string) bool
	IsDownloaded(trackID, title, uploader, outDir string) bool
	Enqueue(trackID, title, uploader, url, outDir, coverURL string)
	EnqueueAs(trackID, title, uploader, url, outDir, coverURL, nameStem string)
	SetFormat(format string)
	Jobs() []*downloader.Job
	Progress() <-chan downloader.ProgressEvent
	Close()
}

// startDownloads starts the downloader on first use, as the TUI does,
// and with it the goroutine that passes its progress on to the page.
// Callers hold a.dlMu.
func (a *App) startDownloads() downloads {
	if a.dl == nil {
		a.mu.Lock()
		format := a.settings.DownloadFormat
		a.mu.Unlock()
		a.dl = a.newDownloader(a.downloadDir(), format)
		go a.watch(a.dl)
	}
	return a.dl
}

func (a *App) downloadDir() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings.ResolveDownloadDir()
}

// watch passes every progress event on to the page, and marks a track
// in the queue as downloaded once it is, so it plays from the file.
func (a *App) watch(dl downloads) {
	for ev := range dl.Progress() {
		if ev.Status == downloader.StatusDone || ev.Status == downloader.StatusSkipped {
			a.mu.Lock()
			queued := false
			for _, t := range a.q.Tracks() {
				queued = queued || t.ID == ev.TrackID
			}
			var s QueueState
			if queued {
				a.q.UpdateTrack(ev.TrackID, func(t *queue.Track) {
					t.Downloaded = true
					if ev.FilePath != "" {
						t.FilePath = ev.FilePath
					}
				})
				s = a.changed()
			}
			a.mu.Unlock()
			if queued {
				a.emit("queue", s)
			}
		}
		a.emit("downloads", jobList(dl))
	}
}

// DownloadJob is one download, as the Downloads page lists it.
type DownloadJob struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Artist   string   `json:"artist"`
	Status   string   `json:"status"`   // "pending", "downloading", "done" or "failed"
	Progress float64  `json:"progress"` // 0–100
	Problem  *Problem `json:"problem"`  // why it failed
}

var statusNames = map[downloader.Status]string{
	downloader.StatusPending:     "pending",
	downloader.StatusDownloading: "downloading",
	downloader.StatusDone:        "done",
	downloader.StatusFailed:      "failed",
	downloader.StatusSkipped:     "done",
}

// Downloads is every download this run, in the order they were asked
// for.
func (a *App) Downloads() []DownloadJob {
	a.dlMu.Lock()
	dl := a.dl
	a.dlMu.Unlock()
	if dl == nil {
		return []DownloadJob{}
	}
	return jobList(dl)
}

func jobList(dl downloads) []DownloadJob {
	out := []DownloadJob{}
	for _, j := range dl.Jobs() {
		d := DownloadJob{ID: j.TrackID, Title: j.Title, Artist: j.Uploader, Status: statusNames[j.Status], Progress: j.Progress}
		if j.Err != nil {
			d.Problem = problem(j.Err).(*Problem)
		}
		out = append(out, d)
	}
	return out
}

// What Download did, for the page to say.
const (
	Queued        = "queued"
	AlreadyQueued = "pending"
	AlreadyThere  = "downloaded"
)

// ToolFetch is how far the download of a missing tool has got, sent
// to the page as the "tool" event. Total is -1 when unknown.
type ToolFetch struct {
	Name  string `json:"name"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// needFFmpeg makes sure ffmpeg is there before anything is downloaded:
// on Windows it is not installed with the app, but fetched the first
// time it is needed. Downloads that ask meanwhile wait for the same
// fetch.
func (a *App) needFFmpeg() error {
	if a.fetchTool == nil {
		return nil
	}
	a.toolMu.Lock()
	defer a.toolMu.Unlock()
	last := int64(-1)
	return problem(a.fetchTool("ffmpeg", func(done, total int64) {
		// A percent at a time: the page needs no more.
		if pct := done * 100 / max(total, 1); total <= 0 || pct != last {
			last = pct
			a.emit("tool", ToolFetch{Name: "ffmpeg", Done: done, Total: total})
		}
	}))
}

// Download downloads t into the download folder: the TUI's x. A song
// already downloading, or already on disk in the current format, is
// left alone.
func (a *App) Download(t queue.Track) (string, error) {
	if err := a.needFFmpeg(); err != nil {
		return "", err
	}
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	dl, dir := a.startDownloads(), a.downloadDir()
	switch {
	case dl.HasPendingJob(t.ID):
		return AlreadyQueued, nil
	case dl.IsDownloaded(t.ID, t.Title, t.Artist, dir):
		return AlreadyThere, nil
	}
	// An empty URL is the downloader's to resolve.
	dl.Enqueue(t.ID, t.Title, t.Artist, t.URL, dir, t.CoverURL)
	a.emit("downloads", jobList(dl))
	return Queued, nil
}

// DownloadAlbum downloads a whole album into its own folder, "Artist -
// Album", its files numbered in album order, and says how many songs it
// queued.
func (a *App) DownloadAlbum(browseID string) (int, error) {
	if browseID == "" {
		return 0, problem(errNoAlbum)
	}
	full, err := a.album(browseID)
	if err != nil {
		return 0, problem(err)
	}
	if err := a.needFFmpeg(); err != nil {
		return 0, err
	}
	a.dlMu.Lock()
	defer a.dlMu.Unlock()
	dl := a.startDownloads()
	dir := downloader.AlbumDir(a.downloadDir(), full.Artist, full.Title)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return 0, problem(err)
	}
	for i, t := range full.Tracks {
		stem := downloader.AlbumStem(i+1, len(full.Tracks), t.Title)
		dl.EnqueueAs(t.VideoID, t.Title, full.Artist, ytmusic.WatchURL(t.VideoID), dir, t.CoverURL, stem)
	}
	a.emit("downloads", jobList(dl))
	return len(full.Tracks), nil
}

// OpenDownloads opens the download folder in Explorer, making it first
// if nothing has been downloaded yet.
func (a *App) OpenDownloads() error {
	dir := a.downloadDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return problem(err)
	}
	return problem(a.open(dir))
}

// SetDownloadFormat sets the format new downloads are saved in: "m4a"
// or "mp3".
func (a *App) SetDownloadFormat(f string) string {
	if f != settings.FormatMP3 {
		f = settings.FormatM4A
	}
	a.mu.Lock()
	a.settings.DownloadFormat = f
	a.saveSettings(a.settings)
	a.mu.Unlock()
	a.dlMu.Lock()
	if a.dl != nil {
		a.dl.SetFormat(f)
	}
	a.dlMu.Unlock()
	return f
}

// ChooseDownloadFolder asks the user for a new download folder and
// returns the folder downloads now go to, which is the old one if they
// cancel.
func (a *App) ChooseDownloadFolder() (string, error) {
	if a.host.PickFolder == nil {
		return a.downloadDir(), nil
	}
	dir, err := a.host.PickFolder("Download folder")
	if err != nil {
		return a.downloadDir(), problem(err)
	}
	if dir != "" {
		a.mu.Lock()
		a.settings.DownloadDir = dir
		a.saveSettings(a.settings)
		a.mu.Unlock()
	}
	return a.downloadDir(), nil
}

// PlaySource is where the page's <audio> plays t from: the file, for a
// track downloaded and still on disk, else a fresh stream. Streams
// expire after some hours, so the page asks each time a track starts.
//
// A file is served by Media under /local/, by track id. Only files
// handed out here are served, so the page cannot read others off disk.
func (a *App) PlaySource(t queue.Track) (string, error) {
	a.mu.Lock()
	path := ""
	for _, have := range append(a.q.Tracks(), t) {
		if have.ID == t.ID && have.Downloaded && have.FilePath != "" {
			path = have.FilePath
			break
		}
	}
	a.mu.Unlock()
	if path != "" {
		if _, err := os.Stat(path); err == nil {
			a.localMu.Lock()
			if a.local == nil {
				a.local = map[string]string{}
			}
			a.local[t.ID] = path
			a.localMu.Unlock()
			return "/local/" + url.PathEscape(t.ID), nil
		}
	}
	if t.URL == "" {
		return "", problem(errNoSource)
	}
	src, err := a.streamURL(t.URL)
	return src, problem(err)
}

var errNoSource = errors.New("this track has no link to play it from")

// Media serves the files PlaySource hands out, with ranges, so the
// player can seek in them. cmd/ytmgo-gui gives it to Wails' asset server,
// which asks it for whatever is not one of the page's own files.
func Media(a *App) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutPrefix(r.URL.Path, "/local/")
		a.localMu.Lock()
		path := a.local[id]
		a.localMu.Unlock()
		if !ok || path == "" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, path)
	})
}
