package selector

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type keyMap struct {
	SelectTest     key.Binding
	SelectAllTests key.Binding
	NextPackage    key.Binding
	PrevPackage    key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		SelectTest: key.NewBinding(
			key.WithKeys(" "), // space
			key.WithHelp("space", "select"),
		),
		SelectAllTests: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("^a", "all"),
		),
		NextPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftDown.String()), // space
			key.WithHelp(tea.KeyCtrlShiftDown.String(), "next package"),
		),
		PrevPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftUp.String()), // space
			key.WithHelp(tea.KeyCtrlShiftUp.String(), "prev package"),
		),
	}
}

func (k keyMap) AdditionalShortHelp() []key.Binding {
	return []key.Binding{k.SelectTest, k.SelectAllTests}
}

func (k keyMap) AdditionalFullHelp() []key.Binding {
	return []key.Binding{
		k.NextPackage,
		k.PrevPackage,
	}
}
