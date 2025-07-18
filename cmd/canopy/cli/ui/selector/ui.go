package selector

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model/references"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/debug"
	uievent "github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/event"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model/referencespane"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/xhelp"
	busevent "github.com/wagoodman/canopy/cmd/canopy/internal/bus/event"
	busparser "github.com/wagoodman/canopy/cmd/canopy/internal/bus/parser"
	"github.com/wagoodman/go-partybus"
)

type Config struct {
	ID string
	//RunController state.RunController
	//RunStore        state.RunStore
	//SessionInfo     test.SessionInfo
	Debug           bool
	FailedTestsOnly bool

	// hidden options
	//showVerticalBorder bool
}

//func setHiddenDefaults(cfg *Config) {
//	cfg.showVerticalBorder = false
//}

type Model struct {
	config Config
	//controller                *controller
	running *sync.WaitGroup
	help    xhelp.Model
	//state                     state.DefinitionViewer
	selection                 model.Dispatch
	alphaNumericInputDisabled bool
	lastWindowSize            tea.WindowSizeMsg
	*keyMap
}

func New(config Config, wg *sync.WaitGroup) Model {
	zone.NewGlobal()

	wg.Add(1)

	//setHiddenDefaults(&config)

	//referencesPane, err := referencespane.New(
	//	referencespane.WithShowFailedOnly(config.FailedTestsOnly),
	//)
	//if err != nil {
	//	panic(err) // TODO: handle this better
	//}

	defaultKeys := newKeyMap()

	selection := model.NewDispatch(defaultKeys)
	selection.Add(referencespane.Name, references.New())

	m := Model{
		//state:          s,
		//controller: newController(config.RunController),
		running:   wg,
		config:    config,
		help:      xhelp.New(),
		selection: selection,
		keyMap:    defaultKeys,
	}
	selection.SetViewer(m.dispatchView)

	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.selection.Init(),
		//m.controller.switchToLatestStoredTestRun(m.config),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	// TODO: respond to event.SwitchState

	var cmds []tea.Cmd

	switch x := msg.(type) {
	// update model state based on UI interactions...
	//case uievent.SwitchState:
	//	m.state = state.NewRunViewer(x.TestRun)
	//m.controller.updateTestRun(x.TestRun)
	//case uievent.SelectedTestReferences:
	//	m.controller.updateSelection(x.Refs)
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
		case busevent.GoTestRunType:
			//m.onRunTestsCompletion()
			r, err := busparser.ParseGoTestRunType(x)
			if err != nil {
				panic("errg, no event parsed?" + err.Error()) // TODO: nope
			}
			cmds = append(cmds, func() tea.Msg {
				return uievent.SwitchState{
					TestRun: r,
				}
			})
		}
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

			//case key.Matches(x, m.ReRunAllTests.Binding):
			//	return m.onReRunTests(true)
			//
			//case key.Matches(x, m.ReRunTestSelection.Binding):
			//	return m.onReRunTests(false)
		}
	}

	return nil
}

//func (m Model) onReRunTests(all bool) tea.Cmd {
//	cmd := m.controller.startTestReRun(context.TODO(), all) // TODO: can we even use context in a valid capacity here?
//	if cmd != nil {
//		m.ReRunTestSelection.SetEnabled(false)
//		m.ReRunAllTests.SetEnabled(false)
//	}
//	return cmd
//}
//
//func (m Model) onRunTestsCompletion() {
//	m.ReRunTestSelection.SetEnabled(true)
//	m.ReRunAllTests.SetEnabled(true)
//}

func (m Model) Wait() {
	m.running.Wait()
}

func (m Model) dispatchView(dispatch model.Dispatch) string {
	refPaneView := dispatch.Get(referencespane.Name).View()
	//if m.config.showVerticalBorder {
	//	rightBrd := lipgloss.NewStyle().
	//		Border(lipgloss.NormalBorder(), false, true, false, false).
	//		BorderForeground(lipgloss.Color("#FFFFFF"))
	//
	//	refPaneView = rightBrd.Render(refPaneView)
	//}

	return refPaneView
}

func (m Model) View() string {
	var rows []string

	rows = append(rows, m.dispatchView(m.selection))

	//btmtBrd := lipgloss.NewStyle().
	//	Border(lipgloss.NormalBorder(), false, false, true, false).
	//	BorderForeground(lipgloss.Color("#FFFFFF"))
	//
	//top := btmtBrd.Render(strings.Join(rows, "\n"))

	//rows = []string{top}

	if m.config.Debug {
		rows = append(rows, debug.Get())
	}
	rows = append(rows, m.help.View(m.selection.KeyMap(), m.config.ID, m.lastWindowSize.Width))
	rendered := strings.Join(rows, "\n")

	// needed for responding to mouse events
	return zone.Scan(rendered)
}
