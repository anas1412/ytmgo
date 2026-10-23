package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ytmgo/internal/db"
	"ytmgo/internal/queue"
)

// Playlists of your own, kept in the database with the GUI's: page 6
// lists them, Enter opens one, esc steps back out. m puts the song the
// cursor is on into one, through a picker drawn in the left panel; M
// saves the whole list on screen as a new one, which is how a pasted
// playlist is kept.

// plPicker is m's picker, or M's name prompt, while it is open.
type plPicker struct {
	tracks []queue.Track // what goes in
	what   string        // what the title calls it: a song, or "12 songs"
	cursor int           // 0 is New playlist…, then m.playlists in order
	naming bool          // typing the new playlist's name
	name   textinput.Model
}

func (m *Model) reloadPlaylists() {
	if m.db == nil {
		return
	}
	if list, err := m.db.LoadPlaylists(); err == nil {
		m.playlists = list
	}
	m.clampPlOffset()
}

// plLen is how many rows the Playlists page lists: playlists, or the
// open playlist's songs.
func (m Model) plLen() int {
	if m.plOpen != nil {
		return len(m.plOpen.Tracks)
	}
	return len(m.playlists)
}

func (m *Model) movePlCursor(step int) {
	m.plCursor += step
	m.clampPlOffset()
}

func (m *Model) clampPlOffset() {
	n, vis := m.plLen(), m.visibleItems()
	m.plCursor = max(0, min(m.plCursor, n-1))
	if m.plCursor < m.plOffset {
		m.plOffset = m.plCursor
	}
	if m.plCursor >= m.plOffset+vis {
		m.plOffset = m.plCursor - vis + 1
	}
	m.plOffset = max(0, m.plOffset)
}

// openPlaylist is Enter on the list: its songs replace it.
func (m *Model) openPlaylist() {
	if m.db == nil || m.plCursor >= len(m.playlists) {
		return
	}
	p, err := m.db.LoadPlaylist(m.playlists[m.plCursor].ID)
	if err != nil {
		m.setStatus("Could not open the playlist: " + err.Error())
		return
	}
	m.plOpen = &p
	m.plCursor, m.plOffset = 0, 0
	m.setStatus(fmt.Sprintf("%s — %d songs", p.Name, len(p.Tracks)))
}

// closePlaylist is esc inside one: back to the list, on the playlist
// just left.
func (m *Model) closePlaylist() {
	id := m.plOpen.ID
	m.plOpen = nil
	m.reloadPlaylists()
	for i, p := range m.playlists {
		if p.ID == id {
			m.plCursor = i
		}
	}
	m.clampPlOffset()
}

// removeFromPlaylist is d on a song of the open playlist.
func (m *Model) removeFromPlaylist() {
	p := m.plOpen
	if m.db == nil || m.plCursor >= len(p.Tracks) {
		return
	}
	t := p.Tracks[m.plCursor]
	tracks := append(p.Tracks[:m.plCursor:m.plCursor], p.Tracks[m.plCursor+1:]...)
	if err := m.db.SetPlaylistTracks(p.ID, tracks); err != nil {
		m.setStatus("Could not remove it: " + err.Error())
		return
	}
	p.Tracks = tracks
	m.clampPlOffset()
	m.setStatus("Removed " + t.Title + " from " + p.Name)
}

// movePlaylistSong is ctrl+↑/↓ in an open playlist: the song moves one
// place, and the cursor with it.
func (m *Model) movePlaylistSong(step int) {
	p, i := m.plOpen, m.plCursor
	j := i + step
	if m.db == nil || i >= len(p.Tracks) || j < 0 || j >= len(p.Tracks) {
		return
	}
	tracks := append([]queue.Track{}, p.Tracks...)
	tracks[i], tracks[j] = tracks[j], tracks[i]
	if err := m.db.SetPlaylistTracks(p.ID, tracks); err != nil {
		m.setStatus("Could not move it: " + err.Error())
		return
	}
	p.Tracks = tracks
	m.movePlCursor(step)
}

// askDeletePlaylist is d on the list: it asks first, as every delete does.
func (m *Model) askDeletePlaylist() {
	if m.plCursor >= len(m.playlists) {
		return
	}
	p := m.playlists[m.plCursor]
	action := lipgloss.NewStyle().Foreground(colorText).Bold(true).Render("Delete playlist")
	title := lipgloss.NewStyle().Foreground(colorText).Render("\"" + p.Name + "\"?")
	m.startConfirm(confirmDeletePlaylist, p.Name)
	m.setStatus(confirmStatus(action, title))
}

