package studio

import (
	"context"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/debug"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/model"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/model/outputpane"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/model/referencespane"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/state"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio/xhelp"
	busevent "github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	busparser "github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"
	"github.com/wagoodman/go-partybus"
)

type Config struct {
	ID              string
	RunController   state.RunController
	RunStore        state.RunStore
	SessionInfo     test.SessionInfo
	Debug           bool
	FailedTestsOnly bool

	// hidden options
	showVerticalBorder bool
}

func setHiddenDefaults(cfg *Config) {
	cfg.showVerticalBorder = false
}

type Model struct {
	config                    Config
	controller                *controller
	running                   *sync.WaitGroup
	help                      xhelp.Model
	state                     state.RunViewer
	selection                 model.Dispatch
	alphaNumericInputDisabled bool
	lastWindowSize            tea.WindowSizeMsg
	*keyMap
}

func New(config Config, wg *sync.WaitGroup) Model {
	zone.NewGlobal()

	wg.Add(1)

	setHiddenDefaults(&config)

	referenceWidthRatio := 0.3
	outputWidthRatio := 1 - referenceWidthRatio

	// barHeight := 1
	// if config.Debug {
	//	barHeight += 1
	//}

	// s := state.New(config.RunController, config.RunStore) // TODO: pass DB object with runID reference that does just in time lookups

	outputPane, err := outputpane.New(
		outputpane.WithWidthRatio(outputWidthRatio),
		// outputpane.WithHighPerformanceRenderer(),
	)
	if err != nil {
		panic(err) // TODO: handle this better
	}

	referencesPane, err := referencespane.New(
		referencespane.WithWidthRatio(referenceWidthRatio),
		referencespane.WithShowFailedOnly(config.FailedTestsOnly),
	)
	if err != nil {
		panic(err) // TODO: handle this better
	}

	defaultKeys := newKeyMap()

	selection := model.NewDispatch(defaultKeys)
	selection.Add(referencespane.Name, referencesPane)
	selection.Add(outputpane.Name, outputPane)

	m := Model{
		//state:          s,
		controller: newController(config.RunController),
		running:    wg,
		config:     config,
		help:       xhelp.New(),
		selection:  selection,
		keyMap:     defaultKeys,
	}
	selection.SetViewer(m.dispatchView)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.selection.Init(), m.controller.switchToLatestStoredTestRun(m.config))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// if x, ok := msg.(partybus.Event); ok {
	//	eventStr := fmt.Sprintf("%#v", x)
	//	if len(eventStr) > 100 {
	//		eventStr = eventStr[:100] + "..."
	//	}
	//	debug.SetLine(eventStr)
	//}
	// if strings.Contains(eventStr, "GoTestRunType") {
	//	panic("got it")
	//}
	// time.Sleep(500 * time.Millisecond)

	// TODO: respond to event.SwitchTestRun

	var cmds []tea.Cmd

	switch x := msg.(type) {
	// update model state based on UI interactions...
	case uievent.SwitchTestRun:
		m.state = state.NewRunViewer(x.TestRun)
		m.controller.updateTestRun(x.TestRun)
	case uievent.SelectedTestReferences:
		m.controller.updateSelection(x.Refs)
	case uievent.FilteringInput:
		// debug.SetLine(fmt.Sprintf("alpha numeric controllable: %v", x.Completed))
		m.alphaNumericInputDisabled = !x.Completed

	case tea.WindowSizeMsg:
		m.lastWindowSize = x
		// remove any unavailable vertical space for other panes
		helpHeight := m.help.Height(m.keyMap)
		x.Height -= helpHeight // help bar
		if m.config.Debug {
			x.Height-- // debug bar
		}
		msg = x

	// update model state based on core application behavior...
	case partybus.Event:
		switch x.Type {
		// case busevent.GoTestType:
		//	// make this a little easier for downstream consumers by unwrapping the partybus object into
		//	// a business type before sending downstream
		//	e, err := busparser.ParseGoTestType(x)
		//	if err != nil {
		//		panic("errg, no event parsed?" + err.Error()) // TODO: nope
		//	}
		//	msg = e
		case busevent.GoTestRunType:
			m.onRunTestsCompletion()
			r, err := busparser.ParseGoTestRunType(x)
			if err != nil {
				panic("errg, no event parsed?" + err.Error()) // TODO: nope
			}
			cmds = append(cmds, func() tea.Msg {
				return uievent.SwitchTestRun{
					TestRun: r,
				}
			})
		}
		// case gotest.Event:
		// debug.SetLine(fmt.Sprintf("event: %+v", msg))
		// panic("got it!")
		// m.state.(state.Updater).Update(msg)
	}

	// update panes...
	s, cmd := m.selection.Update(msg)
	m.selection = s.(model.Dispatch)
	cmds = append(cmds, cmd)

	cmds = append(cmds, m.respondToGlobalKeybindings(msg))

	return m, tea.Batch(cmds...)
}

