// Package outputpane provides a viewport-based UI component for displaying test
// output with syntax highlighting and test statistics.
package outputpane

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Config holds configuration options for the output pane's appearance and sizing.
type Config struct {
	// WidthRatio is the width ratio of the viewport to the terminal width. This is useful
	// for sidebars and other UI elements that don't take up the full width of
	// the terminal.
	WidthRatio float64

	// FailedCountStyle is applied to the failed test count in the summary.
	FailedCountStyle lipgloss.Style

	// PassedCountStyle is applied to the passed test count in the summary.
	PassedCountStyle lipgloss.Style

	// SkippedCountStyle is applied to the skipped test count in the summary.
	SkippedCountStyle lipgloss.Style

	// SummaryLineStyle styles the summary line text.
	SummaryLineStyle lipgloss.Style

	// BorderSummaryStyle styles the border around the summary line.
	BorderSummaryStyle lipgloss.Style
}

// defaultOptions returns Config with default styling and sizing.
func defaultOptions() Config {
	baseSummaryStyle := lipgloss.NewStyle()

	return Config{
		WidthRatio: 1.0,

		// counts
		//PassedCountStyle:  summaryBG.Foreground(lipgloss.Color("10")),
		PassedCountStyle:  baseSummaryStyle.Foreground(lipgloss.Color("10")),
		FailedCountStyle:  baseSummaryStyle.Foreground(lipgloss.Color("9")),
		SkippedCountStyle: baseSummaryStyle.Faint(true),
		SummaryLineStyle:  baseSummaryStyle.Faint(true),

		BorderSummaryStyle: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			// Border(lipgloss.NormalBorder(), false, false, true, false).
			// BorderBottom(true).
			// BorderForeground(lipgloss.Color("#666666")),
			BorderForeground(lipgloss.Color("#FFFFFF")),
	}
}

// Option is a functional option for configuring the output pane.
type Option func(*Config) error

// WithWidthRatio sets the width ratio for the output pane. Must be > 0 and <= 1.
func WithWidthRatio(ratio float64) Option {
	return func(c *Config) error {
		if ratio > 1 || ratio <= 0 {
			return fmt.Errorf("invalid width ratio %f, must be > 0 or <= 1", ratio)
		}
		c.WidthRatio = ratio
		return nil
	}
}

// apply creates a Config by applying all options to the default configuration.
func apply(options ...Option) (Config, error) {
	opts := defaultOptions()
	for _, o := range options {
		if err := o(&opts); err != nil {
			return Config{}, err
		}
	}
	return opts, nil
}
