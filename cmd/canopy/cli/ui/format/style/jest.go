package style

import "github.com/charmbracelet/lipgloss"

type Jest struct {
	CheckTitle lipgloss.Style
	XTitle     lipgloss.Style

	RunningTitle lipgloss.Style
	SuccessTitle lipgloss.Style
	FailureTitle lipgloss.Style
	SkipTitle    lipgloss.Style
	Aux          lipgloss.Style
	Title        lipgloss.Style
}

func NewJest(color bool) Jest {
	if color {
		return Jest{
			CheckTitle:   lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true),
			XTitle:       lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
			RunningTitle: lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")).Bold(true),
			SkipTitle:    lipgloss.NewStyle().Background(lipgloss.Color("246")).Foreground(lipgloss.Color("0")).Bold(true),
			SuccessTitle: lipgloss.NewStyle().Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true),
			FailureTitle: lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Bold(true),
			Aux:          lipgloss.NewStyle().Faint(true),
			Title:        lipgloss.NewStyle().Bold(true),
		}
	}

	return Jest{
		RunningTitle: lipgloss.NewStyle(),
		SkipTitle:    lipgloss.NewStyle(),
		SuccessTitle: lipgloss.NewStyle(),
		FailureTitle: lipgloss.NewStyle(),
		Aux:          lipgloss.NewStyle(),
		Title:        lipgloss.NewStyle(),
	}
}
