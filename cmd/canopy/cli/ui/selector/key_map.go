package selector

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/xhelp"
)

type keyMap struct {
	Help xhelp.Item
	//ReRunAllTests      xhelp.Item
	//ReRunTestSelection xhelp.Item
	Quit xhelp.Item
}

func newKeyMap() *keyMap {
	return &keyMap{
		Help: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("?"),
				key.WithHelp("?", "toggle help"),
			),
		),
		//ReRunAllTests: xhelp.NewKeyBinding(
		//	key.NewBinding(
		//		key.WithKeys("a"),
		//		key.WithHelp("a", "re-run all"),
		//	),
		//),
		//ReRunTestSelection: xhelp.NewKeyBinding(
		//	key.NewBinding(
		//		key.WithKeys("r"),
		//		key.WithHelp("r", "re-run"),
		//	),
		//),
		Quit: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("esc", "ctrl+c"),
				key.WithHelp("esc", "quit"),
			),
		),
	}
}

func (k keyMap) ShortHelp() []xhelp.Item {
	return []xhelp.Item{
		k.Help, k.Quit,
		//k.ReRunAllTests, k.ReRunTestSelection,
	}
}

func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		{
			k.Help, k.Quit,
		},
		//{
		//	k.ReRunAllTests, k.ReRunTestSelection,
		//},
	}
}
