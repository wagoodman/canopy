package state

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
)

// Common holds shared state that is passed to all UI models, including
// terminal window dimensions and spinner state for consistent animations.
type Common struct {
	// Window holds the current terminal window size.
	Window tea.WindowSizeMsg

	// Spinner is the synchronized spinner state shared across all models.
	Spinner syncspinner.TickMsg
}

// OnMessage updates common state from Bubble Tea messages, handling window
// resize and spinner tick events.
func (c *Common) OnMessage(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.Window = msg
	case syncspinner.TickMsg:
		if msg.ID == c.Spinner.ID {
			c.Spinner = msg
		}
	}
}
