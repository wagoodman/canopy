package selector

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
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
	config Config
	size   tea.WindowSizeMsg
	list   list.Model
	keyMap keyMap

	refState referenceState // the current state of the references

	titleStyle       lipgloss.Style
	auxStyle         lipgloss.Style
	filterTitleStyle lipgloss.Style

	Selected []gotest.Reference // references that are selected by the user
}

func New(config Config) Model {
	//zone.NewGlobal()

	km := newKeyMap()

	l := list.New(
		newItems(false), // empty, but will be populated later with an event
		newItemDelegate(km),
		0,
		0,
	)
	// we reserve the right for 'q' for the user to start filtering results
	l.KeyMap.Quit.SetKeys("esc")
	l.KeyMap.Quit.SetHelp("esc", "quit")

	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false) // we will handle this
	l.SetShowPagination(false)
	l.Paginator.Type = paginator.Arabic
	l.SetShowFilter(false) // we will handle this

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
		refState:   referenceState{},
		keyMap:     km,
		titleStyle: lipgloss.NewStyle().Bold(true).Italic(true),
		auxStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
			Light: "#909090",
			Dark:  "#626262",
		}),
		filterTitleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"}),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == tea.KeyEscape.String() {
			log.Debugf("selector: received escape key, quitting")
		}
	case tea.WindowSizeMsg:
		// TODO: what to do about the height?
		m.size = msg
		m.list.SetSize(msg.Width, msg.Height-4)

	case uievent.SwitchState:
		m.refState.state = state.NewDefinitionViewer(msg.Definitions)

	case uievent.SelectedTestReferences:
		if msg.All {
			m.Selected = m.refState.state.References()
		} else {
			m.Selected = msg.Refs
		}

	}

	// handle list updates...
	cmds = append(cmds, m.updateList(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) updateList(msg any) tea.Cmd {
	var cmds []tea.Cmd
	wasFiltering := m.list.FilterState() == list.Filtering || m.list.IsFiltered()

	// this will also call our delegate's update function.
	newListModel, cmd := m.list.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.list.KeyMap.ClearFilter) {
			m.list.SetFilterState(list.Unfiltered)
		}
	}

	nowFiltering := newListModel.FilterState() == list.Filtering || m.list.IsFiltered()

	if nowFiltering != wasFiltering {
		// if we just switched to filtering, we need to update the items
		// to reflect the current filter state.
		cmds = append(cmds, m.refState.update(&newListModel))
	}

	m.list = newListModel
	cmds = append(cmds, cmd)

	return tea.Batch(cmds...)
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
		m.bottomView(),
	)
}

func (m Model) titleView() string {
	left := m.titleStyle.Render("Search/Select tests to run")

	if m.list.FilterState() == list.Filtering || m.list.IsFiltered() {
		left += m.filterTitleStyle.Render(" [filter]") + ": " + m.list.FilterValue()
	}

	//right := m.auxStyle.Render(m.config.ID)
	//
	//leftWidth := lipgloss.Width(left)
	//rightWidth := lipgloss.Width(right)
	//
	//spacingWidth := m.size.Width - leftWidth - rightWidth
	//if spacingWidth < 0 {
	//	// only render the left side if we don't have enough space
	//	return left
	//}
	//
	//spacing := strings.Repeat(" ", spacingWidth)
	//
	//return lipgloss.JoinHorizontal(lipgloss.Top, left, spacing, right)
	return left
}

func (m Model) bottomView() string {
	var page string
	if m.list.Paginator.TotalPages > 1 {
		page = m.list.Paginator.View() + "  "
	}
	return m.joinedView(
		page+m.list.Help.View(m.list),
		m.auxStyle.Render(m.config.ID),
	)
}

func (m Model) joinedView(left, right string) string {
	//left := m.titleStyle.Render("Search/Select tests to run")

	//if m.list.FilterState() == list.Filtering || m.list.IsFiltered() {
	//	left += m.filterTitleStyle.Render(" [filter]") + ": " + m.list.FilterValue()
	//}

	//right := m.auxStyle.Render(m.config.ID)

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
