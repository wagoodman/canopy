package outputpane

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

type keyMap struct {
	// ReRunAllTests      xhelp.Item
	// ReRunTestSelection xhelp.Item
	// defaultKeyMap
}

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

func (k keyMap) ShortHelp() []xhelp.Item {
	return nil
	// return []xhelp.Item{k.ReRunAllTests, k.ReRunTestSelection}
	// k.Quit,
}

func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		// {k.ReRunAllTests, k.ReRunTestSelection}, // first column
		// {k.Quit},                                // second column
	}
}
