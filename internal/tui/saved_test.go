package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"ytmgo/internal/ytmusic"
)

func pressKeys(m Model, keys ...string) Model {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		nm, _ := m.handleKey(msg)
		m = nm.(Model)
	}
	return m
}

// b likes the album on screen, and again unlikes it; the header says
// which, and the Favs page's Albums tab lists it.
func TestLikeAnAlbum(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	if m.db == nil {
		t.Skip("no database")
	}
	m.activePage, m.activePanel = PageStream, PanelSearch
	m.openAlbum = &ytmusic.Album{BrowseID: "MPREb_disc", Title: "Discovery", Artist: "Daft Punk"}
	if m.savedHint() != "[b] like" {
		t.Errorf("hint %q", m.savedHint())
	}
	m = pressKeys(m, "b")
	if !m.isSaved("album", "MPREb_disc") || !strings.Contains(m.statusMessage, "Liked Discovery") || m.savedHint() != "♥ liked · [b] unlike" {
		t.Fatalf("after F: saved %v, status %q, hint %q", m.isSaved("album", "MPREb_disc"), m.statusMessage, m.savedHint())
	}

	m.openAlbum = nil
	m.switchPage(PageFavorites)
	m.activePanel = PanelSearch
	m.switchFavTab(1)
	m.switchFavTab(1)
	if m.favTab != favAlbums || len(m.savedList()) != 1 || m.savedList()[0].Subtitle != "Daft Punk" {
		t.Fatalf("albums tab: tab %d, %+v", m.favTab, m.savedList())
	}
	if out := m.renderFavorites(80, 20); !strings.Contains(out, "Discovery") || !strings.Contains(out, "Albums 1") {
		t.Errorf("drawn as:\n%s", out)
	}
	// b on the tab lets it go.
	m = pressKeys(m, "b")
	if len(m.savedAlbums) != 0 {
		t.Errorf("still liked: %+v", m.savedAlbums)
	}
}

// Following an artist, then Enter on the Artists tab opens their page.
func TestFollowedArtistOpens(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	if m.db == nil {
		t.Skip("no database")
	}
	m.activePage, m.activePanel = PageStream, PanelSearch
	m.openArtist = &ytmusic.ArtistPage{BrowseID: "UCdaft", Name: "Daft Punk"}
	m = pressKeys(m, "b")
	if !m.isSaved("artist", "UCdaft") || m.savedHint() != "♥ following · [b] unfollow" {
		t.Fatalf("follow: %v, %q", m.isSaved("artist", "UCdaft"), m.savedHint())
	}
	m.openArtist = nil
	m.switchPage(PageFavorites)
	m.activePanel = PanelSearch
	m.switchFavTab(1)
	cmd := m.openSavedSelected()
	if cmd == nil || m.activePage != PageStream || !m.isLoadingArtist {
		t.Errorf("enter: cmd %v, page %v, loading %v", cmd != nil, m.activePage, m.isLoadingArtist)
	}
}

// 2 opens Favs on Songs; 2 again steps to Artists, Albums, and back.
func TestTwoAgainStepsFavTabs(t *testing.T) {
	m := worstCaseModel(t, 150, 40)
	m.activePage = PageStream
	var tabs []int
	for i := 0; i < 4; i++ {
		m = pressKeys(m, "2")
		tabs = append(tabs, m.favTab)
	}
	if m.activePage != PageFavorites || tabs[0] != favSongs || tabs[1] != favArtists || tabs[2] != favAlbums || tabs[3] != favSongs {
		t.Errorf("page %v, tabs %v", m.activePage, tabs)
	}
}
