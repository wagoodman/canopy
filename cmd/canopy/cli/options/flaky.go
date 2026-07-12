package options

import (
	"fmt"
	"regexp"
	"time"

	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*Flaky)(nil)
	_ fangs.PostLoader = (*Flaky)(nil)
)

// FlakyOutputFormat represents the output format for flaky test results.
type FlakyOutputFormat string

const (
	// FlakyOutputTable outputs results as a human-readable table.
	FlakyOutputTable FlakyOutputFormat = "table"
	// FlakyOutputJSON outputs results as JSON for machine consumption.
	FlakyOutputJSON FlakyOutputFormat = "json"
)

// Flaky configures flaky test detection analysis.
type Flaky struct {
	// Threshold is the minimum flaky score (0.0-1.0) required to report a test.
	// Tests with scores below this are not reported as flaky.
	Threshold float64 `yaml:"threshold" json:"threshold" mapstructure:"threshold"`

	// MinRuns is the minimum number of runs required to consider a test for flakiness.
	// Tests with fewer runs are excluded from analysis.
	MinRuns int `yaml:"min-runs" json:"min-runs" mapstructure:"min-runs"`

	// WindowStr is the string representation of the time window (e.g., "168h", "7d", "0" for all).
	WindowStr string `yaml:"window" json:"window" mapstructure:"window"`

	// Window is the parsed duration from WindowStr.
	Window time.Duration `yaml:"-" json:"-" mapstructure:"-"`

	// Output is the output format (table, json).
	Output string `yaml:"output" json:"output" mapstructure:"output"`

	// Scoping options

	// Specifiers are package path patterns to include in the analysis.
	// These are glob patterns matched against stored package paths.
	Specifiers []string `yaml:"packages" json:"packages" mapstructure:"packages"`

	// ExcludePatterns are glob patterns of package paths to exclude from analysis.
	ExcludePatterns []string `yaml:"exclude" json:"exclude" mapstructure:"exclude"`

	// TestStr is a regex pattern to filter test function names.
	TestStr string `yaml:"test" json:"test" mapstructure:"test"`

	// Test is the compiled regex from TestStr.
	Test *regexp.Regexp `yaml:"-" json:"-" mapstructure:"-"`

	// Sessions are session UUIDs or keywords ("last") to limit analysis to.
	Sessions []string `yaml:"sessions" json:"sessions" mapstructure:"sessions"`
}

// DefaultFlaky returns the default flaky detection options.
func DefaultFlaky() Flaky {
	return Flaky{
		Threshold: 0,
		MinRuns:   2,
		WindowStr: "",
		Window:    0,
		Output:    string(FlakyOutputTable),
	}
}

// AddFlags registers flaky detection flags with the flag set.
func (o *Flaky) AddFlags(flags fangs.FlagSet) {
	flags.Float64VarP(&o.Threshold, "threshold", "t", "minimum flaky score (0.0-1.0) to report a test")
	flags.IntVarP(&o.MinRuns, "min-runs", "m", "minimum number of runs required for flaky analysis")
	flags.StringVarP(&o.WindowStr, "window", "w", "time window for analysis (e.g., 168h for 7 days, empty for all)")
	flags.StringVarP(&o.Output, "output", "o", "output format (table, json)")

	// scoping flags
	flags.StringArrayVarP(&o.ExcludePatterns, "exclude", "e", "glob patterns of package paths to exclude")
	flags.StringVarP(&o.TestStr, "test", "", "regex pattern to filter test function names")
	flags.StringArrayVarP(&o.Sessions, "session", "s", "session UUID or 'last' to limit analysis (can be repeated)")
}

// PostLoad parses the window string into a duration and compiles the test pattern.
func (o *Flaky) PostLoad() error {
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

	// validate output format
	switch FlakyOutputFormat(o.Output) {
	case FlakyOutputTable, FlakyOutputJSON:
		// valid
	default:
		return fmt.Errorf("invalid output format %q (valid: table, json)", o.Output)
	}

	return nil
}
