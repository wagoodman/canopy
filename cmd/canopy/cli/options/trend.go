package options

import (
	"fmt"
	"regexp"
	"time"

	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*Trend)(nil)
	_ fangs.PostLoader = (*Trend)(nil)
)

// TrendOutputFormat is the output format shared by trend subcommands.
type TrendOutputFormat string

const (
	// TrendOutputTable outputs a human-readable table.
	TrendOutputTable TrendOutputFormat = "table"
	// TrendOutputJSON outputs JSON for machine consumption.
	TrendOutputJSON TrendOutputFormat = "json"
)

// Trend holds the scoping and output options shared by the trend subcommands
// (duration, failures, count). Flaky predates this and keeps its own options.
type Trend struct {
	// Last limits analysis to the most recent N runs (0 = no limit).
	Last int `yaml:"last" json:"last" mapstructure:"last"`

	// WindowStr is the raw time window (e.g. "168h", empty/"0" for all).
	WindowStr string `yaml:"window" json:"window" mapstructure:"window"`
	// Window is the parsed WindowStr.
	Window time.Duration `yaml:"-" json:"-" mapstructure:"-"`

	// Output is the output format (table, json).
	Output string `yaml:"output" json:"output" mapstructure:"output"`

	// Specifiers are package glob patterns to include (empty = all).
	Specifiers []string `yaml:"packages" json:"packages" mapstructure:"packages"`
	// ExcludePatterns are package globs to exclude.
	ExcludePatterns []string `yaml:"exclude" json:"exclude" mapstructure:"exclude"`

	// TestStr is a regex filtering test function names.
	TestStr string `yaml:"test" json:"test" mapstructure:"test"`
	// Test is the compiled TestStr.
	Test *regexp.Regexp `yaml:"-" json:"-" mapstructure:"-"`

	// Sessions are session UUIDs or "last" to limit analysis to.
	Sessions []string `yaml:"sessions" json:"sessions" mapstructure:"sessions"`
}

// DefaultTrend returns the default shared trend options.
func DefaultTrend() Trend {
	return Trend{
		Last:   10,
		Output: string(TrendOutputTable),
	}
}

// AddFlags registers the shared trend flags.
func (o *Trend) AddFlags(flags fangs.FlagSet) {
	flags.IntVarP(&o.Last, "last", "n", "analyze the most recent N runs (0 for all)")
	flags.StringVarP(&o.WindowStr, "window", "w", "time window for analysis (e.g. 168h for 7 days, empty for all)")
	flags.StringVarP(&o.Output, "output", "o", "output format (table, json)")
	flags.StringArrayVarP(&o.ExcludePatterns, "exclude", "e", "glob patterns of package paths to exclude")
	flags.StringVarP(&o.TestStr, "test", "", "regex pattern to filter test function names")
	flags.StringArrayVarP(&o.Sessions, "session", "s", "session UUID or 'last' to limit analysis (can be repeated)")
}

// PostLoad parses the window string, compiles the test pattern, and validates output.
func (o *Trend) PostLoad() error {
	if o.WindowStr == "" || o.WindowStr == "0" {
		o.Window = 0
	} else {
		d, err := ParseDuration(o.WindowStr)
		if err != nil {
			return fmt.Errorf("invalid window duration %q: %w", o.WindowStr, err)
		}
		o.Window = d
	}

	if o.TestStr != "" {
		re, err := regexp.Compile(o.TestStr)
		if err != nil {
			return fmt.Errorf("invalid test pattern %q: %w", o.TestStr, err)
		}
		o.Test = re
	}

	switch TrendOutputFormat(o.Output) {
	case TrendOutputTable, TrendOutputJSON:
	default:
		return fmt.Errorf("invalid output format %q (valid: table, json)", o.Output)
	}

	return nil
}
