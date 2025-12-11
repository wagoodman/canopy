package fragment

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// TestCounts renders test pass/fail/skip counts with appropriate styling.
type TestCounts struct {
	// FailedCountStyle is applied to the failed test count.
	FailedCountStyle lipgloss.Style

	// PassedCountStyle is applied to the passed test count.
	PassedCountStyle lipgloss.Style

	// SkippedCountStyle is applied to the skipped test count.
	SkippedCountStyle lipgloss.Style

	// DefaultStyle is used for counts when not highlighted.
	DefaultStyle lipgloss.Style

	// AuxStyle is used for separators and auxiliary text.
	AuxStyle lipgloss.Style
}

// NewTestCounts creates a TestCounts with default color styling.
func NewTestCounts() TestCounts {
	return TestCounts{
		FailedCountStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		PassedCountStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		SkippedCountStyle: lipgloss.NewStyle().Faint(true),
		DefaultStyle:      lipgloss.NewStyle(),
		AuxStyle:          lipgloss.NewStyle().Faint(true),
	}
}

// View renders test counts in the format "X passed / Y failed / Z skipped".
// Failed tests are shown first if present. Passed tests use default styling
// when failures exist, otherwise use the passed style.
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

// insertBetween inserts str between each element of slice. Returns a new slice
// with interleaved values.
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
