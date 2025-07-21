package references

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model/toggle"
)

type KeyMap struct {
	SelectTest       key.Binding
	SelectAllTests   key.Binding
	ShowFailedTests  toggle.Toggle
	ShowPassedTests  toggle.Toggle
	ShowSkippedTests toggle.Toggle
	NextPackage      key.Binding
	PrevPackage      key.Binding
	NextTestFunc     key.Binding
	PrevTestFunc     key.Binding
}

func NewKeyMap() KeyMap {
	return KeyMap{
		SelectTest: key.NewBinding(
			key.WithKeys(" "), // space
			key.WithHelp("space", "select"),
		),
		SelectAllTests: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^a", "all"),
		),
		ShowFailedTests: toggle.New(
			key.NewBinding(
				key.WithKeys("ctrl+f"),
				key.WithHelp("^f", "failed"),
			),
			//toggle.WithEngagedDescription("hide failed"),
			//toggle.WithDisengagedDescription("show failed"),
			toggle.WithEngaged(true),
		),
		ShowPassedTests: toggle.New(
			key.NewBinding(
				key.WithKeys("ctrl+p"),
				key.WithHelp("^p", "passed"),
			),
			//toggle.WithEngagedDescription("hide passed"),
			//toggle.WithDisengagedDescription("show passed"),
			toggle.WithEngaged(true),
		),
		ShowSkippedTests: toggle.New(
			key.NewBinding(
				key.WithKeys("ctrl+s"),
				key.WithHelp("^s", "skipped"),
			),
			//toggle.WithEngagedDescription("hide skipped"),
			//toggle.WithDisengagedDescription("show skipped"),
			toggle.WithEngaged(true),
		),
		NextPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftDown.String()), // space
			key.WithHelp(tea.KeyCtrlShiftDown.String(), "next package"),
		),
		PrevPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftUp.String()), // space
			key.WithHelp(tea.KeyCtrlShiftUp.String(), "prev package"),
		),
		NextTestFunc: key.NewBinding(
			key.WithKeys(tea.KeyShiftDown.String()), // space
			key.WithHelp(tea.KeyShiftDown.String(), "next test function"),
		),
		PrevTestFunc: key.NewBinding(
			key.WithKeys(tea.KeyShiftUp.String()), // space
			key.WithHelp(tea.KeyShiftUp.String(), "prev test function"),
		),
		//ReRunAllTests:
		//		key.WithKeys("a"),
		//		key.WithHelp("a", "re-run all"),
		//),
		//ReRunTestSelection: key.NewBinding(
		//		key.WithKeys("r"),
		//		key.WithHelp("r", "re-run"),
		//),
	}
}

func (k KeyMap) Toggles() toggle.Toggles {
	return toggle.Toggles{
		k.ShowFailedTests,
		k.ShowPassedTests,
		k.ShowSkippedTests,
	}
}

func (k KeyMap) AdditionalShortHelp() []key.Binding {
	return []key.Binding{k.SelectTest, k.SelectAllTests, k.ShowFailedTests.Binding, k.ShowPassedTests.Binding, k.ShowSkippedTests.Binding}
}

//func (k KeyMap) FullHelp() [][]key.Binding {
//	return [][]key.Binding{
//		{k.SelectTest, k.SelectAllTests},
//		{k.ShowFailedTests.Binding, k.ShowPassedTests.Binding, k.ShowSkippedTests.Binding},
//		{k.PrevPackage, k.NextPackage, k.PrevTestFunc, k.NextTestFunc},
//	}
//}

func (k KeyMap) AdditionalFullHelp() []key.Binding {
	return []key.Binding{
		k.NextPackage,
		k.PrevPackage,
		k.NextTestFunc,
		k.PrevTestFunc,
	}
}