func (m *Model) deleteSelectedPlaylist() {
	if m.db == nil || m.plCursor >= len(m.playlists) {
		return
	}
	p := m.playlists[m.plCursor]
	if err := m.db.DeletePlaylist(p.ID); err != nil {
		m.setStatus("Could not delete it: " + err.Error())
		return
	}
	m.reloadPlaylists()
	m.setStatus("Deleted playlist " + p.Name)
}

// newPlPicker makes the picker, its name field ready for when it is
// wanted.
func (m *Model) newPlPicker(tracks []queue.Track, what string) *plPicker {
	in := textinput.New()
	in.Prompt = "Name: "
	in.PromptStyle = textinputStyle
	in.TextStyle = textinputStyle
	in.CharLimit = 100
	m.searchFocused = false
	m.searchInput.Blur()
	m.reloadPlaylists()
	return &plPicker{tracks: tracks, what: what, name: in}
}

// addToPlaylistAction is m: the picker, for the song the cursor is on.
func (m *Model) addToPlaylistAction() tea.Cmd {
	t, ok := m.selectedTrack()
	if !ok {
		m.setStatus("Select a song to add to a playlist")
		return nil
	}
	m.plPick = m.newPlPicker([]queue.Track{t}, t.Title)
	return nil
}

// savePlaylistAction is M: the list on screen as a new playlist, named
// after the pasted playlist or the album it is, which the name field
// starts with.
func (m *Model) savePlaylistAction() tea.Cmd {
	if !m.browsingHere() || (m.albumMode && !m.streamShowsTracks()) {
		m.setStatus("M saves a list of songs as a playlist")
		return nil
	}
	list := m.results
	name := m.resultsTitle
	if m.streamShowsTracks() {
		list = m.streamTracks()
		name = ""
		if m.openAlbum != nil {
			name = m.openAlbum.Title
		}
	}
	if len(list) == 0 {
		m.setStatus("Nothing to save")
		return nil
	}
	tracks := make([]queue.Track, len(list))
	for i, r := range list {
		tracks[i] = m.resolveTrack(r)
	}
	m.plPick = m.newPlPicker(tracks, fmt.Sprintf("%d songs", len(tracks)))
	m.startNaming(name)
	return nil
}

func (m *Model) startNaming(name string) {
	m.plPick.naming = true
	m.plPick.name.SetValue(name)
	m.plPick.name.CursorEnd()
	m.plPick.name.Focus()
}

