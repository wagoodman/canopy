package toggle

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

const (
	openCircle   = "○"
	closedCircle = "●"
)

type Model struct {
	toggles  []Toggle
	Vertical bool
}

func NewModel(toggles ...Toggle) Model {
	return Model{
		toggles: toggles,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		for i := range m.toggles {
			toggle := &m.toggles[i]
			if key.Matches(msg, toggle.Binding) {
				toggle.Press()
			}
		}
	}
	return m, nil
}

func (m Model) View() string {
	// TODO: styles...
	var outputs []string
	for _, toggle := range m.toggles {
		t := toggle
		if t.Engaged() {
			st := t.EngagedStyle
			if st == nil {
				st = &engagedStyle
			}
			outputs = append(outputs, st.Render(closedCircle)+" "+t.engagedDesc)
		} else {

			st := t.DisengagedStyle
			if st == nil {
				st = &disengagedStyle
			}
			outputs = append(outputs, st.Render(openCircle)+" "+t.disengagedDesc)
		}
	}
	return strings.Join(outputs, "  ")
}
