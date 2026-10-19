package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ytmgo/internal/db"
	"ytmgo/internal/ytmusic"
)

// Artists followed and albums liked: b (bookmark) on an artist's page
// or an album's, kept in saved_items with the GUI, and listed on the
// Favs page's Artists and Albums tabs (2 again steps through them).

const (
	favSongs = iota
	favArtists
	favAlbums
)

var favTabNames = []string{"Songs", "Artists", "Albums"}

// savedList is what the Favs page's tab lists, when it is Artists or
// Albums.
func (m Model) savedList() []db.Saved {
	switch m.favTab {
	case favArtists:
		return m.savedArtists
	case favAlbums:
		return m.savedAlbums
	}
	return nil
}

func (m Model) isSaved(kind, id string) bool {
	list := m.savedAlbums
	if kind == "artist" {
		list = m.savedArtists
	}
	for _, s := range list {
		if s.ID == id {
			return true
		}
	}
	return false
}

func (m *Model) reloadSaved() {
	if m.db == nil {
		return
	}
	if a, err := m.db.LoadSaved("artist"); err == nil {
		m.savedArtists = a
	}
	if a, err := m.db.LoadSaved("album"); err == nil {
		m.savedAlbums = a
	}
	m.clampSavedOffset()
}

// toggleSavedAction is b: on an album's page it likes or unlikes the
// album, on an artist's it follows or unfollows them; on the Favs
// page's Artists or Albums tab it lets go of the one selected.
func (m *Model) toggleSavedAction() tea.Cmd {
	var s db.Saved
	switch {
	case m.activePage == PageFavorites && m.favTab != favSongs:
		list := m.savedList()
		if m.savedCursor < 0 || m.savedCursor >= len(list) {
			return nil
		}
		s = list[m.savedCursor]
	case m.activePage == PageStream && m.openAlbum != nil:
		a := m.openAlbum
		s = db.Saved{Kind: "album", ID: a.BrowseID, Title: a.Title, Subtitle: a.Artist, CoverURL: a.CoverURL}
	case m.activePage == PageStream && m.openArtist != nil:
		a := m.openArtist
		s = db.Saved{Kind: "artist", ID: a.BrowseID, Title: a.Name, CoverURL: a.ThumbURL}
	default:
		m.setStatus("b follows an artist, or likes an album, on its page")
		return nil
	}
	if m.db == nil || s.ID == "" {
		return nil
	}
	saved := m.isSaved(s.Kind, s.ID)
	var err error
	if saved {
		err = m.db.Unsave(s.Kind, s.ID)
	} else {
		err = m.db.Save(s)
	}
	if err != nil {
		m.setStatus("Could not save: " + err.Error())
		return nil
	}
	m.reloadSaved()
	switch {
	case s.Kind == "artist" && saved:
		m.setStatus("Unfollowed " + s.Title)
	case s.Kind == "artist":
		m.setStatus("Following " + s.Title)
	case saved:
		m.setStatus("Unliked " + s.Title)
	default:
		m.setStatus("Liked " + s.Title)
	}
	return nil
}

// switchFavTab is 2 pressed again on the Favs page.
func (m *Model) switchFavTab(step int) {
	m.favTab = (m.favTab + step + len(favTabNames)) % len(favTabNames)
	m.savedCursor, m.savedOffset = 0, 0
}

// openSavedSelected is Enter on the Artists or Albums tab: the page
// opens where pages open, on the Stream page.
func (m *Model) openSavedSelected() tea.Cmd {
	list := m.savedList()
	if m.savedCursor < 0 || m.savedCursor >= len(list) {
		return nil
	}
	s := list[m.savedCursor]
	m.switchPage(PageStream)
	m.browseLoadingName = s.Title
	m.setStatus("Opening " + s.Title + "…")
	if s.Kind == "artist" {
		m.isLoadingArtist = true
		m.artistSeq++
		return openArtistCmd(s.ID, m.artistSeq)
	}
	m.isLoadingAlbum = true
	m.albumSeq++
	m.albumCoverURL = s.CoverURL
	return openAlbumCmd(ytmusic.Album{BrowseID: s.ID}, m.albumSeq)
}

func (m *Model) moveSavedCursor(step int) {
	m.savedCursor += step
	m.clampSavedOffset()
}

func (m *Model) clampSavedOffset() {
	n := len(m.savedList())
	vis := m.visibleItems()
	m.savedCursor = max(0, min(m.savedCursor, n-1))
	if m.savedCursor < m.savedOffset {
		m.savedOffset = m.savedCursor
	}
	if m.savedCursor >= m.savedOffset+vis {
		m.savedOffset = m.savedCursor - vis + 1
	}
	m.savedOffset = max(0, m.savedOffset)
}

// favTabsLine is the Favs page's tab row: the one shown bright, the
// others dim, with how many each holds.
func (m Model) favTabsLine(width int) string {
	counts := []int{len(m.favorites), len(m.savedArtists), len(m.savedAlbums)}
	var parts []string
	for i, name := range favTabNames {
		label := fmt.Sprintf("%s %d", name, counts[i])
		if i == m.favTab {
			parts = append(parts, styleNowTitle.Render(label))
		} else {
			parts = append(parts, styleTextDim.Render(label))
		}
	}
	return truncate(strings.Join(parts, styleTextDim.Render("  ·  "))+styleTextDim.Render("   2 switch"), width)
}

// renderSaved lists the Artists or Albums tab: Enter opens one, b
// lets it go.
func (m Model) renderSaved(width, height int) string {
	list := m.savedList()
	rowW := max(1, width-2)
	if len(list) == 0 {
		hint := "No artists followed yet — press b on an artist's page"
		if m.favTab == favAlbums {
			hint = "No albums liked yet — press b on an album's page"
		}
		return styleEmpty.Width(width - 2).Height(height).Render(hint)
	}
	var lines []string
	maxItems := max(1, (height-1)/2)
	end := min(m.savedOffset+maxItems, len(list))
	for i := m.savedOffset; i < end; i++ {
		s := list[i]
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.savedCursor
		line := truncate(fmt.Sprintf("%d. %s", i+1, s.Title), rowW)
		info := "   Artist"
		if s.Kind == "album" {
			info = "   " + s.Subtitle
		}
		right := "♥  "
		spacing := max(1, rowW-lipgloss.Width(info)-lipgloss.Width(right))
		info = truncate(info, rowW-lipgloss.Width(right)-1) + strings.Repeat(" ", spacing) + right
		lines = append(lines, renderListItemBlock(line, truncate(info, rowW), isSelected, false, rowW))
	}
	if ind := scrollIndicator(m.savedOffset, len(list)-end, m.savedCursor+1, len(list)); ind != "" {
		lines = append(lines, ind)
	}
	result := padToWidth(strings.Join(lines, "\n"), rowW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(make([]string, height-cnt), "\n"+strings.Repeat(" ", rowW))
	}
	return result
}

// savedHint is what the album or artist page header says about b.
func (m Model) savedHint() string {
	switch {
	case m.openAlbum != nil && m.isSaved("album", m.openAlbum.BrowseID):
		return "♥ liked · [b] unlike"
	case m.openAlbum != nil:
		return "[b] like"
	case m.openArtist != nil && m.isSaved("artist", m.openArtist.BrowseID):
		return "♥ following · [b] unfollow"
	case m.openArtist != nil:
		return "[b] follow"
	}
	return ""
}
