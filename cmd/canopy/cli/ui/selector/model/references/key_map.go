package references

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/xhelp"
)

//type listKeyMap struct {
//	toggleSpinner    key.Binding
//	toggleTitleBar   key.Binding
//	toggleStatusBar  key.Binding
//	togglePagination key.Binding
//	toggleHelpMenu   key.Binding
//	insertItem       key.Binding
//}
//
//func newListKeyMap() *listKeyMap {
//	return &listKeyMap{
//		//insertItem: key.NewBinding(
//		//	key.WithKeys("a"),
//		//	key.WithHelp("a", "add item"),
//		//),
//		toggleSpinner: key.NewBinding(
//			key.WithKeys("s"),
//			key.WithHelp("s", "toggle spinner"),
//		),
//		toggleTitleBar: key.NewBinding(
//			key.WithKeys("T"),
//			key.WithHelp("T", "toggle title"),
//		),
//		toggleStatusBar: key.NewBinding(
//			key.WithKeys("S"),
//			key.WithHelp("S", "toggle status"),
//		),
//		togglePagination: key.NewBinding(
//			key.WithKeys("P"),
//			key.WithHelp("P", "toggle pagination"),
//		),
//		toggleHelpMenu: key.NewBinding(
//			key.WithKeys("H"),
//			key.WithHelp("H", "toggle help"),
//		),
//	}
//}

type keyMap struct {
	SelectTest       xhelp.Item
	SelectAllTests   xhelp.Item
	ShowFailedTests  xhelp.Item
	ShowPassedTests  xhelp.Item
	ShowSkippedTests xhelp.Item
	NextPackage      xhelp.Item
	PrevPackage      xhelp.Item
	NextTestFunc     xhelp.Item
	PrevTestFunc     xhelp.Item
}

func newKeyMap(showFailedOnly bool) keyMap {
	return keyMap{
		SelectTest: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys(" "), // space
				key.WithHelp("space", "select test"),
			),
		),
		SelectAllTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("ctrl+a"),
				key.WithHelp("ctrl+a", "select all"),
			),
		),
		ShowFailedTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("ctrl+f"),
			),
		).WithToggle(true, "hide failed", "show failed"),
		ShowPassedTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("ctrl+p"),
			),
		).WithToggle(!showFailedOnly, "hide passed", "show passed"),
		ShowSkippedTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("ctrl+s"),
			),
		).WithToggle(false, "hide skipped", "show skipped"),
		NextPackage: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys(tea.KeyCtrlShiftDown.String()), // space
				key.WithHelp(tea.KeyCtrlShiftDown.String(), "next package"),
			),
		),
		PrevPackage: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys(tea.KeyCtrlShiftUp.String()), // space
				key.WithHelp(tea.KeyCtrlShiftUp.String(), "prev package"),
			),
		),
		NextTestFunc: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys(tea.KeyShiftDown.String()), // space
				key.WithHelp(tea.KeyShiftDown.String(), "next test function"),
			),
		),
		PrevTestFunc: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys(tea.KeyShiftUp.String()), // space
				key.WithHelp(tea.KeyShiftUp.String(), "prev test function"),
			),
		),
	}
}

func (k keyMap) ShortHelp() []xhelp.Item {
	return []xhelp.Item{k.SelectTest, k.SelectAllTests, k.ShowFailedTests, k.ShowPassedTests, k.ShowSkippedTests}
}

func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		{k.SelectTest, k.SelectAllTests},
		{k.ShowFailedTests, k.ShowPassedTests, k.ShowSkippedTests},
		{k.PrevPackage, k.NextPackage, k.PrevTestFunc, k.NextTestFunc},
	}
}
