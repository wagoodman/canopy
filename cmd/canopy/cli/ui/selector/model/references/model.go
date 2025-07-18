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

type Model struct {
	list        list.Model
	state       state.DefinitionViewer
	visibleRefs []gotest.Reference

	keyMap
}

func New() Model {
	km := newKeyMap(false)

	l := list.New(
		newItems(false), // empty, but will be populated later with an event
		newItemDelegate(
			km.ShowPassedTests.Binding,
			km.ShowFailedTests.Binding,
			km.ShowSkippedTests.Binding,
			km.NextTestFunc.Binding,
			km.PrevTestFunc.Binding,
			km.NextPackage.Binding,
			km.PrevPackage.Binding,
		),
		0,
		0,
	)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.Filter = filter
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			km.ShowPassedTests.Binding,
			km.ShowFailedTests.Binding,
			km.ShowSkippedTests.Binding,
			km.NextTestFunc.Binding,
			km.PrevTestFunc.Binding,
			km.NextPackage.Binding,
			km.PrevPackage.Binding,
		}
	}

	return Model{
		list:   l,
		keyMap: km,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)

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

		//switch {
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

		//case key.Matches(msg, m.keys.insertItem):
		//	m.delegateKeys.remove.SetEnabled(true)
		//	newItem := m.itemGenerator.next()
		//	insCmd := m.list.InsertItem(0, newItem)
		//	statusCmd := m.list.NewStatusMessage(statusMessageStyle("Added " + newItem.Title()))
		//	return m, tea.Batch(insCmd, statusCmd)
		//}

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

	wasFiltering := m.list.FilterState() == list.Filtering

	// This will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel
	cmds = append(cmds, cmd)

	nowFiltering := m.list.FilterState() == list.Filtering

	if nowFiltering != wasFiltering {
		// If we just switched to filtering, we need to update the items
		// to reflect the current filter state.
		cmds = append(cmds, m.refreshReferences())
	}

	return m, tea.Batch(cmds...)
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
