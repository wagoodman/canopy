package style

import "github.com/charmbracelet/lipgloss"

type Dot struct {
	CheckTitle lipgloss.Style
	XTitle     lipgloss.Style

	RunningTitle lipgloss.Style
	SuccessTitle lipgloss.Style
	FailureTitle lipgloss.Style
	SkipTitle    lipgloss.Style
	Aux          lipgloss.Style
	Dot          lipgloss.Style
	Nested       lipgloss.Style
	Title        lipgloss.Style
}

func NewDot(color bool) Dot {
	if color {
		return Dot{
			CheckTitle:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			XTitle:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			RunningTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			SkipTitle:    lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			SuccessTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			FailureTitle: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			Aux:          lipgloss.NewStyle().Faint(true),
			Dot:          lipgloss.NewStyle().Faint(true), // 12: hi blue
			Nested:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			Title:        lipgloss.NewStyle().Bold(true),
		}
	}

	return Dot{
		RunningTitle: lipgloss.NewStyle(),
		SkipTitle:    lipgloss.NewStyle(),
		SuccessTitle: lipgloss.NewStyle(),
		FailureTitle: lipgloss.NewStyle(),
		Aux:          lipgloss.NewStyle(),
		Dot:          lipgloss.NewStyle(),
		Nested:       lipgloss.NewStyle(),
		Title:        lipgloss.NewStyle(),
	}
}
