package referencespane

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
)

// keyMap defines keybindings for the references pane, including test selection,
// filtering, and navigation commands.
type keyMap struct {
	// SelectTest toggles selection of the current test reference.
	SelectTest xhelp.Item

	// SelectAllTests selects all visible test references.
	SelectAllTests xhelp.Item

	// ShowFailedTests toggles visibility of failed tests.
	ShowFailedTests xhelp.Item

	// ShowPassedTests toggles visibility of passed tests.
	ShowPassedTests xhelp.Item

	// ShowSkippedTests toggles visibility of skipped tests.
	ShowSkippedTests xhelp.Item

	// NextPackage navigates to the next package in the list.
	NextPackage xhelp.Item

	// PrevPackage navigates to the previous package in the list.
	PrevPackage xhelp.Item

	// NextTestFunc navigates to the next test function in the list.
	NextTestFunc xhelp.Item

	// PrevTestFunc navigates to the previous test function in the list.
	PrevTestFunc xhelp.Item
}

// newKeyMap creates a keyMap with default keybindings. The showFailedOnly parameter
// determines the initial state of the ShowPassedTests toggle.
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
				key.WithKeys("f"),
			),
		).WithToggle(true, "hide failed", "show failed"),
		ShowPassedTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("p"),
			),
		).WithToggle(!showFailedOnly, "hide passed", "show passed"),
		ShowSkippedTests: xhelp.NewKeyBinding(
			key.NewBinding(
				key.WithKeys("s"),
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

// ShortHelp returns keybindings for the short help view.
func (k keyMap) ShortHelp() []xhelp.Item {
	return []xhelp.Item{k.SelectTest, k.SelectAllTests, k.ShowFailedTests, k.ShowPassedTests, k.ShowSkippedTests}
}

// FullHelp returns keybindings organized into columns for the full help view.
func (k keyMap) FullHelp() [][]xhelp.Item {
	return [][]xhelp.Item{
		{k.SelectTest, k.SelectAllTests},
		{k.ShowFailedTests, k.ShowPassedTests, k.ShowSkippedTests},
		{k.PrevPackage, k.NextPackage, k.PrevTestFunc, k.NextTestFunc},
	}
}
