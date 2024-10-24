package state

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/model/bubble/syncspinner"
)

type Common struct {
	Window  tea.WindowSizeMsg
	Spinner syncspinner.TickMsg
}

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
