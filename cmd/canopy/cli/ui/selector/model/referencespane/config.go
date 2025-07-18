package referencespane

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

type Config struct {
	// The width ratio of the viewport to the terminal width. This is useful
	// for sidebars and other UI elements that don't take up the full width of
	// the terminal.
	WidthRatio float64

	// Reference list / selection
	// AllRefStyle     lipgloss.Style
	// RunningRefStyle lipgloss.Style
	// FailedRefStyle  lipgloss.Style
	// PassedRefStyle  lipgloss.Style
	// SkippedRefStyle lipgloss.Style

	ShowFailedOnly bool

	// Summary Counts
	FailedCountStyle  lipgloss.Style
	PassedCountStyle  lipgloss.Style
	SkippedCountStyle lipgloss.Style
	SummaryLineStyle  lipgloss.Style

	// Summary Conclusion
	RunningStyle          lipgloss.Style
	FailedConclusionStyle lipgloss.Style
	PassedConclusionStyle lipgloss.Style

	BorderSummaryStyle lipgloss.Style
}

func defaultOptions() Config {
	baseSummaryStyle := lipgloss.NewStyle()
	bdr := lipgloss.NormalBorder()
	// bdr.Bottom = "▔"
	return Config{
		WidthRatio: 1.0, // full width
		BorderSummaryStyle: lipgloss.NewStyle().
			Border(bdr, false, false, true, false).
			// Border(lipgloss.NormalBorder(), false, false, true, false).
			// BorderBottom(true).
			// BorderForeground(lipgloss.Color("#666666")),
			BorderForeground(lipgloss.Color("#FFFFFF")),

		// ref list
		//AllRefStyle:     lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("246")),
		//RunningRefStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("246")),
		//FailedRefStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		//PassedRefStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		////PassedRefStyle:  lipgloss.NewStyle(),
		//SkippedRefStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("11")),

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

func WithShowFailedOnly(show bool) Option {
	return func(c *Config) error {
		c.ShowFailedOnly = show
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
