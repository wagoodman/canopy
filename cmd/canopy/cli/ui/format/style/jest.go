package style

import "github.com/charmbracelet/lipgloss"

// Jest holds styling configuration for Jest-style test output formatting.
type Jest struct {
	// CheckTitle is the style for success check marks.
	CheckTitle lipgloss.Style

	// XTitle is the style for failure X marks.
	XTitle lipgloss.Style

	// RunningTitle is the style for running test titles.
	RunningTitle lipgloss.Style

	// SuccessTitle is the style for successful test titles.
	SuccessTitle lipgloss.Style

	// FailureTitle is the style for failed test titles.
	FailureTitle lipgloss.Style

	// SkipTitle is the style for skipped test titles.
	SkipTitle lipgloss.Style

	// Aux is the style for auxiliary information.
	Aux lipgloss.Style

	// Title is the style for test titles.
	Title lipgloss.Style
}

// NewJest creates a new Jest style configuration with optional color support.
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
