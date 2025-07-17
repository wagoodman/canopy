package fragment

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TestCounts struct {
	FailedCountStyle  lipgloss.Style
	PassedCountStyle  lipgloss.Style
	SkippedCountStyle lipgloss.Style
	DefaultStyle      lipgloss.Style
	AuxStyle          lipgloss.Style
}

func NewTestCounts() TestCounts {
	return TestCounts{
		FailedCountStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		PassedCountStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		SkippedCountStyle: lipgloss.NewStyle().Faint(true),
		DefaultStyle:      lipgloss.NewStyle(),
		AuxStyle:          lipgloss.NewStyle().Faint(true),
	}
}

func (tc TestCounts) View(passed, failed, skipped int) string {
	var sections []string

	if failed > 0 {
		sections = append(sections, tc.FailedCountStyle.Render(
			fmt.Sprintf("%d failed", failed),
		))
	}

	if passed > 0 {
		sty := tc.PassedCountStyle
		if failed > 0 {
			sty = tc.DefaultStyle
		}
		sections = append([]string{sty.Render(
			fmt.Sprintf("%d passed", passed),
		)}, sections...)
	}

	if skipped > 0 {
		sections = append(sections, tc.SkippedCountStyle.Render(
			fmt.Sprintf("%d skipped", skipped),
		))
	}

	// insert joiner between all elements
	sections = insertBetween(sections, tc.AuxStyle.Render(" / "))

	return strings.Join(sections, "")
}

func insertBetween(slice []string, str string) []string {
	if len(slice) == 0 {
		return slice
	}

	result := make([]string, 0, len(slice)*2-1)

	for i, s := range slice {
		result = append(result, s)
		if i < len(slice)-1 {
			result = append(result, str)
		}
	}

	return result
}
