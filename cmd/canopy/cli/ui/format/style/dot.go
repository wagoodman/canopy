package style

import "github.com/charmbracelet/lipgloss"

// Dot holds styling configuration for dot-style test output formatting.
type Dot struct {
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

	// Dot is the style for test progress dots.
	Dot lipgloss.Style

	// Nested is the style for nested test names.
	Nested lipgloss.Style

	// Title is the style for test titles.
	Title lipgloss.Style
}

// NewDot creates a new dot style configuration with optional color support.
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