func (m *Model) respondToGlobalKeybindings(msg tea.Msg) tea.Cmd {
	// respond to global keybindings...
	switch x := msg.(type) {
	case tea.KeyMsg:
		// debug.SetLine(fmt.Sprintf("key: %+v disabled: %v isAlpha: %v", x, m.alphaNumericInputDisabled, model.IsAlphaNumeric(x)))

		if m.alphaNumericInputDisabled && model.IsAlphaNumeric(x) {
			return nil
		}

		switch {
		// case key.Matches(msg, defaultKeys.Help):
		//	m.help.ShowAll = !m.help.ShowAll

		case key.Matches(x, m.Quit.Binding):
			m.running.Done()
			return tea.Quit

		case key.Matches(x, m.Help.Binding):
			m.help.ShowAll = !m.help.ShowAll
			return func() tea.Msg {
				// trigger a resizing
				return m.lastWindowSize
			}

		case key.Matches(x, m.ReRunAllTests.Binding):
			return m.onReRunTests(true)

		case key.Matches(x, m.ReRunTestSelection.Binding):
			return m.onReRunTests(false)
		}
	}

	return nil
}

func (m Model) onReRunTests(all bool) tea.Cmd {
	cmd := m.controller.startTestReRun(context.TODO(), all) // TODO: can we even use context in a valid capacity here?
	if cmd != nil {
		m.ReRunTestSelection.SetEnabled(false)
		m.ReRunAllTests.SetEnabled(false)
	}
	return cmd
}

func (m Model) onRunTestsCompletion() {
	m.ReRunTestSelection.SetEnabled(true)
	m.ReRunAllTests.SetEnabled(true)
}

func (m Model) Wait() {
	m.running.Wait()
}

func (m Model) dispatchView(dispatch model.Dispatch) string {
	refPaneView := dispatch.Get(referencespane.Name).View()
	if m.config.showVerticalBorder {
		rightBrd := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("#FFFFFF"))

		refPaneView = rightBrd.Render(refPaneView)
	}
	// else {
	//	// pad the right side of the reference pane with one space
	//	rightBrd := lipgloss.NewStyle().
	//		Border(lipgloss.HiddenBorder(), false, true, false, false)
	//
	//	refPaneView = rightBrd.Render(refPaneView)
	// }refPaneView

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		refPaneView,
		dispatch.Get(outputpane.Name).View(),
	)
}

func (m Model) View() string {
	var rows []string

	rows = append(rows, m.dispatchView(m.selection))

	btmtBrd := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("#FFFFFF"))

	top := btmtBrd.Render(strings.Join(rows, "\n"))

	rows = []string{top}

	if m.config.Debug {
		rows = append(rows, debug.Get())
	}
	rows = append(rows, m.help.View(m.selection.KeyMap(), m.config.ID, m.lastWindowSize.Width))
	rendered := strings.Join(rows, "\n")

	// needed for responding to mouse events
	return zone.Scan(rendered)
}
