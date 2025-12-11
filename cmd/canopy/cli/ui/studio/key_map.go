package studio

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

// keyMap defines the global keybindings for the studio UI.
type keyMap struct {
	// Help toggles the help display between short and full modes.
	Help xhelp.Item

	// ReRunAllTests re-runs all tests in the current test run.
	ReRunAllTests xhelp.Item

	// ReRunTestSelection re-runs only the currently selected tests.
	ReRunTestSelection xhelp.Item

	// Quit exits the studio UI.
	Quit xhelp.Item
}

// newKeyMap creates a new keyMap with default keybindings.
func newKeyMap() *keyMap {
	return &keyMap{
		Help: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("?"),
				key.WithHelp("?", "toggle help"),
			),
		),
		ReRunAllTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "re-run all"),
			),
		),
		ReRunTestSelection: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "re-run"),
			),
		),
		Quit: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("q", "ctrl+c"),
				key.WithHelp("q", "quit"),
			),
		),
	}
}

// ShortHelp returns keybindings to display in the short help view.
func (k keyMap) ShortHelp() []xhelp.Item {
	return []xhelp.Item{
		k.Help, k.Quit,
		k.ReRunAllTests, k.ReRunTestSelection,
	}
}

// FullHelp returns keybindings organized into columns for the full help view.
func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		{
			k.Help, k.Quit,
		},
		{
			k.ReRunAllTests, k.ReRunTestSelection,
		},
	}
}
