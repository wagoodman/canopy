package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*GoTest)(nil)

// GoTest mimics the flags for `go test` as much as is feasible. There are additional flags that are not supported directly
// by `go test` (e.g. coverage, no cache, etc.). The RenderedFlags field ultimately holds the rendered flags to be passed to the `go test` command.
type GoTest struct {
	// Bench runs benchmarks matching the regular expression.
	Bench string `yaml:"bench" json:"bench" mapstructure:"bench"`
	// Count runs tests and benchmarks n times (default 1).
	Count int `yaml:"count" json:"count" mapstructure:"count"`
	// CoverMode sets the mode for coverage analysis (set, count, or atomic).
	CoverMode string `yaml:"covermode" json:"covermode" mapstructure:"covermode"`
	// CoverPkg applies coverage analysis to packages matching the patterns.
	CoverPkg string `yaml:"coverpkg" json:"coverpkg" mapstructure:"coverpkg"`
	// Exec runs the test binary using the specified program.
	Exec string `yaml:"exec" json:"exec" mapstructure:"exec"`
	// NoCache disables caching of test results (custom flag, not in standard go test).
	NoCache bool `yaml:"no-cache" json:"no-cache" mapstructure:"no-cache"` // custom flag
	// Parallel allows parallel execution of test functions that call t.Parallel.
	Parallel int `yaml:"parallel" json:"parallel" mapstructure:"parallel"`
	// Run runs only tests and examples matching the regular expression.
	Run string `yaml:"run" json:"run" mapstructure:"run"`
	// Timeout sets a timeout for each test.
	Timeout string `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	// Vet configures go vet ('off', 'all', or a comma-separated list of checks).
	Vet string `yaml:"vet" json:"vet" mapstructure:"vet"`

	// RenderedFlags contains the final flags to be passed to the go test command after post-load processing.
	RenderedFlags []string `yaml:"-" json:"-" mapstructure:"-"`
	// IgnoreRenderingFlags contains flag names that should not be included in RenderedFlags.
	IgnoreRenderingFlags []string `yaml:"-" json:"-" mapstructure:"-"`
	tracker              *xflagset.Decorator
	NamedFlagSet         *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultGoTest returns test options with Go's default test configuration.
func DefaultGoTest() GoTest {
	return GoTest{}
}

// PostLoad renders all test flags into a format suitable for passing to `go test`, excluding ignored flags.
func (o *GoTest) PostLoad() error {
	o.RenderedFlags = o.tracker.RenderFlags(o.IgnoreRenderingFlags...)
	return nil
}

// AddFlags registers all go test-related flags with the flag set.
func (o *GoTest) AddFlags(fangFlags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(fangFlags, o.NamedFlagSet.FlagSet("Test"))
	flags := o.tracker

	flags.StringVarP(&o.Exec, "exec", "", "run the test binary using xprog")
	flags.StringVarP(&o.Bench, "bench", "", "run benchmarks matching the regular expression")
	flags.StringVarP(&o.Vet, "vet", "", "configure go vet ('off', 'all', or a comma-separated list of checks)")
	flags.IntVarP(&o.Count, "count", "", "run tests and benchmarks n times")
	flags.StringVarP(&o.Timeout, "timeout", "", "timeout for each test")
	flags.StringVarP(&o.Run, "run", "", "run only those tests and examples matching the regular expression")
	flags.IntVarP(&o.Parallel, "parallel", "", "allow parallel execution of test functions that call t.Parallel")
	flags.StringVarP(&o.CoverMode, "covermode", "", "set the mode for coverage analysis (options: set, count, atomic)")
	flags.StringVarP(&o.CoverPkg, "coverpkg", "", "apply coverage analysis to each package matching the patterns")
}
