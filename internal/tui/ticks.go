package tui

import (
	"time"

	"ytmgo/internal/player"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Periodic tick (progress bar animation) ──────────────────────────

func (m Model) handleTick(msg tickMsg) (tea.Model, tea.Cmd) {
	// Dev mode (no player): fake position advance
	if m.player == nil && m.playerState == player.StatePlaying && m.duration > 0 {
		m.position += 0.5
		// Keep the interpolation anchor fresh in dev mode so the
		// smooth bar reads the dev-simulated position accurately.
		m.lastPosition = m.position
		m.lastPositionAt = time.Now()
		if m.position >= m.duration {
			m.position = 0
			if t, ok := m.queue.Next(); ok {
				m.queueCursor = m.queue.CurrentIndex()
				m.duration = float64(t.DurationSec)
				m.setStatus("Now playing: " + t.Title)
			} else {
				m.playerState = player.StateStopped
			}
		}
	}
	// Auto-clear status messages after 3 seconds so the idle quote
	// cycles back into view. Never auto-clear during confirmation
	// — the prompt must stay visible until Enter or Esc.
	if m.statusMessage != "" && m.err == nil && !m.isConfirming() && !m.statusMessageSetAt.IsZero() && time.Since(m.statusMessageSetAt) >= 3*time.Second {
		m.clearStatus()
	}
	// Advance the idle quote every quoteRotateEvery ticks.
	// Only count when no live status message or error is being shown.
	var cmds []tea.Cmd
	if m.statusMessage == "" && m.err == nil {
		m.tickCount++
		if m.tickCount >= quoteRotateEvery {
			m.tickCount = 0
			// Advance to next fallback quote (always works, instant)
			m.fallbackIdx++
			if m.fallbackIdx >= len(fallbackQuotes) {
				m.fallbackIdx = 0
			}
			m.currentQuote = fallbackQuotes[m.fallbackIdx]
			// Fire API fetch only when quotes setting is on
			if m.settings.ShowQuotes {
				m.quoteSeq++
				cmds = append(cmds, fetchQuoteCmd(m.quoteSeq))
			}
		}
	}
	// Real position comes from PositionMsg when player is active
	cmds = append(cmds, tickCmd())
	return m, tea.Batch(cmds...)
}

// ── Fast player tick (smooth progress interpolation) ─────────────────

func (m Model) handlePlayerTick(msg playerTickMsg) (tea.Model, tea.Cmd) {
	// Fires every 50ms while a track is playing. The model itself
	// doesn't need to change — View reads time.Now() against
	// lastPositionAt and renders a gliding bar. We just keep the
	// ticker alive as long as we're in the playing state, and let
	// it die off naturally when paused/stopped.
	if m.playerState == player.StatePlaying {
		return m, playerTickCmd()
	}
	return m, nil
}
