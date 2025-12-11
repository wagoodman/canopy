package outputpane

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

// keyMap defines keybindings for the output pane. Currently empty as the output
// pane uses default viewport navigation bindings.
type keyMap struct {
	// ReRunAllTests      xhelp.Item
	// ReRunTestSelection xhelp.Item
	// defaultKeyMap
}

// newKeyMap creates an output pane keyMap with no custom bindings.
func newKeyMap() keyMap {
	return keyMap{
		//ReRunAllTests: xhelp.NewKeyBinding(
		//	key.NewBinding(
		//		key.WithKeys("a"),
		//		key.WithHelp("a", "re-run all"),
		//	),
		// ),
		//ReRunTestSelection: xhelp.NewKeyBinding(
		//	key.NewBinding(
		//		key.WithKeys("r"),
		//		key.WithHelp("r", "re-run"),
		//	),
		// ),
		//defaultKeyMap: defaultKeys,
	}
}

// ShortHelp returns no custom keybindings for the output pane.
func (k keyMap) ShortHelp() []xhelp.Item {
	return nil
	// return []xhelp.Item{k.ReRunAllTests, k.ReRunTestSelection}
	// k.Quit,
}

// FullHelp returns no custom keybindings for the output pane.
func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		// {k.ReRunAllTests, k.ReRunTestSelection}, // first column
		// {k.Quit},                                // second column
	}
}
