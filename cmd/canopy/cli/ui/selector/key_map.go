package selector

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMap defines the keyboard shortcuts for the selector UI.
// it provides bindings for test selection, navigation, and view toggling.
type keyMap struct {
	// SelectTest toggles selection for the current test reference.
	SelectTest key.Binding
	// SelectAllTests selects or deselects all visible tests.
	SelectAllTests key.Binding
	// Finish confirms the current selection and starts running tests.
	Finish key.Binding
	// NextPackage jumps to the first test in the next package.
	NextPackage key.Binding
	// PrevPackage jumps to the first test in the previous package.
	PrevPackage key.Binding
	// ToggleReferenceLongForm switches between short and long reference display formats.
	ToggleReferenceLongForm key.Binding
	// ToggleTests shows or hides test function references (package-only view).
	ToggleTests key.Binding
}

// newKeyMap creates a new keyMap with default key bindings.
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

// AdditionalShortHelp returns the key bindings to show in the short help view.
func (k keyMap) AdditionalShortHelp() []key.Binding {
	return []key.Binding{k.SelectTest, k.SelectAllTests, k.Finish}
}

// AdditionalFullHelp returns the key bindings to show in the full help view.
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
