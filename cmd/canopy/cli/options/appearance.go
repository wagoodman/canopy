package options

import (
	"os"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/cienv"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/fangs"
)

var (
	_ fangs.PostLoader = (*Appearance)(nil)
)

// Appearance configures visual presentation options for test output including color support and package name formatting.
type Appearance struct {
	// CombineMultipleRuns controls whether to show a single summary for multiple test run sessions.
	CombineMultipleRuns bool `yaml:"-" json:"-" mapstructure:"-"`
	// NoColor disables all colorized output in test results and UI.
	NoColor bool `yaml:"no-color" json:"no-color" mapstructure:"no-color"`
	// ShowPackagesWithNoTests controls whether to display packages that have no test files.
	ShowPackagesWithNoTests bool `yaml:"show-packages-with-no-tests" json:"show-packages-with-no-tests" mapstructure:"show-packages-with-no-tests"`
	// UseShortNames strips the module prefix from package names in output for brevity.
	UseShortNames bool `yaml:"use-short-names" json:"use-short-names" mapstructure:"use-short-names"`
	// CIGrouping configures collapsible output grouping for CI environments.
	CIGrouping CIGrouping `yaml:"ci-grouping" json:"ci-grouping" mapstructure:"ci-grouping"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// CIGrouping configures collapsible output groups for CI environments like GitHub Actions.
// When enabled and running in a supported CI, package output is wrapped in collapsible groups.
type CIGrouping struct {
	// Disabled explicitly disables CI grouping even in CI environments.
	Disabled bool `yaml:"disabled" json:"disabled" mapstructure:"disabled"`
	// GroupPassedPackages controls whether passed package output is grouped (collapsed by default).
	GroupPassedPackages bool `yaml:"group-passed-packages" json:"group-passed-packages" mapstructure:"group-passed-packages"`
	// GroupFailedPackages controls whether failed package output is grouped.
	GroupFailedPackages bool `yaml:"group-failed-packages" json:"group-failed-packages" mapstructure:"group-failed-packages"`
}

// DefaultCIGrouping returns the default CI grouping configuration.
// By default, grouping is auto-detected from the CI environment, passed packages are grouped,
// and failed packages are not grouped (so failures are immediately visible).
func DefaultCIGrouping() CIGrouping {
	return CIGrouping{
		Disabled:            false,
		GroupPassedPackages: true,
		GroupFailedPackages: false,
	}
}

// ToGroupConfig converts CIGrouping to a cienv.GroupConfig for use with the grouping writer.
func (c CIGrouping) ToGroupConfig() cienv.GroupConfig {
	var enabled *bool
	if c.Disabled {
		f := false
		enabled = &f
	}
	return cienv.GroupConfig{
		Enabled:             enabled,
		GroupPassedPackages: c.GroupPassedPackages,
		GroupFailedPackages: c.GroupFailedPackages,
	}
}

// DefaultAppearance returns appearance options with sensible defaults (color enabled, short names enabled).
func DefaultAppearance() Appearance {
	return Appearance{
		NoColor:                 false,
		ShowPackagesWithNoTests: false,
		UseShortNames:           true,
		CIGrouping:              DefaultCIGrouping(),
	}
}

// AddFlags registers the appearance flags with the flag set.
func (o *Appearance) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Format"))
	flags = o.tracker

	flags.BoolVarP(
		&o.NoColor,
		"no-color", "",
		"disable all colorized output (can be overridden by the NO_COLOR environment variable as well)",
	)

	flags.BoolVarP(
		&o.CIGrouping.Disabled,
		"no-ci-grouping", "",
		"disable collapsible output groups in CI environments (GitHub Actions, Azure Pipelines)",
	)
}

// PostLoad applies environment variable overrides (e.g., NO_COLOR) to the configuration.
func (o *Appearance) PostLoad() error {
	overrideNoColorFromEnv(&o.NoColor)
	return nil
}

// overrideNoColorFromEnv checks the NO_COLOR environment variable and disables color if set to a truthy value.
func overrideNoColorFromEnv(opt *bool) {
	// override no-color with NO_COLOR env var
	noColorEnvVar := strings.TrimSpace(os.Getenv("NO_COLOR"))
	switch strings.ToLower(noColorEnvVar) {
	case "true", "1", "t":
		log.WithFields("NO_COLOR", noColorEnvVar).Trace("disabling colorized output")
		*opt = true
	}
}
