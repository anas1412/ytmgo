package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines every key binding in the TUI.
type KeyMap struct {
	Quit         key.Binding
	Help         key.Binding
	FocusNext    key.Binding
	Enter        key.Binding
	Up           key.Binding
	Down         key.Binding
	PlayPause    key.Binding
	NextTrack    key.Binding
	PrevTrack    key.Binding
	SeekForward  key.Binding
	SeekBackward key.Binding
	VolumeUp     key.Binding
	VolumeDown   key.Binding
	Delete       key.Binding
	Shuffle      key.Binding
	Repeat       key.Binding
	ClearQueue   key.Binding
	MoveUp       key.Binding
	MoveDown     key.Binding
	Download      key.Binding
	Library       key.Binding
	Recs          key.Binding
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
	Library: key.NewBinding(
		key.WithKeys("L"),
		key.WithHelp("L", "library toggle"),
	),
	Recs: key.NewBinding(
		key.WithKeys("R"),
		key.WithHelp("R", "recommendations"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back / close"),
	),
}

// ShortHelp returns key bindings for the compact help line.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.FocusNext,
		k.Enter,
		k.PlayPause,
		k.Library,
		k.Recs,
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
			k.MoveUp,
			k.MoveDown,
			k.Download,
		},
		{
			k.Help,
			k.Library,
			k.Recs,
			k.Escape,
			k.Quit,
		},
	}
}
