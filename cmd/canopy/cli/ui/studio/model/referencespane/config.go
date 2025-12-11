// Package referencespane provides a navigable list UI component for browsing and
// selecting test references (packages, functions, and test cases) with filtering
// and multi-selection support.
package referencespane

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Config holds configuration options for the references pane's appearance and behavior.
type Config struct {
	// WidthRatio is the width ratio of the viewport to the terminal width. This is useful
	// for sidebars and other UI elements that don't take up the full width of
	// the terminal.
	WidthRatio float64

	// ShowFailedOnly when true displays only failed tests initially.
	ShowFailedOnly bool

	// FailedCountStyle is applied to the failed test count in the summary.
	FailedCountStyle lipgloss.Style

	// PassedCountStyle is applied to the passed test count in the summary.
	PassedCountStyle lipgloss.Style

	// SkippedCountStyle is applied to the skipped test count in the summary.
	SkippedCountStyle lipgloss.Style

	// SummaryLineStyle styles the summary line text.
	SummaryLineStyle lipgloss.Style

	// RunningStyle styles the summary when tests are still running.
	RunningStyle lipgloss.Style

	// FailedConclusionStyle styles the summary when tests have failed.
	FailedConclusionStyle lipgloss.Style

	// PassedConclusionStyle styles the summary when all tests have passed.
	PassedConclusionStyle lipgloss.Style

	// BorderSummaryStyle styles the border around the summary line.
	BorderSummaryStyle lipgloss.Style
}

// defaultOptions returns Config with default styling and sizing.
func defaultOptions() Config {
	baseSummaryStyle := lipgloss.NewStyle()
	bdr := lipgloss.NormalBorder()
	// bdr.Bottom = "▔"
	return Config{
		BorderSummaryStyle: lipgloss.NewStyle().
			Border(bdr, false, false, true, false).
			// Border(lipgloss.NormalBorder(), false, false, true, false).
			// BorderBottom(true).
			// BorderForeground(lipgloss.Color("#666666")),
			BorderForeground(lipgloss.Color("#FFFFFF")),

		// counts
		//PassedCountStyle:  summaryBG.Foreground(lipgloss.Color("10")),
		PassedCountStyle:  baseSummaryStyle,
		FailedCountStyle:  baseSummaryStyle.Foreground(lipgloss.Color("9")),
		SkippedCountStyle: baseSummaryStyle.Foreground(lipgloss.Color("11")),
		SummaryLineStyle:  baseSummaryStyle,

		// conclusion
		RunningStyle:          baseSummaryStyle.Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0")).Bold(true),
		PassedConclusionStyle: baseSummaryStyle.Background(lipgloss.Color("10")).Foreground(lipgloss.Color("0")).Bold(true),
		FailedConclusionStyle: baseSummaryStyle.Background(lipgloss.Color("9")).Foreground(lipgloss.Color("15")).Bold(true),
		//FailedConclusionStyle: baseSummaryStyle.Background(lipgloss.Color("9")).Foreground(lipgloss.Color("0")).Bold(true),
	}
}

// Option is a functional option for configuring the references pane.
type Option func(*Config) error

// WithWidthRatio sets the width ratio for the references pane. Must be > 0 and <= 1.
func WithWidthRatio(ratio float64) Option {
	return func(c *Config) error {
		if ratio > 1 || ratio <= 0 {
			return fmt.Errorf("invalid width ratio %f, must be > 0 or <= 1", ratio)
		}
		c.WidthRatio = ratio
		return nil
	}
}

// WithShowFailedOnly configures whether to show only failed tests initially.
func WithShowFailedOnly(show bool) Option {
	return func(c *Config) error {
		c.ShowFailedOnly = show
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
