package gui

import (
	"strings"

	"ytmgo/internal/discordrpc"
	"ytmgo/internal/library"
	"ytmgo/internal/player"
	"ytmgo/internal/queue"
	"ytmgo/internal/ytmusic"
)

// ShareLink is t's link for sharing, the TUI's u: on youtube.com, or on
// music.youtube.com with Copy Link As. Empty for a song with no link, a
// local file or an old history entry.
func (a *App) ShareLink(t queue.Track) string {
	if !ytmusic.IsVideoID(t.ID) {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return ytmusic.ShareURL(t.ID, a.settings.CopyMusicLinks)
}

// SetCopyMusicLinks sets which site copied links point to, and saves it.
func (a *App) SetCopyMusicLinks(music bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings.CopyMusicLinks = music
	a.saveSettings(a.settings)
	return music
}

// Presence tells Discord what is playing, as the TUI does: the song,
// with its elapsed time while it plays, or "Browsing Music" with
// nothing (t nil). It connects on the first call after Discord starts,
// and does nothing while Discord RPC is off in Settings.
func (a *App) Presence(t *queue.Track, playing bool, position float64) {
	a.mu.Lock()
	on := a.settings.DiscordRPCEnabled
	a.mu.Unlock()
	if !on || a.discord == nil {
		return
	}
	if t != nil && t.CoverURL == "" {
		c := *t
		c.CoverURL = a.publicCover(c)
		t = &c
	}
	a.discord.update(t, playing, position)
}

// publicCover finds a cover Discord can show for a song with none of
// its own, a library file: Discord shows only pictures on the web, and
// a file's cover is inside the file. The same song in the queue or the
// favourites may carry one; failing that, YouTube Music is asked once,
// and the answer, even none, is kept.
func (a *App) publicCover(t queue.Track) string {
	key := library.MatchKey(t)
	a.mu.Lock()
	for _, have := range append(a.q.Tracks(), a.favs...) {
		if have.CoverURL != "" && library.MatchKey(have) == key {
			a.mu.Unlock()
			return have.CoverURL
		}
	}
	url, asked := a.covers[key]
	a.mu.Unlock()
	if asked || t.Title == "" || a.search == nil {
		return url
	}
	if results, err := a.search(strings.TrimSpace(t.Artist+" "+t.Title), 1); err == nil && len(results) > 0 {
		url = results[0].CoverURL
	} else if err != nil {
		return "" // offline, say: asked again next time
	}
	a.mu.Lock()
	if a.covers == nil {
		a.covers = map[string]string{}
	}
	a.covers[key] = url
	a.mu.Unlock()
	return url
}

// SetDiscord turns Discord RPC on or off, and saves it. Off clears what
// Discord shows.
func (a *App) SetDiscord(on bool) bool {
	a.mu.Lock()
	a.settings.DiscordRPCEnabled = on
	a.saveSettings(a.settings)
	a.mu.Unlock()
	if !on && a.discord != nil {
		a.discord.close()
	}
	return on
}

// discord is discordrpc, which tests replace.
type discord struct {
	update func(t *queue.Track, playing bool, position float64)
	close  func()
}

var realDiscord = &discord{
	update: func(t *queue.Track, playing bool, position float64) {
		if discordrpc.Init() != nil {
			return // not running; the next call tries again
		}
		if t == nil {
			discordrpc.ShowIdle()
			return
		}
		state := player.StatePaused
		if playing {
			state = player.StatePlaying
		}
		discordrpc.Update(*t, state, position)
	},
	close: discordrpc.Close,
}
