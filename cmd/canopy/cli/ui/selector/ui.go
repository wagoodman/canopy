package selector

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model/references"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector/model/toggle"
)

type Config struct {
	ID string
	//RunController state.RunController
	//RunStore        state.RunStore
	//SessionInfo     test.SessionInfo
	Debug           bool
	FailedTestsOnly bool
}

type Model struct {
	config Config
	toggle tea.Model
	list   tea.Model
	keyMap references.KeyMap
}

func New(config Config) Model {
	zone.NewGlobal()

	km := references.NewKeyMap()

	return Model{
		config: config,
		list:   references.New(km),
		toggle: toggle.NewModel(km.Toggles()...),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	//switch msg.(type) {
	//case tea.QuitMsg:
	//	m.running.Done()
	//}

	var cmd tea.Cmd
	m.toggle, cmd = m.toggle.Update(msg)
	cmds = append(cmds, cmd)

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	return zone.Scan(m.view())
}

func (m Model) view() string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.toggle.View(),
		m.list.View(),
	)
}
