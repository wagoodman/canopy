package referencespane

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/fragment"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
)

var _ tea.Model = (*Model)(nil)

const Name = "references-pane"

type Model struct {
	config Config

	spinner        spinner.Model
	list           list.Model
	testCountsView fragment.TestCounts

	// reference state
	visibleRefs []gotest.Reference

	// go test state
	currentTestRun state.RunViewer

	keyMap
}

func New(options ...Option) (Model, error) {
	// TODO: account for the stats line + border

	cfg, err := apply(options...)
	if err != nil {
		return Model{}, err
	}

	km := newKeyMap(cfg.ShowFailedOnly)

	s := spinner.New()
	s.Spinner = spinner.Jump
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // purple... TODO: put this style somewhere central

	return Model{
		spinner: s,
		list: newList(
			km.ShowPassedTests.Binding,
			km.ShowFailedTests.Binding,
			km.ShowSkippedTests.Binding,
			km.NextTestFunc.Binding,
			km.PrevTestFunc.Binding,
			km.NextPackage.Binding,
			km.PrevPackage.Binding,
		),
		testCountsView: fragment.NewTestCounts(),
		keyMap:         km,
		config:         cfg,
	}, nil
}

func (m Model) isRunning() bool {
	if m.currentTestRun == nil {
		return false
	}

	_, isRunning := m.currentTestRun.Passed()
	return isRunning
}

func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
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
	case uievent.SwitchTestRun:
		cmds = append(cmds, m.onSwitchTestRun(state.NewRunViewer(msg.TestRun)))

	case gotest.Event:
		// respond to core application behavior
		cmds = append(cmds, m.refreshRun())

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.isRunning() {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
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
		isFiltering = true // TODO: bad
		m.list.SetShowFilter(true)
	case list.FilterApplied:
		completed = true
		m.ShowFailedTests.SetEnabled(false)
		m.ShowPassedTests.SetEnabled(false)
		m.ShowSkippedTests.SetEnabled(false)
		isFiltering = true // TODO: bad
		m.list.SetShowFilter(false)
	case list.Unfiltered:
		completed = true
		m.ShowFailedTests.SetEnabled(true)
		m.ShowPassedTests.SetEnabled(true)
		m.ShowSkippedTests.SetEnabled(true)
		isFiltering = false // TODO: bad
		m.list.SetShowFilter(false)
	}
	return func() tea.Msg {
		return uievent.FilteringInput{
			Name:      Name,
			Completed: completed,
		}
	}
}

func (m *Model) refreshRun() tea.Cmd {
	return m.onSwitchTestRun(m.currentTestRun)
}

func (m *Model) onSwitchTestRun(run state.RunViewer) tea.Cmd {
	if run == nil {
		// no run loaded yet (e.g. a session with zero runs). every status-toggle and
		// package/func-nav key funnels through refreshRun -> here, so one guard keeps them
		// all from dereferencing a nil RunViewer via run.References().
		return nil
	}
	// TODO: we need to add and remove the difference of the new refs and the old refs
	// then update the viewport selected indexes and cursor position
	m.currentTestRun = run

	return tea.Batch(
		m.setReferences(run.References()...),
		m.spinner.Tick, // restart the spinner
	)
}

func (m *Model) setReferences(refs ...gotest.Reference) tea.Cmd {
	sort.Sort(gotest.References(refs))
	m.visibleRefs = m.filterToVisibleRefs(refs, m.currentTestRun)

	return tea.Batch(
		m.list.SetItems(newItems(m.visibleRefs...)),
	)
}

func (m Model) filterToVisibleRefs(original []gotest.Reference, currentTestRun state.RunViewer) []gotest.Reference {
	showFailed := m.ShowFailedTests.Engaged()
	showPassed := m.ShowPassedTests.Engaged()
	showSkipped := m.ShowSkippedTests.Engaged()

	var refs []gotest.Reference
	refs = append(refs, gotest.Reference{Package: "*"})
	for _, ref := range original {
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

		refs = append(refs, ref)
	}

	return refs
}

func (m Model) statsView() string {
	if m.currentTestRun == nil {
		return ""
	}
	width := m.list.Width() + 1

	stats := m.currentTestRun.TestStats()

	coverage, covExists := m.currentTestRun.Coverage()

	elapsed := formatDuration(m.currentTestRun.Elapsed(m.isRunning()))

	var status string
	if m.isRunning() {
		status = m.spinner.View() + " "

		var statsStr string
		if stats.Total() > 0 {
			statsStr = m.testCountsView.View(stats.Passed, stats.Failed, stats.Skipped)
		} else {
			statsStr = m.testCountsView.AuxStyle.Render("Running...")
		}

		left := lipgloss.JoinHorizontal(lipgloss.Top, status, statsStr)

		right := m.config.SummaryLineStyle.Width(width - lipgloss.Width(left)).Align(lipgloss.Right).Faint(true).Render(elapsed)

		line := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

		return m.config.BorderSummaryStyle.Width(width).Render(line)
	}

	// concluded view...

	if pass, _ := m.currentTestRun.Passed(); pass {
		status = m.testCountsView.PassedCountStyle.Render("✔ ")
	} else {
		status = m.testCountsView.FailedCountStyle.Render("✘ ")
	}

	left := lipgloss.JoinHorizontal(lipgloss.Top, status, m.testCountsView.View(stats.Passed, stats.Failed, stats.Skipped))

	var right string
	var covStr string
	if stats.Total() > 0 && covExists {
		covStr = drop0Decimal(coverage)
		right += "with " + covStr + ", "
	}

	right += "in " + elapsed

	// right aligned variant...
	// right := m.config.SummaryLineStyle.Width(width - lipgloss.Width(left)).Align(lipgloss.Right).Faint(true).Render(right)
	//
	// line := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	line := left + " " + right

	return m.config.BorderSummaryStyle.Width(width).Render(line)
}

func formatDuration(d time.Duration) string {
	hours := d / time.Hour
	d -= hours * time.Hour
	minutes := d / time.Minute
	d -= minutes * time.Minute
	seconds := float64(d) / float64(time.Second)

	switch {
	case hours > 0:
		return fmt.Sprintf("%dh %02dm %02.2fs", hours, minutes, seconds)
	case minutes > 0:
		return fmt.Sprintf("%dm %02.2fs", minutes, seconds)
	default:
		return fmt.Sprintf("%0.2fs", seconds)
	}
}

// func insertBetween(slice []string, str string) []string {
//	if len(slice) == 0 {
//		return slice
//	}
//
//	result := make([]string, 0, len(slice)*2-1)
//
//	for i, s := range slice {
//		result = append(result, s)
//		if i < len(slice)-1 {
//			result = append(result, str)
//		}
//	}
//
//	return result
//}

func (m Model) View() string {
	// debug.SetLine(fmt.Sprintf("item count from view: %d", len(m.list.Items())))
	return lipgloss.JoinVertical(lipgloss.Left, m.statsView(), m.list.View())
}

func drop0Decimal(coverage float64) string {
	truncated := math.Trunc(coverage*100) / 100
	if strconv.FormatFloat(truncated, 'f', 2, 64) == strconv.FormatFloat(math.Trunc(truncated), 'f', 2, 64) {
		return fmt.Sprintf("%d%% coverage", int(truncated))
	}
	return fmt.Sprintf("%.2f%% coverage", truncated)
}