// handlePlPickKey owns the keyboard while the picker is open.
func (m Model) handlePlPickKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	pk := m.plPick
	if pk.naming {
		switch msg.String() {
		case "esc":
			m.plPick = nil
			m.setStatus("Cancelled")
		case "enter":
			name := strings.TrimSpace(pk.name.Value())
			if name == "" {
				return m, nil
			}
			m.plPick = nil
			if m.db == nil {
				return m, nil
			}
			if _, err := m.db.CreatePlaylist(name, pk.tracks); err != nil {
				m.setStatus("Could not make the playlist: " + err.Error())
				return m, nil
			}
			m.reloadPlaylists()
			m.setStatus(fmt.Sprintf("Made playlist %s with %d songs", name, len(pk.tracks)))
		default:
			var cmd tea.Cmd
			pk.name, cmd = pk.name.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	switch msg.String() {
	case "esc", "q", "m":
		m.plPick = nil
		m.setStatus("Cancelled")
	case "up", "k":
		pk.cursor = max(0, pk.cursor-1)
	case "down", "j":
		pk.cursor = min(len(m.playlists), pk.cursor+1)
	case "enter":
		if pk.cursor == 0 {
			m.startNaming("")
			return m, nil
		}
		m.plPick = nil
		m.addToPlaylist(m.playlists[pk.cursor-1], pk.tracks)
	}
	return m, nil
}

// addToPlaylist puts songs at the end of a playlist, leaving out those
// it has already.
func (m *Model) addToPlaylist(info db.PlaylistInfo, tracks []queue.Track) {
	if m.db == nil {
		return
	}
	p, err := m.db.LoadPlaylist(info.ID)
	if err != nil {
		m.setStatus("Could not add it: " + err.Error())
		return
	}
	all, added := db.AppendNew(p.Tracks, tracks)
	if added == 0 {
		m.setStatus("Already in " + p.Name)
		return
	}
	if err := m.db.SetPlaylistTracks(p.ID, all); err != nil {
		m.setStatus("Could not add it: " + err.Error())
		return
	}
	if m.plOpen != nil && m.plOpen.ID == p.ID {
		m.plOpen.Tracks = all
	}
	m.reloadPlaylists()
	if added == 1 {
		m.setStatus("Added " + tracks[0].Title + " to " + p.Name)
	} else {
		m.setStatus(fmt.Sprintf("Added %d songs to %s", added, p.Name))
	}
}

// plPickLabel is the left panel's title while the picker is open.
func (m Model) plPickLabel() string {
	esc := styleKeyHint.Render("[esc]")
	if m.plPick.naming {
		return "NEW PLAYLIST  " + m.plPick.what + m.hints("  "+styleKeyHint.Render("[enter]")+" save  "+esc+" cancel")
	}
	return "ADD TO PLAYLIST  " + m.plPick.what + m.hints("  "+styleKeyHint.Render("[enter]")+" add  "+esc+" cancel")
}

// renderPlPicker draws the picker where the list was.
func (m Model) renderPlPicker(width, height int) string {
	rowW := max(1, width-2)
	pk := m.plPick
	var lines []string
	if pk.naming {
		pk.name.Width = max(1, rowW-lipgloss.Width(pk.name.Prompt)-1)
		lines = append(lines, pk.name.View())
	} else {
		rows := []string{"+ New playlist…"}
		for _, p := range m.playlists {
			rows = append(rows, fmt.Sprintf("%s   %d songs", p.Name, p.Count))
		}
		// The cursor stays in sight on a long list.
		start := max(0, pk.cursor-height+1)
		for i := start; i < len(rows) && i < start+height; i++ {
			line := "  " + truncate(rows[i], rowW-2)
			if i == pk.cursor {
				line = styleNowTitle.Render("▶ " + truncate(rows[i], rowW-2))
			}
			lines = append(lines, line)
		}
	}
	result := padToWidth(strings.Join(lines, "\n"), rowW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(make([]string, height-cnt), "\n"+strings.Repeat(" ", rowW))
	}
	return result
}

// playlistsLabel is the Playlists page's panel title.
func (m Model) playlistsLabel() string {
	esc := styleKeyHint.Render("[esc]")
	d := styleKeyHint.Render("[d]")
	e := styleKeyHint.Render("[e]")
	X := styleKeyHint.Render("[X]")
	if m.plOpen != nil {
		return strings.ToUpper(m.plOpen.Name) + m.hints("  "+e+" queue all  "+X+" download all  "+d+" remove  "+
			styleKeyHint.Render("[ctrl+↑↓]")+" reorder  "+esc+" back")
	}
	return fmt.Sprintf("PLAYLISTS %d", len(m.playlists)) +
		m.hints("  "+styleKeyHint.Render("[enter]")+" open  "+d+" delete")
}

// renderPlaylists lists the playlists, or the open one's songs, two
// lines a row like every other list.
func (m Model) renderPlaylists(width, height int) string {
	rowW := max(1, width-2)
	n := m.plLen()
	if n == 0 {
		hint := "No playlists yet — press m on any song, or M on a list"
		if m.plOpen != nil {
			hint = "Empty — press m on any song to add it here"
		}
		return styleEmpty.Width(width - 2).Height(height).Render(hint)
	}
	var lines []string
	end := min(m.plOffset+max(1, (height-1)/2), n)
	for i := m.plOffset; i < end; i++ {
		var line, left, right string
		if m.plOpen != nil {
			t := m.plOpen.Tracks[i]
			line, left, right = t.Title, t.Artist, t.Duration
		} else {
			p := m.playlists[i]
			line, left, right = p.Name, fmt.Sprintf("%d songs", p.Count), formatDuration(p.Seconds)
		}
		line = truncate(fmt.Sprintf("%d. %s", i+1, line), rowW)
		left = truncate("   "+left, max(1, rowW-lipgloss.Width(right)-2))
		info := left + strings.Repeat(" ", max(1, rowW-lipgloss.Width(left)-lipgloss.Width(right))) + right
		isSelected := !m.searchFocused && m.activePanel == PanelSearch && i == m.plCursor
		lines = append(lines, renderListItemBlock(line, truncate(info, rowW), isSelected, false, rowW))
	}
	if ind := scrollIndicator(m.plOffset, n-end, m.plCursor+1, n); ind != "" {
		lines = append(lines, ind)
	}
	result := padToWidth(strings.Join(lines, "\n"), rowW)
	if cnt := strings.Count(result, "\n") + 1; cnt < height {
		result += "\n" + strings.Join(make([]string, height-cnt), "\n"+strings.Repeat(" ", rowW))
	}
	return result
}
