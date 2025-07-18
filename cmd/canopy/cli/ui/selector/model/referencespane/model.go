package referencespane

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

var _ tea.Model = (*Model)(nil)

const Name = "references-pane"

type Model struct {
	config Config

	list list.Model

	// reference state
	visibleRefs []gotest.Reference

	// go test state
	currentTestRun state.DefinitionViewer

	keyMap
}

func New(options ...Option) (Model, error) {
	// TODO: account for the stats line + border

	cfg, err := apply(options...)
	if err != nil {
		return Model{}, err
	}

	km := newKeyMap(cfg.ShowFailedOnly)

	return Model{
		list: newList(
			km.ShowPassedTests.Binding,
			km.ShowFailedTests.Binding,
			km.ShowSkippedTests.Binding,
			km.NextTestFunc.Binding,
			km.PrevTestFunc.Binding,
			km.NextPackage.Binding,
			km.PrevPackage.Binding,
		),
		keyMap: km,
		config: cfg,
	}, nil
}

func (m Model) isRunning() bool {
	return false // TODO: implement this properly
	//if m.currentTestRun == nil {
	//	return false
	//}
	//
	//_, isRunning := m.currentTestRun.Passed()
	//return isRunning
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint: funlen
	var cmds []tea.Cmd

	// respond to UI interactions...
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		width := int(float64(msg.Width) * m.config.WidthRatio)
		height := msg.Height - 5 // stats line
		m.list.SetSize(width, height)

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.CursorUp()
			// m.list.CursorUp()
			// m.list.CursorUp()

		case tea.MouseButtonWheelDown:
			m.list.CursorDown()
			// m.list.CursorDown()
			// m.list.CursorDown()
		default:
			if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
				for i, listItem := range m.list.VisibleItems() {
					v, _ := listItem.(item)
					// check each item to see if it's in bounds
					if zone.Get(v.id).InBounds(msg) {
						// ...ff so, select it in the list
						m.list.Select(i)
						break
					}
				}
			}
		}

	case tea.KeyMsg:

		switch {
		case key.Matches(msg, m.NextPackage.Binding):
			cmds = append(cmds, m.nextPkg())

		case key.Matches(msg, m.PrevPackage.Binding):
			cmds = append(cmds, m.prevPkg())

		case key.Matches(msg, m.NextTestFunc.Binding):
			cmds = append(cmds, m.nextTestFn())

		case key.Matches(msg, m.PrevTestFunc.Binding):
			cmds = append(cmds, m.prevTestFn())

		case key.Matches(msg, m.ShowFailedTests.Binding):
			m.ShowFailedTests.Press()
			cmds = append(cmds, m.refreshRun())

		case key.Matches(msg, m.ShowPassedTests.Binding):
			m.ShowPassedTests.Press()
			cmds = append(cmds, m.refreshRun())

		case key.Matches(msg, m.ShowSkippedTests.Binding):
			m.ShowSkippedTests.Press()
			cmds = append(cmds, m.refreshRun())

			// case key.Matches(msg, m.SelectAllTests.Binding):
			//	panic("select all!")

			// case key.Matches(msg, m.list.KeyMap.CursorDown, m.list.KeyMap.CursorUp, m.list.KeyMap.PrevPage, m.list.KeyMap.NextPage, m.list.KeyMap.AcceptWhileFiltering, m.list.KeyMap.CancelWhileFiltering, m.list.KeyMap.Filter):
			//	cmds = append(cmds, m.refreshRun())
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

	case gotest.Event:
		// respond to core application behavior
		cmds = append(cmds, m.refreshRun())

	}

	filterStateBefore := m.list.FilterState()

	// TODO: update refs when new refs come in and set content and reset viewport cursor to 0
	vp, cmd := m.list.Update(msg)
	m.list = vp
	cmds = append(cmds, cmd)

	if filterStateBefore != m.list.FilterState() {
		cmds = append(cmds, m.toggleFilter(m.list.FilterState()))
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) toggleFilter(filterState list.FilterState) tea.Cmd {
	var completed bool
	switch filterState {
	case list.Filtering:
		completed = false
		m.ShowFailedTests.SetEnabled(false)
		m.ShowPassedTests.SetEnabled(false)
		m.ShowSkippedTests.SetEnabled(false)
		//isFiltering = true // TODO: bad
		//m.list.SetShowFilter(true)
	case list.FilterApplied:
		completed = true
		m.ShowFailedTests.SetEnabled(false)
		m.ShowPassedTests.SetEnabled(false)
		m.ShowSkippedTests.SetEnabled(false)
		//isFiltering = true // TODO: bad
		//m.list.SetShowFilter(false)
	case list.Unfiltered:
		completed = true
		m.ShowFailedTests.SetEnabled(true)
		m.ShowPassedTests.SetEnabled(true)
		m.ShowSkippedTests.SetEnabled(true)
		//isFiltering = false // TODO: bad
		//m.list.SetShowFilter(false)
	}
	return func() tea.Msg {
		return uievent.FilteringInput{
			Name:      Name,
			Completed: completed,
		}
	}
}

func (m *Model) refreshRun() tea.Cmd {
	return m.onSwitchState(m.currentTestRun)
}

func (m *Model) onSwitchState(run state.DefinitionViewer) tea.Cmd {
	// TODO: we need to add and remove the difference of the new refs and the old refs
	// then update the viewport selected indexes and cursor position
	m.currentTestRun = run

	return tea.Batch(
		m.setReferences(run.References()...),
	)
}

func (m *Model) setReferences(refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	m.visibleRefs = m.filterToVisibleRefs(refs, m.currentTestRun)

	return tea.Batch(
		m.list.SetItems(newItems(m.visibleRefs...)),
	)
}

func (m Model) filterToVisibleRefs(original []gotest.Reference, currentDefs state.DefinitionViewer) []gotest.Reference {
	showFailed := m.ShowFailedTests.Engaged()
	showPassed := m.ShowPassedTests.Engaged()
	showSkipped := m.ShowSkippedTests.Engaged()

	currentTestRun, hasRunInfo := currentDefs.(state.RunViewer)

	var refs []gotest.Reference
	refs = append(refs, gotest.Reference{Package: "*"})
	for _, ref := range original {
		if hasRunInfo {
			action := currentTestRun.ReferenceConclusiveAction(ref)

			if action == gotest.FailAction && !showFailed {
				continue
			}

			if action == gotest.PassAction && !showPassed {
				continue
			}

			if action == gotest.SkipAction && !showSkipped {
				continue
			}
		}

		refs = append(refs, ref)
	}

	return refs
}

func (m Model) View() string {
	return m.list.View()
}
