package selector

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

// Config holds configuration options for the selector UI.
type Config struct {
	// ID is the unique identifier for this selector instance, displayed in the UI footer.
	ID string
	// Debug enables debug logging for the selector.
	Debug bool
}

// filterKeyBindings holds key bindings for individual letter characters used in filtering.
// initialized in init() to include all a-z and A-Z characters.
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

// Model is the Bubble Tea model for the interactive test selector UI.
// it manages the list of test references, user selections, and visual styling.
type Model struct {
	// config holds the selector configuration.
	config Config
	// size tracks the current terminal window dimensions.
	size tea.WindowSizeMsg
	// list is the underlying Bubble Tea list component displaying test items.
	list list.Model
	// keyMap defines the keyboard shortcuts for selector actions.
	keyMap keyMap

	// selected holds the test references currently selected by the user.
	selected gotest.References
	// finished indicates whether the user has confirmed their selection.
	finished bool
	// cancelled indicates whether the user has cancelled test selection.
	cancelled bool

	// titleStyle is the style used for the main title text.
	titleStyle lipgloss.Style
	// cancelledStyle is the style used when displaying cancellation status.
	cancelledStyle lipgloss.Style
	// auxStyle is the style used for auxiliary UI elements like stats and ID.
	auxStyle lipgloss.Style
	// filterTitleStyle is the style used for the filter indicator.
	filterTitleStyle lipgloss.Style
}

// New creates a new selector model with the given configuration.
// it initializes the list component with custom filtering and key bindings.
func New(config Config) Model {
	// zone.NewGlobal()

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

	// we want a little more control over how filtering works, so we provide our own filter function
	l.Filter = filter

	l.AdditionalShortHelpKeys = km.AdditionalShortHelp
	l.AdditionalFullHelpKeys = km.AdditionalFullHelp

	return Model{
		config:     config,
		list:       l,
		keyMap:     km,
		titleStyle: lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#04B5F7")),
		cancelledStyle: lipgloss.NewStyle().Italic(true).
			Foreground(matchColor),
		auxStyle: lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{
			Light: "#909090",
			Dark:  "#626262",
		}),
		filterTitleStyle: lipgloss.NewStyle().
			Foreground(matchColor),
	}
}

// Selected returns the test references selected by the user.
// it returns nil if the selection was cancelled or not yet finished.
func (m Model) Selected() []gotest.Reference {
	if !m.finished || m.cancelled {
		return nil
	}

	return m.selected
}

// Init implements tea.Model and returns the initial command for the model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and updates the model state accordingly.
// it processes window resize events, selection events, and keyboard input.
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
		if msg.Finished {
			cmds = append(cmds, tea.Quit)
		}
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.list.KeyMap.ForceQuit):
			m.cancelled = true
		case key.Matches(msg, m.list.KeyMap.Quit):
			// if we are not filtering, then treat this as a cancel
			if !m.list.SettingFilter() && !m.list.IsFiltered() {
				m.cancelled = true
			}
		}
	}

	// handle list updates...
	cmds = append(cmds, m.updateList(msg))

	return m, tea.Batch(cmds...)
}

// updateList handles list-specific updates and manages filter state transitions.
// it ensures that reference display format (short/long) is updated appropriately
// when entering or exiting filter mode.
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

// completed returns true if the selection process has finished or been cancelled.
func (m Model) completed() bool {
	return m.finished || m.cancelled
}

// View renders the complete selector UI.
func (m Model) View() string {
	return m.view()
	// return zone.Scan(m.view())
}

// view builds the complete UI by joining the top, references, and bottom views.
func (m Model) view() string {
	views := []string{
		m.topView(),
	}

	if !m.completed() {
		views = append(views, m.refrencesView(), m.bottomView())
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		views...,
	)
}

// joinedView creates a horizontal layout with left and right content justified
// to the edges of the terminal, with spacing in between.
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

// topView renders the title bar with selection status and filter indicator.
func (m Model) topView() string {
	msg := "Search/Select tests to run"
	st := m.titleStyle
	if m.completed() {
		switch {
		case m.cancelled:
			msg = "Test run cancelled"
			st = m.cancelledStyle
		case len(m.selected) == 0:
			msg = `¯\_(ツ)_/¯`
		default:
			msg = fmt.Sprintf("Running %d test functions", m.selected.TestFunctionsCount())
		}
	}
	title := st.Render(msg)

	if !m.completed() && (m.list.SettingFilter() || m.list.IsFiltered()) {
		title += m.filterTitleStyle.Render(" [filter]") + ": " + m.list.FilterValue()
	}

	if !m.completed() {
		return m.joinedView(title, m.statsView())
	}

	return m.auxStyle.Render(m.config.ID) + " " + title
}

// bottomView renders the pagination indicator, help text, and config ID.
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

// statsView renders the selection statistics (e.g., "selected 5 tests").
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

// refrencesView renders the list of test references.
func (m Model) refrencesView() string {
	return m.list.View()
}
