package selector

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

type listItemDelegate struct {
	list.DefaultDelegate

	navigateBindings []key.Binding

	highlightedStyle       lipgloss.Style
	highlightedFailedStyle lipgloss.Style
	highlightedRunStyle    lipgloss.Style
	highlightedSkipStyle   lipgloss.Style
	highlightedPassStyle   lipgloss.Style

	multiSelectStyle       lipgloss.Style
	multiSelectFailedStyle lipgloss.Style
	multiSelectRunStyle    lipgloss.Style
	multiSelectSkipStyle   lipgloss.Style
	multiSelectPassStyle   lipgloss.Style

	passStyle     lipgloss.Style
	failStyle     lipgloss.Style
	runStyle      lipgloss.Style
	skipStyle     lipgloss.Style
	allTestsStyle lipgloss.Style

	current     *gotest.Reference
	cursorScope map[gotest.Reference]struct{}
	multiSelect map[gotest.Reference]struct{}

	state state.DefinitionViewer
}

func newItemDelegate(navigateBindings ...key.Binding) *listItemDelegate {
	d := list.NewDefaultDelegate()
	d.ShowDescription = false
	d.SetHeight(1)
	d.SetSpacing(0)

	// d.Styles.SelectedTitle = lipgloss.NewStyle().
	//	Border(lipgloss.HiddenBorder(), false, false, false, true).
	//	BorderBackground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
	//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
	//	Padding(0, 0, 0, 1)

	cursorBrd := lipgloss.NormalBorder()
	cursorBrd.Left = "❯" // ❯❱ ●• » ▚ [ "\U0001FB6A", this is a powerline glyph, but it doesn't work in all terminals, so we use a normal character instead )

	highlightPadding := lipgloss.NewStyle().Padding(0, 0, 0, 1)
	notHighlightedPadding := lipgloss.NewStyle().Padding(0, 0, 0, 2)

	d.Styles.SelectedTitle = highlightPadding.
		Border(cursorBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		// BorderForeground(lipgloss.AdaptiveColor{Light: "#AD58B4", Dark: "#EEEEEE"}).
		// BorderBackground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})

	scopeBrd := lipgloss.NormalBorder()
	scopeBrd.Left = "░" // █ ░ ▎ ▚ │

	highlightStyle := highlightPadding.
		Border(scopeBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	// Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})

	multiSelectBrd := lipgloss.NormalBorder()
	multiSelectBrd.Left = "█" // ✔

	multiSelectStyle := highlightStyle.
		Border(multiSelectBrd, false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})

	failedStyle := notHighlightedPadding.Foreground(lipgloss.Color("9"))
	runStyle := notHighlightedPadding.Foreground(lipgloss.Color("11"))
	skipStyle := notHighlightedPadding.Faint(true)
	// passStyle := notHighlightedPadding.Foreground(lipgloss.Color("10"))
	passStyle := notHighlightedPadding

	return &listItemDelegate{
		navigateBindings: navigateBindings,
		DefaultDelegate:  d,
		multiSelect:      make(map[gotest.Reference]struct{}),
		cursorScope:      make(map[gotest.Reference]struct{}),
		//highlightedStyle: lipgloss.NewStyle().
		//	Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
		//	Padding(0, 0, 0, 2),
		highlightedStyle:       highlightStyle,
		highlightedFailedStyle: highlightStyle.Foreground(failedStyle.GetForeground()),
		highlightedRunStyle:    highlightStyle.Foreground(runStyle.GetForeground()),
		highlightedSkipStyle:   highlightStyle.Foreground(skipStyle.GetForeground()),
		highlightedPassStyle:   highlightStyle.Foreground(passStyle.GetForeground()),
		multiSelectStyle:       multiSelectStyle,
		multiSelectFailedStyle: multiSelectStyle.Foreground(failedStyle.GetForeground()),
		multiSelectRunStyle:    multiSelectStyle.Foreground(runStyle.GetForeground()),
		multiSelectSkipStyle:   multiSelectStyle.Foreground(skipStyle.GetForeground()),
		multiSelectPassStyle:   multiSelectStyle.Foreground(passStyle.GetForeground()),
		failStyle:              failedStyle,
		runStyle:               runStyle,
		skipStyle:              skipStyle,
		passStyle:              passStyle,
		allTestsStyle:          lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#888888")).Padding(0, 0, 0, 2),
	}
}

//func newItemDelegate(keys *delegateKeyMap) list.DefaultDelegate {
//	d := list.NewDefaultDelegate()
//	d.ShowDescription = false
//
//	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
//		//var title string
//		//
//		//if i, ok := m.SelectedItem().(item); ok {
//		//	title = i.Title()
//		//} else {
//		//	return nil
//		//}
//		//switch msg := msg.(type) {
//		//case tea.KeyMsg:
//		//switch {
//		//case key.Matches(msg, keys.choose):
//		//	return m.NewStatusMessage(statusMessageStyle("You chose " + title))
//		//
//		//case key.Matches(msg, keys.remove):
//		//	index := m.Index()
//		//	m.RemoveItem(index)
//		//	if len(m.Items()) == 0 {
//		//		keys.remove.SetEnabled(false)
//		//	}
//		//	return m.NewStatusMessage(statusMessageStyle("Deleted " + title))
//		//}
//		//}
//
//		return nil
//	}
//
//	help := []key.Binding{keys.choose, keys.remove}
//
//	d.ShortHelpFunc = func() []key.Binding {
//		return help
//	}
//
//	d.FullHelpFunc = func() [][]key.Binding {
//		return [][]key.Binding{help}
//	}
//
//	return d
//}

type delegateKeyMap struct {
	choose key.Binding
	remove key.Binding
}

// Additional short help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		d.choose,
		d.remove,
	}
}

// Additional full help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			d.choose,
			d.remove,
		},
	}
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
		remove: key.NewBinding(
			key.WithKeys("x", "backspace"),
			key.WithHelp("x", "delete"),
		),
	}
}
