package tui

import (
	"fmt"

	"ytmgo/internal/settings"

	tea "github.com/charmbracelet/bubbletea"
)

// settingKind describes how a settings row is interacted with.
type settingKind int

const (
	settingToggle settingKind = iota // Enter toggles on/off
	settingCycle                     // Enter cycles through options
	settingNumber                    // +/- adjusts
	settingString                    // Enter starts inline editing
)

// settingDef declares one row of the Settings page. settingDefs below
// is the single source of truth for row order: rendering, keyboard, and
// mouse all derive from it, so rows can be added or removed without
// hunting down index-based switch statements. (Duplicated switches are
// what made +/- edit the wrong row before.)
type settingDef struct {
	label    string
	kind     settingKind
	value    func(m *Model) string
	desc     func(m *Model) string
	activate func(m *Model) tea.Cmd          // Enter for toggle/cycle rows
	adjust   func(m *Model, dir int) tea.Cmd // +/- for number rows (dir = +1/-1)
	editGet  func(m *Model) string           // current value for string rows
	editSet  func(m *Model, v string) tea.Cmd
	openBtn  bool // render the inline [Open] hint next to the value
}

func staticDesc(s string) func(*Model) string {
	return func(*Model) string { return s }
}

var settingDefs = []settingDef{
	{
		label: "Playback Mode",
		kind:  settingCycle,
		value: func(m *Model) string { return settings.PlaybackModeLabel(m.settings.PlaybackMode) },
		desc:  staticDesc("Stream (online) · Hybrid (play+download) · Offline (download first)"),
		activate: func(m *Model) tea.Cmd {
			m.settings.PlaybackMode = (m.settings.PlaybackMode + 1) % 3
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Theme",
		kind:  settingCycle,
		value: func(m *Model) string { return string(ParseTheme(m.settings.Theme)) },
		desc:  func(m *Model) string { return ThemeDesc(ParseTheme(m.settings.Theme)) },
		activate: func(m *Model) tea.Cmd {
			cur := ParseTheme(m.settings.Theme)
			next := themeOrder[0]
			for i, t := range themeOrder {
				if t == cur {
					next = themeOrder[(i+1)%len(themeOrder)]
					break
				}
			}
			m.settings.Theme = string(next)
			ApplyTheme(next)
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Show Quotes",
		kind:  settingToggle,
		value: func(m *Model) string { return boolStr(m.settings.ShowQuotes) },
		desc:  staticDesc("Show quotes in status bar when idle"),
		activate: func(m *Model) tea.Cmd {
			m.settings.ShowQuotes = !m.settings.ShowQuotes
			m.tickCount = 0
			if m.settings.ShowQuotes {
				// Start from first fallback quote
				m.fallbackIdx = 0
				m.currentQuote = fallbackQuotes[0]
			} else {
				// Advance to next tip
				m.advanceTip()
			}
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Discord RPC",
		kind:  settingToggle,
		value: func(m *Model) string { return boolStr(m.settings.DiscordRPCEnabled) },
		desc:  staticDesc("Show currently playing track on your Discord profile"),
		activate: func(m *Model) tea.Cmd {
			m.settings.DiscordRPCEnabled = !m.settings.DiscordRPCEnabled
			m.reinitDiscordRPC()
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Autoplay",
		kind:  settingToggle,
		value: func(m *Model) string { return boolStr(m.settings.AutoplayEnabled) },
		desc:  staticDesc("Auto-queue related tracks when queue runs out"),
		activate: func(m *Model) tea.Cmd {
			m.settings.AutoplayEnabled = !m.settings.AutoplayEnabled
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Default Volume",
		kind:  settingNumber,
		value: func(m *Model) string { return fmt.Sprintf("%d", m.settings.DefaultVolume) },
		desc:  staticDesc("Starting volume 0-100  (+/- adjust)"),
		adjust: func(m *Model, dir int) tea.Cmd {
			m.settings.DefaultVolume = min(100, max(0, m.settings.DefaultVolume+dir*5))
			if m.player != nil {
				m.player.SetVolume(m.settings.DefaultVolume)
			}
			m.volume = m.settings.DefaultVolume
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label: "Search Limit",
		kind:  settingNumber,
		value: func(m *Model) string { return fmt.Sprintf("%d", m.settings.SearchLimit) },
		desc:  staticDesc("Max results per search  (+/- adjust)"),
		adjust: func(m *Model, dir int) tea.Cmd {
			m.settings.SearchLimit = min(100, max(5, m.settings.SearchLimit+dir*5))
			return saveSettingsCmd(m.db, m.settings)
		},
	},
	{
		label:   "Download Dir",
		kind:    settingString,
		value:   func(m *Model) string { return m.settings.DownloadDir },
		desc:    staticDesc("Where files are saved  (press 'o' to open)"),
		editGet: func(m *Model) string { return m.settings.DownloadDir },
		editSet: func(m *Model, v string) tea.Cmd {
			m.settings.DownloadDir = v
			return saveSettingsCmd(m.db, m.settings)
		},
		openBtn: true,
	},
	{
		label: "Download Format",
		kind:  settingCycle,
		value: func(m *Model) string { return settings.DownloadFormatLabel(m.settings.DownloadFormat) },
		desc:  func(m *Model) string { return settings.DownloadFormatHint(m.settings.DownloadFormat) },
		activate: func(m *Model) tea.Cmd {
			switch m.settings.DownloadFormat {
			case settings.FormatM4A:
				m.settings.DownloadFormat = settings.FormatMP3
			default:
				m.settings.DownloadFormat = settings.FormatM4A
			}
			if m.downloader != nil {
				m.downloader.SetFormat(m.settings.DownloadFormat)
			}
			return saveSettingsCmd(m.db, m.settings)
		},
	},
}

// activateSettingsItem handles Enter (and double-click) on the settings
// page, including committing an in-progress string edit.
func (m Model) activateSettingsItem() (Model, tea.Cmd) {
	if m.settingsCursor < 0 || m.settingsCursor >= len(settingDefs) {
		return m, nil
	}
	def := settingDefs[m.settingsCursor]

	if m.settingsEditField {
		newVal := m.settingsEditInput.Value()
		m.settingsEditField = false
		m.settingsEditInput.Blur()
		if def.editSet != nil {
			return m, def.editSet(&m, newVal)
		}
		return m, nil
	}

	switch def.kind {
	case settingString:
		m.startSettingsEdit()
		return m, nil
	default:
		if def.activate != nil {
			return m, def.activate(&m)
		}
	}
	return m, nil
}

// adjustSetting handles +/- on the settings page. Rows that aren't
// numeric ignore the key.
func (m *Model) adjustSetting(dir int) tea.Cmd {
	if m.settingsCursor < 0 || m.settingsCursor >= len(settingDefs) {
		return nil
	}
	def := settingDefs[m.settingsCursor]
	if def.adjust == nil {
		return nil
	}
	return def.adjust(m, dir)
}
