package style

import "github.com/charmbracelet/lipgloss"

type GoStd struct {
	Bold         lipgloss.Style
	Success      lipgloss.Style
	Failed       lipgloss.Style
	Running      lipgloss.Style
	Skipped      lipgloss.Style
	Aux          lipgloss.Style
	Info         lipgloss.Style
	PanicGroup   lipgloss.Style
	PanicTitle   lipgloss.Style
	PanicFunc    lipgloss.Style
	PanicFuncAux lipgloss.Style
	PanicFile    lipgloss.Style
	PanicFileAux lipgloss.Style
}

func NewGoStd(color bool) GoStd {
	if color {
		return GoStd{
			Bold:    lipgloss.NewStyle().Bold(true),
			Success: lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
			Failed:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
			Running: lipgloss.NewStyle().Foreground(lipgloss.Color("14")),
			Skipped: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
			Aux:     lipgloss.NewStyle().Faint(true),
			Info:    lipgloss.NewStyle().Foreground(lipgloss.Color("13")),

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
	return GoStd{
		Bold:         lipgloss.NewStyle(),
		Success:      lipgloss.NewStyle(),
		Running:      lipgloss.NewStyle(),
		Failed:       lipgloss.NewStyle(),
		Skipped:      lipgloss.NewStyle(),
		Aux:          lipgloss.NewStyle(),
		Info:         lipgloss.NewStyle(),
		PanicGroup:   lipgloss.NewStyle(),
		PanicTitle:   lipgloss.NewStyle(),
		PanicFunc:    lipgloss.NewStyle(),
		PanicFuncAux: lipgloss.NewStyle(),
		PanicFile:    lipgloss.NewStyle(),
		PanicFileAux: lipgloss.NewStyle(),
	}
}
