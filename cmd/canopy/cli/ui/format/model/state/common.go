package state

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/style"
)

// Common holds shared state that is passed to all UI models, including
// terminal window dimensions and spinner state for consistent animations.
type Common struct {
	// Window holds the current terminal window size.
	Window tea.WindowSizeMsg

	// Spinner is the synchronized spinner state shared across all models.
	Spinner syncspinner.TickMsg

	// Canceled indicates the user interrupted the run (esc/ctrl+c).
	Canceled bool
}

// OnMessage updates common state from Bubble Tea messages, handling window
// resize, spinner tick, and interrupt events.
func (c *Common) OnMessage(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		c.Window = msg
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			// swap the spinner for a stopped glyph, otherwise every in-flight row is left with a frozen spinner
			c.Canceled = true
			c.Spinner.View = style.CanceledGlyph
		}
	case syncspinner.TickMsg:
		if msg.ID == c.Spinner.ID && !c.Canceled {
			c.Spinner = msg
		}
	}
}
