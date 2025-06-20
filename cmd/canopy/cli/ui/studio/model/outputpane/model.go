package outputpane

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/format/handler/gopp"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/fragment"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ide"
)

var _ tea.Model = (*Model)(nil)

const Name = "output-pane"

type Model struct {
	content string

	config Config

	// view Model components
	ready          bool
	viewport       viewport.Model
	testCountsView fragment.TestCounts
	allSelected    bool

	currentTestRun    state.RunViewer
	selectedRefs      []gotest.Reference
	selectedTestCount int

	widthOverride  int
	heightOverride int

	viewModel handler.Handler

	keyMap
}

func New(options ...Option) (Model, error) {
	cfg, err := apply(options...)
	if err != nil {
		return Model{}, err
	}

	return Model{
		config: cfg,
		viewport: viewport.Model{
			HighPerformanceRendering: cfg.UseHighPerformanceRenderer,
		},
		testCountsView: fragment.NewTestCounts(),
		keyMap:         newKeyMap(),
	}, nil
}

func (m *Model) onSetRun(run state.RunViewer) {
	m.currentTestRun = run
}

func (m *Model) refreshSelect() error {
	return m.onSelect(m.currentTestRun.Config(), m.selectedRefs)
}

func (m *Model) onSelect(cfg gotest.RunnerConfig, refs []gotest.Reference) error {
	// debug.SetLine(fmt.Sprintf("[output] onSelect refs=%d (%d)", len(refs), rand.Int()))

	m.selectedRefs = refs
	m.selectedTestCount = 0
	for _, ref := range refs {
		if !ref.IsPackage() {
			m.selectedTestCount++
		}
	}
	m.viewModel = newViewModel(cfg.UserArgs)

	var events []gotest.Event
	for _, ref := range refs {
		events = append(events, m.currentTestRun.ReferenceEvents(ref)...)
	}

	// sort events chronologically
	sort.Slice(events, func(i, j int) bool {
		return events[i].Index < events[j].Index
	})

	for _, e := range events {
		err := m.viewModel.OnGoTestEvent(e)
		if err != nil {
			if !errors.Is(err, gopp.ErrPackageComplete) {
				return err
			}
		}
	}

	m.SetContent(m.viewModel.String())
	return nil
}

type viewModel struct {
	handler.Handler
	buffer *bytes.Buffer
}

func newViewModel(userArgs []string) viewModel {
	sb := bytes.Buffer{}

	// TODO: this is broken... and the user can still select this via the config and not CLI
	isVerbose := slices.Contains(userArgs, "-v")
	var hnd handler.Handler
	if isVerbose {
		hnd = gopp.NewVerboseHandler(
			&sb,
			gopp.VerbosePackageConfig{
				Color:            true,
				PackageNameWidth: 50,
				IDE:              ide.Select(&ide.OSEnvironmentGetter{}),
			},
		)
	} else {
		hnd = gopp.NewDefaultHandler(
			&sb,
			gopp.DefaultPackageConfig{
				Color:            true,
				PackageNameWidth: 50,
				IDE:              ide.Select(&ide.OSEnvironmentGetter{}),
			},
		)
	}

	h := gopp.NewMultiPackageHandler(
		func(_ gotest.Reference, _ io.Writer) handler.Handler {
			return hnd
		},
	)

	return viewModel{
		Handler: h,
		buffer:  &sb,
	}
}

func (v viewModel) String() string {
	sb := bytes.Buffer{}
	sb.WriteString(v.buffer.String())
	sb.WriteString(v.Handler.String())
	return sb.String()
}

func (m *Model) SetContent(content string) {
	m.content = content
	m.viewport.SetContent(m.content)
}

func (m *Model) SetWidth(width int) {
	m.widthOverride = width
}

