package selector

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type keyMap struct {
	SelectTest              key.Binding
	SelectAllTests          key.Binding
	Finish                  key.Binding
	NextPackage             key.Binding
	PrevPackage             key.Binding
	ToggleReferenceLongForm key.Binding
	ToggleTests             key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		SelectTest: key.NewBinding(
			key.WithKeys(" "), // space
			key.WithHelp("space", "select"),
		),
		SelectAllTests: key.NewBinding(
			key.WithKeys("ctrl+a"),
			key.WithHelp("ctrl+a", "all"),
		),
		Finish: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "run tests"),
		),
		NextPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftDown.String()), // space
			key.WithHelp(tea.KeyCtrlShiftDown.String(), "next package"),
		),
		PrevPackage: key.NewBinding(
			key.WithKeys(tea.KeyCtrlShiftUp.String()), // space
			key.WithHelp(tea.KeyCtrlShiftUp.String(), "prev package"),
		),
		ToggleReferenceLongForm: key.NewBinding(
			key.WithKeys("ctrl+l"),
			key.WithHelp("ctrl+l", "long form"),
		),
		ToggleTests: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "toggle tests"),
		),
	}
}

func (k keyMap) AdditionalShortHelp() []key.Binding {
	return []key.Binding{k.SelectTest, k.SelectAllTests, k.Finish}
}

func (k keyMap) AdditionalFullHelp() []key.Binding {
	return []key.Binding{
		k.NextPackage,
		k.PrevPackage,
		k.ToggleReferenceLongForm,
		k.ToggleTests,
		k.SelectAllTests,
		k.SelectTest,
		k.Finish,
	}
}
