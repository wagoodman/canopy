package selector

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
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

	selected gotest.References // the currently selected references
	finished bool

	titleStyle       lipgloss.Style
	auxStyle         lipgloss.Style
	filterTitleStyle lipgloss.Style
}

func New(config Config) Model {
	//zone.NewGlobal()

	km := newKeyMap()

	l := list.New(
		newItems(false, false), // empty, but will be populated later with an event
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
		keyMap:     km,
		titleStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#04B5F7")),
		auxStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
			Light: "#909090",
			Dark:  "#626262",
		}),
		filterTitleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"}),
	}
}

func (m Model) Selected() []gotest.Reference {
	return m.selected
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
		m.selected = msg.Refs
		m.finished = msg.Finished
		if m.finished {
			cmds = append(cmds, tea.Quit)
		}
	}

	// handle list updates...
	cmds = append(cmds, m.updateList(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) updateList(msg any) tea.Cmd {
	var cmds []tea.Cmd
	// get a sense of the current filter state before anything is updated/applied
	wasFiltering := m.list.SettingFilter() || m.list.IsFiltered()

	// if we are about to start filtering (but have not yet done so) then we need to let the list model know
	// that the format for test references should be in long form, but only temporarily.
	// Why do we need to do this here and not in the delegate? It seems that the delegate's update function is not called
	// in the filtering path within the list model Update() function. That means the wrapping model needs to explicitly
	// let the delegate know in advance that it is about to filter.
	var startingListModel *list.Model
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// don't match any of the keys below if we're actively filtering
		if m.list.SettingFilter() {
			break
		}

		switch {
		case key.Matches(msg, m.list.KeyMap.Filter):
			// we know that filtering is about to start, so we need to tell the list model to use long form
			// references, but only temporarily.
			updatedListModel, cmd := m.list.Update(uievent.RefreshReferences{AboutToFilter: true})
			startingListModel = &updatedListModel
			cmds = append(cmds, cmd)
		}
	}

	if startingListModel == nil {
		// we are not about to filter, so we can just use the current list model
		startingListModel = &m.list
	}

	// this will also call our delegate's update function
	newListModel, cmd := startingListModel.Update(msg)

	// if we observe a filter state change, we need to tell the delegate to update the items (and their format)
	nowFiltering := newListModel.SettingFilter() || newListModel.IsFiltered()
	if nowFiltering != wasFiltering {
		// if we just switched to filtering, we need to update the items
		// to reflect the current filter state.
		cmds = append(cmds, func() tea.Msg { return uievent.RefreshReferences{AboutToFilter: nowFiltering} })
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
	views := []string{
		m.topView(),
	}

	if !m.finished {
		views = append(views, m.refrencesView(), m.bottomView())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		views...,
	)
}

func (m Model) joinedView(left, right string) string {
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

func (m Model) topView() string {
	msg := "Search/Select tests to run"
	if m.finished {
		if len(m.selected) == 0 {
			msg = `¯\_(ツ)_/¯`
		} else {
			msg = fmt.Sprintf("Running %d test functions", m.selected.TestFunctionsCount())
		}
	}
	title := m.titleStyle.Render(msg)

	if !m.finished && (m.list.SettingFilter() || m.list.IsFiltered()) {
		title += m.filterTitleStyle.Render(" [filter]") + ": " + m.list.FilterValue()
	}

	if !m.finished {
		return m.joinedView(title, m.statsView())
	}

	return m.auxStyle.Render(m.config.ID) + " " + title
}

func (m Model) bottomView() string {
	var page string
	if m.list.Paginator.TotalPages > 1 {
		page = m.auxStyle.Reverse(true).Render(" "+m.list.Paginator.View()+" ") + "  "
	}
	return m.joinedView(
		page+m.list.Help.View(m.list),
		m.auxStyle.Render(m.config.ID),
	)
}

func (m Model) statsView() string {
	tests := 0
	for _, ref := range m.selected {
		if ref.IsPackage() {
			continue
		}
		tests++
	}

	selection := "(no tests selected)"
	if tests != 0 {

		testTitle := "tests"
		if tests == 1 {
			testTitle = "test"
		}

		selection = fmt.Sprintf("selected %d %s", tests, testTitle)
	}

	return m.auxStyle.Render(selection)
}

func (m Model) refrencesView() string {
	return m.list.View()
}
