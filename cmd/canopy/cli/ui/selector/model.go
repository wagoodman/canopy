package selector

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"sort"
	"strings"
)

type Config struct {
	ID    string
	Debug bool
}

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
	config      Config
	size        tea.WindowSizeMsg
	list        list.Model
	state       state.DefinitionViewer
	visibleRefs []gotest.Reference
	keyMap      keyMap

	titleStyle lipgloss.Style
	auxStyle   lipgloss.Style

	Selected []gotest.Reference // references that are selected by the user
}

func New(config Config) Model {
	//zone.NewGlobal()

	km := newKeyMap()

	l := list.New(
		newItems(false), // empty, but will be populated later with an event
		newItemDelegate(
			km.SelectTest,
			[]key.Binding{
				km.NextPackage,
				km.PrevPackage,
			},
		),
		0,
		0,
	)
	// we reserve the right for 'q' for the user to start filtering results
	l.KeyMap.Quit.SetKeys("esc")
	l.KeyMap.Quit.SetHelp("esc", "quit")

	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.SetShowPagination(false)

	// TODO: why isn't this working on test functions? only on packages?
	//l.Filter = filter

	l.AdditionalShortHelpKeys = func() []key.Binding {
		return km.AdditionalShortHelp()
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return km.AdditionalFullHelp()
	}

	return Model{
		config:     config,
		list:       l,
		keyMap:     km,
		titleStyle: lipgloss.NewStyle().Bold(true),
		auxStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
			Light: "#909090",
			Dark:  "#626262",
		}),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// TODO: what to do about the height?
		m.size = msg
		m.list.SetSize(msg.Width, msg.Height-4)

	case uievent.SelectedTestReferences:
		if msg.All {
			m.Selected = m.state.References()
		} else {
			m.Selected = msg.Refs
		}

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.list.CursorUp()

		case tea.MouseButtonWheelDown:
			m.list.CursorDown()
			//default:
			//	if msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft {
			//		for i, listItem := range m.list.VisibleItems() {
			//			v, _ := listItem.(item)
			//			// check each item to see if it's in bounds
			//			if zone.Get(v.title).InBounds(msg) {
			//				// ...ff so, select it in the list
			//				m.list.Select(i)
			//				break
			//			}
			//		}
			//	}
		}

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keyMap.NextPackage):
			cmds = append(cmds, m.nextPkg())

		case key.Matches(msg, m.keyMap.PrevPackage):
			cmds = append(cmds, m.prevPkg())

		}

		// don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, filterKeyBindings...):
			// if matches a-z, A-Z then we set the filter state to filtering
			m.list.SetFilterState(list.Filtering)
		}

	// handle core interactions...
	case uievent.SwitchState:
		cmds = append(cmds, m.onSwitchState(state.NewDefinitionViewer(msg.Definitions)))
	}

	// handle list updates...
	cmds = append(cmds, m.updateList(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) nextPkg() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	for i := currentIdx; i < len(m.visibleRefs); i++ {
		if m.visibleRefs[i].Package != curPkg {
			m.list.Select(i)
			break
		}
	}

	return m.refreshReferences()
}

func (m *Model) prevPkg() tea.Cmd {
	currentIdx := m.list.Index()
	currentElement := m.list.SelectedItem()
	if currentElement == nil {
		return nil
	}
	currentItem := currentElement.(item)
	curPkg := currentItem.ref.Package
	targetPkg := ""
	for i := currentIdx; i >= 0; i-- {
		if targetPkg == "" {
			if m.visibleRefs[i].Package != curPkg {
				targetPkg = m.visibleRefs[i].Package
				continue
			}
		} else {
			// head to the top of the package
			if m.visibleRefs[i].Package != targetPkg {
				// select the previous reference...
				m.list.Select(i + 1)
				break
			}
		}
	}

	return m.refreshReferences()
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
	m.state = run
	return m.refreshReferences()
}

func (m *Model) refreshReferences() tea.Cmd {
	if m.state == nil {
		return nil
	}

	return m.setReferences(m.state.References()...)
}

func (m *Model) setReferences(refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	m.visibleRefs = m.filterToVisibleRefs(refs)

	return tea.Batch(
		m.list.SetItems(newItems(m.list.FilterState() == list.Filtering, m.visibleRefs...)),
	)
}

func (m Model) filterToVisibleRefs(original []gotest.Reference) []gotest.Reference {
	var refs []gotest.Reference
	refs = append(refs, gotest.Reference{Package: "*"})
	refs = append(refs, original...)

	return refs
}

func (m Model) View() string {
	return m.view()
	//return zone.Scan(m.view())
}

func (m Model) view() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.titleView(),
		m.refrencesView(),
	)
}

func (m Model) titleView() string {
	left := m.titleStyle.Render("Search/Select tests to run")
	right := m.auxStyle.Render(m.config.ID)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	spacingWidth := m.size.Width - leftWidth - rightWidth
	if spacingWidth < 0 {
		// only render the left side if we don't have enough space
		return left
	}

	spacing := strings.Repeat(" ", spacingWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacing, right)
}

//func (m Model) statsView() string {
//	pkgTitle := "package"
//
//	pkgs := make(map[string]struct{})
//	passedTests := 0
//	skippedTests := 0
//	failedTests := 0
//	runningTests := 0
//	tests := 0
//	for _, ref := range m.selectedRefs {
//		if _, ok := pkgs[ref.Package]; !ok {
//			pkgs[ref.Package] = struct{}{}
//		}
//		if ref.IsPackage() {
//			continue
//		}
//		tests++
//		switch m.currentTestRun.ReferenceConclusiveAction(ref) {
//		case gotest.PassAction:
//			passedTests++
//		case gotest.SkipAction:
//			skippedTests++
//		case gotest.FailAction:
//			failedTests++
//		case gotest.RunAction:
//			runningTests++
//		}
//	}
//
//	selection := "no tests selected"
//	if tests != 0 {
//		if len(pkgs) > 1 || len(pkgs) == 0 {
//			pkgTitle = "packages"
//		}
//
//		testTitle := "tests"
//		if tests == 1 {
//			testTitle = "test"
//		}
//
//		selection = fmt.Sprintf("selected %d %s across %d %s", tests, testTitle, len(pkgs), pkgTitle)
//	}
//
//	var testsSummary string
//	if !m.allSelected {
//		testsSummary = m.testCountsView.View(passedTests, failedTests, skippedTests)
//	}
//
//	width := m.viewport.Width
//	left := lipgloss.JoinHorizontal(lipgloss.Top, testsSummary)
//	right := m.config.SummaryLineStyle.Width(width - lipgloss.Width(left)).Align(lipgloss.Right).Render(selection)
//	line := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
//	return m.config.BorderSummaryStyle.Width(width).Render(line)
//}

func (m Model) refrencesView() string {
	return m.list.View()
}
