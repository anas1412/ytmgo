package tui

import (
	"testing"

	"ytmgo/internal/player"
	"ytmgo/internal/search"

	"ytmgo/internal/ytmusic"

	tea "github.com/charmbracelet/bubbletea"
)

var albumForTest = ytmusic.Album{BrowseID: "MPREb_x", Title: "An Album"}

func enqueueModel(t *testing.T, n int) Model {
	t.Helper()
	isolateUserDirs(t)
	m := InitialModel()
	m.npOn = false
	// Playing, so enqueueing does not try to start mpv from a unit test.
	m.playerState = player.StatePlaying
	m.activePage = PageStream
	m.activePanel = PanelSearch
	m.showingRecommendations = false
	for i := range n {
		m.results = append(m.results, search.Result{
			ID: "aaaaaaaaaa" + string(rune('a'+i)), Title: "Song", Uploader: "Artist", URL: "u",
		})
	}
	return m
}

func pressE(t *testing.T, m Model) Model {
	t.Helper()
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	return nm.(Model)
}

func pressUppercaseE(t *testing.T, m Model) Model {
	t.Helper()
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'E'}})
	return nm.(Model)
}

// E downloads the whole list on screen — the download-queue sibling of
// e. Every track lands as a download job.
func TestBulkDownloadDownloadsTheList(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // keep the worker's exec from launching a real yt-dlp
	m := enqueueModel(t, 5)
	m = pressUppercaseE(t, m)
	if m.downloader == nil || len(m.downloader.Jobs()) != 5 {
		t.Fatalf("E queued %d download jobs, want all 5", len(m.downloader.Jobs()))
	}
	if m.activePage != PageDownloads {
		t.Errorf("E left the user on page %v, want the downloads page", m.activePage)
	}
}

// With the focus in the queue the subject is the queue, not the list
// beside it — the same rule e follows.
func TestBulkDownloadIgnoredFromQueuePanel(t *testing.T) {
	m := enqueueModel(t, 5)
	m.activePanel = PanelQueue
	if got := pressUppercaseE(t, m); got.downloader != nil && len(got.downloader.Jobs()) != 0 {
		t.Fatalf("E queued download jobs with the focus in the queue, want none")
	}
}

// An album grid is releases, not tracks; there is nothing to download.
func TestBulkDownloadSkipsAlbumGrid(t *testing.T) {
	m := enqueueModel(t, 5)
	m.albumMode = true
	if got := pressUppercaseE(t, m); got.downloader != nil && len(got.downloader.Jobs()) != 0 {
		t.Fatalf("E queued download jobs from the album grid, want none")
	}
}

// Already-queued tracks are not enqueued a second time.
func TestBulkDownloadSkipsQueuedTracks(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := enqueueModel(t, 5)
	m = pressUppercaseE(t, m) // queues all 5
	before := len(m.downloader.Jobs())
	m = pressUppercaseE(t, m) // all pending already — nothing new
	if got := len(m.downloader.Jobs()); got != before {
		t.Fatalf("E queued %d jobs on a second press, want it to stay at %d", got, before)
	}
}

// e queues the whole list on screen — the reason a pasted playlist is
// worth having, since adding a hundred tracks one Enter at a time is not.
func TestEnqueueAllQueuesTheList(t *testing.T) {
	m := enqueueModel(t, 5)
	m = pressE(t, m)
	if got := m.queue.Len(); got != 5 {
		t.Fatalf("queue has %d tracks, want all 5", got)
	}
	// Playlist order must survive.
	for i, tr := range m.queue.Tracks() {
		if tr.ID != m.results[i].ID {
			t.Errorf("position %d: queued %q, want %q", i, tr.ID, m.results[i].ID)
		}
	}
}

// With the focus in the queue the subject is the queue, not the list
// beside it — the same rule a and A follow.
func TestEnqueueAllIgnoredFromQueuePanel(t *testing.T) {
	m := enqueueModel(t, 5)
	m.activePanel = PanelQueue
	if got := pressE(t, m).queue.Len(); got != 0 {
		t.Fatalf("queued %d tracks with the focus in the queue, want none", got)
	}
}

// An album grid is releases, not tracks; there is nothing to enqueue.
func TestEnqueueAllSkipsAlbumGrid(t *testing.T) {
	m := enqueueModel(t, 5)
	m.albumMode = true
	if got := pressE(t, m).queue.Len(); got != 0 {
		t.Fatalf("queued %d tracks from the album grid, want none", got)
	}
}

// Inside an album e queues that tracklist, not the stale search results.
func TestEnqueueAllPrefersTheOpenTracklist(t *testing.T) {
	m := enqueueModel(t, 5)
	m.openAlbum = &albumForTest
	m.albumTracks = []search.Result{{ID: "bbbbbbbbbbb", Title: "Album cut", URL: "u"}}
	m = pressE(t, m)
	if got := m.queue.Len(); got != 1 {
		t.Fatalf("queued %d tracks, want the 1 album track", got)
	}
	if id := m.queue.Tracks()[0].ID; id != "bbbbbbbbbbb" {
		t.Errorf("queued %q, want the album track", id)
	}
}

// a still queues an open album — it shares queueAll with e now, so this
// guards the refactor that gave them one loop instead of two.
func TestAlbumKeyStillQueuesTheAlbum(t *testing.T) {
	m := enqueueModel(t, 0)
	m.openAlbum = &albumForTest
	m.albumTracks = []search.Result{
		{ID: "ccccccccccc", Title: "One", URL: "u"},
		{ID: "ddddddddddd", Title: "Two", URL: "u"},
	}
	nm, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = nm.(Model)
	if got := m.queue.Len(); got != 2 {
		t.Fatalf("queue has %d tracks, want the album's 2", got)
	}
	if id := m.queue.Tracks()[0].ID; id != "ccccccccccc" {
		t.Errorf("first queued %q, want album order", id)
	}
}
