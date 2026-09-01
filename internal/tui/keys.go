package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines every key binding in the TUI.
type KeyMap struct {
	Quit          key.Binding
	Help          key.Binding
	FocusNext     key.Binding
	Enter         key.Binding
	Up            key.Binding
	Down          key.Binding
	JumpTop       key.Binding
	JumpBottom    key.Binding
	PlayPause     key.Binding
	NextTrack     key.Binding
	PrevTrack     key.Binding
	SeekForward   key.Binding
	SeekBackward  key.Binding
	VolumeUp      key.Binding
	VolumeDown    key.Binding
	Delete        key.Binding
	Favorite      key.Binding
	Shuffle       key.Binding
	Repeat        key.Binding
	ClearQueue    key.Binding
	MoveUp        key.Binding
	MoveDown      key.Binding
	Download      key.Binding
	Recs          key.Binding
	Albums        key.Binding
	QueueAlbum    key.Binding
	AlbumInfo     key.Binding
	Visualizer    key.Binding
	Lyrics        key.Binding
	Downloads     key.Binding
	Open          key.Binding
	Update        key.Binding
	ClearHistory  key.Binding
	PageStream    key.Binding // 1
	PageDownloads key.Binding // 6
	PageFavorites key.Binding // 2
	PageLibrary   key.Binding // 3
	PageHistory   key.Binding // 4
	PageSettings  key.Binding // 5
	Escape        key.Binding
}

// Keys is the canonical keymap singleton.
var Keys = KeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
	FocusNext: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "cycle focus"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "play / add to queue"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	JumpTop: key.NewBinding(
		key.WithKeys("g"),
		key.WithHelp("g", "jump to top"),
	),
	JumpBottom: key.NewBinding(
		key.WithKeys("G"),
		key.WithHelp("G", "jump to bottom"),
	),
	PlayPause: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "play / pause"),
	),
	NextTrack: key.NewBinding(
		key.WithKeys("n", "right"),
		key.WithHelp("n/→", "next track"),
	),
	PrevTrack: key.NewBinding(
		key.WithKeys("p", "left"),
		key.WithHelp("p/←", "prev track"),
	),
	SeekForward: key.NewBinding(
		key.WithKeys("l", "ctrl+f"),
		key.WithHelp("l", "seek +5s"),
	),
	SeekBackward: key.NewBinding(
		key.WithKeys("h", "ctrl+b"),
		key.WithHelp("h", "seek -5s"),
	),
	VolumeUp: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+", "volume up"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("-", "_"),
		key.WithHelp("-", "volume down"),
	),
	Delete: key.NewBinding(
		key.WithKeys("d", "delete"),
		key.WithHelp("d", "remove from queue"),
	),
	Favorite: key.NewBinding(
		key.WithKeys("f"),
		key.WithHelp("f", "toggle favorite"),
	),
	Shuffle: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "shuffle"),
	),
	Repeat: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "repeat"),
	),
	ClearQueue: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "clear queue"),
	),
	MoveUp: key.NewBinding(
		key.WithKeys("ctrl+up"),
		key.WithHelp("ctrl+↑", "move item up"),
	),
	MoveDown: key.NewBinding(
		key.WithKeys("ctrl+down"),
		key.WithHelp("ctrl+↓", "move item down"),
	),
	Download: key.NewBinding(
		key.WithKeys("x"),
		key.WithHelp("x", "download track"),
	),
	Recs: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "recommendations"),
	),
	Albums: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "songs / albums"),
	),
	QueueAlbum: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "queue album"),
	),
	AlbumInfo: key.NewBinding(
		key.WithKeys("i"),
		key.WithHelp("i", "open album of track"),
	),
	Visualizer: key.NewBinding(
		key.WithKeys("v"),
		key.WithHelp("v", "visualizer"),
	),
	Lyrics: key.NewBinding(
		key.WithKeys("y"),
		key.WithHelp("y", "lyrics"),
	),
	Downloads: key.NewBinding(
		key.WithKeys("X"),
		key.WithHelp("X", "downloads"),
	),
	Open: key.NewBinding(
		key.WithKeys("o"),
		key.WithHelp("o", "open download dir"),
	),
	Update: key.NewBinding(
		key.WithKeys("U"),
		key.WithHelp("U", "update ytmgo"),
	),
	ClearHistory: key.NewBinding(
		key.WithKeys("C"),
		key.WithHelp("C", "clear history"),
	),
	PageStream: key.NewBinding(
		key.WithKeys("1"),
		key.WithHelp("1", "stream page"),
	),
	PageFavorites: key.NewBinding(
		key.WithKeys("2"),
		key.WithHelp("2", "favorites"),
	),
	PageLibrary: key.NewBinding(
		key.WithKeys("3"),
		key.WithHelp("3/L", "library page"),
	),
	PageHistory: key.NewBinding(
		key.WithKeys("4"),
		key.WithHelp("4", "history"),
	),
	PageSettings: key.NewBinding(
		key.WithKeys("6"),
		key.WithHelp("6", "settings page"),
	),
	PageDownloads: key.NewBinding(
		key.WithKeys("5"),
		key.WithHelp("5", "downloads page"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back / close"),
	),
}

// ShortHelp returns key bindings for the compact help line: the panel
// toggles plus the general-purpose utilities. Playback and navigation
// hints stay inline next to their contextual UI element (player bar
// controls, panel titles, header), but the two pane toggles have no
// permanent home of their own — the pane each one reopens is exactly
// the thing that is not on screen — so they live here. Downloads left
// this list when it became a page: its way in is the [5] tab, which is
// always visible in the header.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Visualizer,
		k.Lyrics,
		k.Help,
		k.Quit,
	}
}

// FullHelp returns all key bindings for the expanded help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			k.FocusNext,
			k.Enter,
			k.Up,
			k.Down,
			k.JumpTop,
			k.JumpBottom,
		},
		{
			k.PlayPause,
			k.NextTrack,
			k.PrevTrack,
			k.SeekForward,
			k.SeekBackward,
		},
		{
			k.VolumeUp,
			k.VolumeDown,
			k.Shuffle,
			k.Repeat,
		},
		{
			k.Delete,
			k.ClearQueue,
			k.ClearHistory,
			k.MoveUp,
			k.MoveDown,
			k.Download,
			k.Open,
			k.Update,
			k.Favorite,
		},
		{
			k.PageStream,
			k.PageFavorites,
			k.PageLibrary,
			k.PageHistory,
			k.PageSettings,
			k.PageDownloads,
			k.Recs,
			k.Albums,
			k.QueueAlbum,
			k.AlbumInfo,
			k.Visualizer,
			k.Lyrics,
			k.Downloads,
		},
		{
			k.Help,
			k.Escape,
			k.Quit,
		},
	}
}

// Globals returns bindings that work regardless of focus mode. These
// are checked first by Update, so focus modes (search input, settings
// text field, etc.) cannot accidentally swallow them.
func (k KeyMap) Globals() []key.Binding {
	return []key.Binding{
		k.PageStream,
		k.PageFavorites,
		k.PageLibrary,
		k.PageHistory,
		k.PageSettings,
		k.PageDownloads,
		k.Recs,
		k.Open,
	}
}