func (m *Model) SetHeight(height int) {
	m.heightOverride = height
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint: funlen
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case event.SwitchTestRun:
		m.onSetRun(state.NewRunViewer(msg.TestRun))
		if err := m.refreshSelect(); err != nil {
			panic("output pane: " + err.Error())
		}

	case event.SelectedTestReferences:
		if m.currentTestRun != nil {
			m.allSelected = msg.All
			if err := m.onSelect(m.currentTestRun.Config(), msg.Refs); err != nil {
				panic("output pane: " + err.Error())
			}
		}

	// update core model state...
	case gotest.Event:
		if m.viewModel != nil {
			if err := m.viewModel.OnGoTestEvent(msg); err != nil {
				panic("erg output model: " + err.Error()) // TODO: make this more robust
			}
			// TODO: this will scroll to the top of the output pane
			// m.viewport.SetContent(m.viewModel.String())
		}

	// update view state...
	// TODO: augment the WS on the parent to affect the children
	case tea.WindowSizeMsg:
		var width int
		if m.widthOverride > 0 {
			width = m.widthOverride - 1 // border
		} else {
			width = (int(float64(msg.Width) * m.config.WidthRatio)) - 1 // border
		}

		var height int
		if m.heightOverride > 0 {
			height = m.heightOverride
		} else {
			height = msg.Height - 5 // stats line
		}

		if !m.ready {
			// Since this program is using the full size of the viewport we
			// need to wait until we've received the window dimensions before
			// we can initialize the viewport. The initial dimensions come in
			// quickly, though asynchronously, which is why we wait for them
			// here.
			m.viewport = viewport.New(width, height)
			// m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = m.config.UseHighPerformanceRenderer
			m.viewport.SetContent(m.content)
			m.ready = true

			// This is only necessary for high performance rendering, which in
			// most cases you won't need.
			//
			// Render the viewport one line below the header.
			// m.viewport.YPosition = headerHeight + 1
		} else {
			m.viewport.Width = width
			m.viewport.Height = height
		}

		if m.config.UseHighPerformanceRenderer {
			// Render (or re-render) the whole viewport. Necessary both to
			// initialize the viewport and when the window is resized.
			//
			// This is needed for high-performance rendering only.
			cmds = append(cmds, viewport.Sync(m.viewport))
		}
	}

	// switch msg := msg.(type) {
	//
	// case tea.KeyMsg:
	//	switch {
	//	//case key.Matches(msg, m.SelectAllTests.Binding):
	//	//	panic("select all!")
	//	case key.Matches(msg, m.ReRunAllTests.Binding):
	//		debug.SetLine("on key: output-pane: re-run all tests")
	//	case key.Matches(msg, m.ReRunTestSelection.Binding):
	//		debug.SetLine("on key: output-pane: re-run selected tests")
	//	}
	//}

	// Handle keyboard and mouse events in the viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// func (m Model) view() string {
//	if !m.ready {
//		return "\n  Initializing..."
//	}
//	return m.viewport.view()
//
//}

func (m Model) View() string {
	outputView := m.viewport.View()
	if !m.ready || len(strings.TrimSpace(outputView)) == 0 {
		outputView = lipgloss.NewStyle().Faint(true).Italic(true).Render("\nNo output")
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.statsView(), outputView)
}

func (m Model) statsView() string {
	pkgTitle := "package"

	pkgs := make(map[string]struct{})
	passedTests := 0
	skippedTests := 0
	failedTests := 0
	runningTests := 0
	tests := 0
	for _, ref := range m.selectedRefs {
		if _, ok := pkgs[ref.Package]; !ok {
			pkgs[ref.Package] = struct{}{}
		}
		if ref.IsPackage() {
			continue
		}
		tests++
		switch m.currentTestRun.ReferenceConclusion(ref) {
		case gotest.PassAction:
			passedTests++
		case gotest.SkipAction:
			skippedTests++
		case gotest.FailAction:
			failedTests++
		case gotest.RunAction:
			runningTests++
		}
	}

	selection := "no tests selected"
	if tests != 0 {
		if len(pkgs) > 1 || len(pkgs) == 0 {
			pkgTitle = "packages"
		}

		testTitle := "tests"
		if tests == 1 {
			testTitle = "test"
		}

		selection = fmt.Sprintf("selected %d %s across %d %s", tests, testTitle, len(pkgs), pkgTitle)
	}

	var testsSummary string
	if !m.allSelected {
		testsSummary = m.testCountsView.View(passedTests, failedTests, skippedTests)
	}

	width := m.viewport.Width
	left := lipgloss.JoinHorizontal(lipgloss.Top, testsSummary)
	right := m.config.SummaryLineStyle.Width(width - lipgloss.Width(left)).Align(lipgloss.Right).Render(selection)
	line := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return m.config.BorderSummaryStyle.Width(width).Render(line)
}

// func formatDuration(d time.Duration) string {
//	hours := d / time.Hour
//	d -= hours * time.Hour
//	minutes := d / time.Minute
//	d -= minutes * time.Minute
//	seconds := float64(d) / float64(time.Second)
//
//	if hours > 0 {
//		return fmt.Sprintf("%d:%02d:%06.2f", hours, minutes, seconds)
//	} else if minutes > 0 {
//		return fmt.Sprintf("%d:%06.2f", minutes, seconds)
//	} else {
//		return fmt.Sprintf("%.2fs", seconds)
//	}
//}
//
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
