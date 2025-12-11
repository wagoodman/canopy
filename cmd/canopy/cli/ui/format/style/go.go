package style

import "github.com/charmbracelet/lipgloss"

// Go holds styling configuration for Go test output formatting.
type Go struct {
	// Bold is the style for bold text.
	Bold lipgloss.Style

	// Success is the style for successful test output.
	Success lipgloss.Style

	// Failed is the style for failed test output.
	Failed lipgloss.Style

	// Running is the style for currently running tests.
	Running lipgloss.Style

	// Skipped is the style for skipped tests.
	Skipped lipgloss.Style

	// Aux is the style for auxiliary information (timestamps, metadata).
	Aux lipgloss.Style

	// Info is the style for informational messages.
	Info lipgloss.Style

	// Waiting is the style for waiting/pending states.
	Waiting lipgloss.Style

	// PanicGroup is the style for panic output grouping markers.
	PanicGroup lipgloss.Style

	// PanicTitle is the style for panic titles/headers.
	PanicTitle lipgloss.Style

	// PanicFunc is the style for function names in panic output.
	PanicFunc lipgloss.Style

	// PanicFuncAux is the style for auxiliary function information in panics.
	PanicFuncAux lipgloss.Style

	// PanicFile is the style for file names in panic output.
	PanicFile lipgloss.Style

	// PanicFileAux is the style for auxiliary file information in panics.
	PanicFileAux lipgloss.Style
}

// NewGo creates a new Go style configuration with optional color support.
func NewGo(color bool) Go {
	if color {
		return Go{
			Bold:    lipgloss.NewStyle().Bold(true),
			Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			Failed:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			Running: lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
			Skipped: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			Aux:     lipgloss.NewStyle().Faint(true),
			Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("13")),
			Waiting: lipgloss.NewStyle().Italic(true),

			PanicGroup: lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			// red background with white text
			PanicTitle:   lipgloss.NewStyle().Background(lipgloss.Color("9")).Foreground(lipgloss.Color("#ffffff")).Bold(true),
			PanicFunc:    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			PanicFuncAux: lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Faint(true),
			PanicFile:    lipgloss.NewStyle().Bold(true),
			PanicFileAux: lipgloss.NewStyle().Faint(true),

			// Border(
			//	lipgloss.Border{
			//		Left: "▒", // ░▒ ▍
			//	}, false, false, false, true).
			// PaddingLeft(1).
			// BorderForeground(lipgloss.Color("9")),
		}
	}
	return Go{
		Bold:         lipgloss.NewStyle(),
		Success:      lipgloss.NewStyle(),
		Running:      lipgloss.NewStyle(),
		Failed:       lipgloss.NewStyle(),
		Skipped:      lipgloss.NewStyle(),
		Aux:          lipgloss.NewStyle(),
		Info:         lipgloss.NewStyle(),
		Waiting:      lipgloss.NewStyle(),
		PanicGroup:   lipgloss.NewStyle(),
		PanicTitle:   lipgloss.NewStyle(),
		PanicFunc:    lipgloss.NewStyle(),
		PanicFuncAux: lipgloss.NewStyle(),
		PanicFile:    lipgloss.NewStyle(),
		PanicFileAux: lipgloss.NewStyle(),
	}
}
