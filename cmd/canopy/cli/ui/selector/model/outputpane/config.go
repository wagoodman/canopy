package outputpane

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	// The width ratio of the viewport to the terminal width. This is useful
	// for sidebars and other UI elements that don't take up the full width of
	// the terminal.
	WidthRatio float64

	// Summary Counts
	FailedCountStyle  lipgloss.Style
	PassedCountStyle  lipgloss.Style
	SkippedCountStyle lipgloss.Style
	SummaryLineStyle  lipgloss.Style

	BorderSummaryStyle lipgloss.Style
}

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

type Option func(*Config) error

func WithWidthRatio(ratio float64) Option {
	return func(c *Config) error {
		if ratio > 1 || ratio <= 0 {
			return fmt.Errorf("invalid width ratio %f, must be > 0 or <= 1", ratio)
		}
		c.WidthRatio = ratio
		return nil
	}
}

func apply(options ...Option) (Config, error) {
	opts := defaultOptions()
	for _, o := range options {
		if err := o(&opts); err != nil {
			return Config{}, err
		}
	}
	return opts, nil
}
