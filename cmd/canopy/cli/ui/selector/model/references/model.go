package references

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"sort"
)

var filterKeyBindings []key.Binding

func init() {
	// for a-z, A-Z , make bindings for each letter
	for i := 'a'; i <= 'z'; i++ {
		filterKeyBindings = append(filterKeyBindings, key.NewBinding(
			key.WithKeys(string(i)),
			key.WithHelp(fmt.Sprintf("%c", i), "filter by letter"),
		))
	}

	for i := 'A'; i <= 'Z'; i++ {
		filterKeyBindings = append(filterKeyBindings, key.NewBinding(
			key.WithKeys(string(i)),
			key.WithHelp(fmt.Sprintf("%c", i), "filter by letter"),
		))
	}
}

type Model struct {
	list        list.Model
	state       state.DefinitionViewer
	visibleRefs []gotest.Reference
	keyMap      KeyMap
}

func New(km KeyMap) Model {
	l := list.New(
		newItems(false), // empty, but will be populated later with an event
		newItemDelegate(
			km.ShowPassedTests.Binding,
			km.ShowFailedTests.Binding,
			km.ShowSkippedTests.Binding,
			km.NextTestFunc,
			km.PrevTestFunc,
			km.NextPackage,
			km.PrevPackage,
		),
		0,
		0,
	)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.SetShowPagination(false)
	l.Filter = filter
	//l.FilterInput = filterInput()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return km.AdditionalShortHelp()
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return km.AdditionalFullHelp()
	}

	return Model{
		list:   l,
		keyMap: km,
	}
}

//func filterInput() textinput.Model {
//	// 	s.FilterPrompt = lipgloss.NewStyle().
//	//		Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})
//	//
//	//	s.FilterCursor = lipgloss.NewStyle().
//	//		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})
//
//	filterInput := textinput.New()
//	filterInput.Prompt = "Filter: "
//	//filterInput.PromptStyle = styles.FilterPrompt
//	//filterInput.Cursor.Style = styles.FilterCursor
//	filterInput.CharLimit = 64
//	filterInput.Focus()
//	return filterInput
//}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// TODO: what to do about the height?
		m.list.SetSize(msg.Width, msg.Height-4)

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.CursorUp()

		case tea.MouseButtonWheelDown:
			m.list.CursorDown()
		default:
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				for i, listItem := range m.list.VisibleItems() {
					v, _ := listItem.(item)
					// check each item to see if it's in bounds
					if zone.Get(v.title).InBounds(msg) {
						// ...ff so, select it in the list
						m.list.Select(i)
						break
					}
				}
			}
		}

	case tea.KeyMsg:
		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, filterKeyBindings...):
			// if matches a-z, A-Z then we set the filter state to filtering
			m.list.SetFilterState(list.Filtering)

		case key.Matches(msg, m.keyMap.ShowPassedTests.Binding):
			// TODO...
		case key.Matches(msg, m.keyMap.ShowFailedTests.Binding):
			// TODO...
		case key.Matches(msg, m.keyMap.ShowSkippedTests.Binding):
			// TODO...

			//case key.Matches(msg, m.key.toggleSpinner):
			//	cmd := m.list.ToggleSpinner()
			//	return m, cmd
			//
			//case key.Matches(msg, m.keys.toggleTitleBar):
			//	v := !m.list.ShowTitle()
			//	m.list.SetShowTitle(v)
			//	m.list.SetShowFilter(v)
			//	m.list.SetFilteringEnabled(v)
			//	return m, nil
			//
			//case key.Matches(msg, m.keys.toggleStatusBar):
			//	m.list.SetShowStatusBar(!m.list.ShowStatusBar())
			//	return m, nil
			//
			//case key.Matches(msg, m.keys.togglePagination):
			//	m.list.SetShowPagination(!m.list.ShowPagination())
			//	return m, nil
			//
			//case key.Matches(msg, m.keys.toggleHelpMenu):
			//	m.list.SetShowHelp(!m.list.ShowHelp())
			//	return m, nil
			//
			//case key.Matches(msg, m.keys.insertItem):
			//	m.delegateKeys.remove.SetEnabled(true)
			//	newItem := m.itemGenerator.next()
			//	insCmd := m.list.InsertItem(0, newItem)
			//	statusCmd := m.list.NewStatusMessage(statusMessageStyle("Added " + newItem.Title()))
			//	return m, tea.Batch(insCmd, statusCmd)
		}

	// handle core interactions...
	case uievent.SwitchState:
		// TODO: make this work a little better....
		if msg.TestRun != nil {
			cmds = append(cmds, m.onSwitchState(state.NewRunViewer(msg.TestRun)))
		} else if msg.Definitions != nil {
			cmds = append(cmds, m.onSwitchState(state.NewDefinitionViewer(msg.Definitions)))
		} else {
			panic(fmt.Sprintf("unexpected switch state message: %#v", msg))
		}
	}

	// handle list updates...
	cmds = append(cmds, m.updateList(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) updateList(msg any) tea.Cmd {
	var cmds []tea.Cmd
	wasFiltering := m.list.FilterState() == list.Filtering

	// this will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	nowFiltering := m.list.FilterState() == list.Filtering

	if nowFiltering != wasFiltering {
		// if we just switched to filtering, we need to update the items
		// to reflect the current filter state.
		cmds = append(cmds, m.refreshReferences())
	}
	return tea.Batch(cmds...)
}

func (m *Model) onSwitchState(run state.DefinitionViewer) tea.Cmd {
	// TODO: we need to add and remove the difference of the new refs and the old refs
	// then update the viewport selected indexes and cursor position
	m.state = run

	return m.refreshReferences()
}

func (m *Model) refreshReferences() tea.Cmd {
	// This is called when the references change, but we don't have a new state.
	// This can happen when the user switches to a different package or test.
	if m.state == nil {
		return nil
	}

	return m.setReferences(m.state.References()...)
}

func (m *Model) setReferences(refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	m.visibleRefs = m.filterToVisibleRefs(refs, m.state)

	return tea.Batch(
		m.list.SetItems(newItems(m.list.FilterState() == list.Filtering, m.visibleRefs...)),
	)
}

func (m Model) filterToVisibleRefs(original []gotest.Reference, currentDefs state.DefinitionViewer) []gotest.Reference {
	//showFailed := m.ShowFailedTests.Engaged()
	//showPassed := m.ShowPassedTests.Engaged()
	//showSkipped := m.ShowSkippedTests.Engaged()

	//currentTestRun, hasRunInfo := currentDefs.(state.RunViewer)

	var refs []gotest.Reference
	refs = append(refs, gotest.Reference{Package: "*"})
	for _, ref := range original {
		//if hasRunInfo {
		//	action := currentTestRun.ReferenceConclusiveAction(ref)
		//
		//	if action == gotest.FailAction && !showFailed {
		//		continue
		//	}
		//
		//	if action == gotest.PassAction && !showPassed {
		//		continue
		//	}
		//
		//	if action == gotest.SkipAction && !showSkipped {
		//		continue
		//	}
		//}

		refs = append(refs, ref)
	}

	return refs
}
func (m Model) View() string {
	return m.list.View()
}
