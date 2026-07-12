package options

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ci"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest/output"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/fangs"
)

var (
	_ fangs.PostLoader = (*Appearance)(nil)
)

const (
	onColor   = "on"
	offColor  = "off"
	autoColor = "auto"
)

// Appearance configures visual presentation options for test output including color support and package name formatting.
type Appearance struct {
	// CombineMultipleRuns controls whether to show a single summary for multiple test run sessions.
	CombineMultipleRuns bool `yaml:"-" json:"-" mapstructure:"-"`
	// Color controls colorized output: "auto" (detect terminal/CI), "on" (force color), "off" (disable color).
	Color string `yaml:"color" json:"color" mapstructure:"color"`
	// ShowPackagesWithNoTests controls whether to display packages that have no test files.
	ShowPackagesWithNoTests bool `yaml:"show-packages-with-no-tests" json:"show-packages-with-no-tests" mapstructure:"show-packages-with-no-tests"`
	// UseShortNames strips the module prefix from package names in output for brevity.
	UseShortNames bool `yaml:"use-short-names" json:"use-short-names" mapstructure:"use-short-names"`
	// ExecutionMarkers controls visibility of test execution state markers (=== RUN, === PAUSE, === CONT).
	// Valid values: "none" (hide all), "all" (show all), "parallel-only" (show only PAUSE/CONT).
	ExecutionMarkers string `yaml:"execution-markers" json:"execution-markers" mapstructure:"execution-markers"`
	// Grouping configures collapsible output groups for CI environments.
	Grouping Grouping `yaml:"grouping" json:"grouping" mapstructure:"grouping"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultAppearance returns appearance options with sensible defaults (auto color detection, short names enabled).
func DefaultAppearance() Appearance {
	return Appearance{
		Color:                   autoColor,
		ShowPackagesWithNoTests: false,
		UseShortNames:           true,
		ExecutionMarkers:        output.ExecutionMarkersParallelOnly,
		Grouping:                DefaultGrouping(),
	}
}

func (o *Appearance) DescribeFields(descriptions fangs.FieldDescriptionSet) {
	descriptions.Add(&o.CombineMultipleRuns, "whether to combine multiple test runs into a single summary")
	descriptions.Add(&o.ExecutionMarkers, "visibility of test execution markers (=== RUN/PAUSE/CONT): none, all, parallel-only")
	descriptions.Add(&o.Color, "color output mode: auto, on, off (respects NO_COLOR and FORCE_COLOR env vars)")
	descriptions.Add(&o.ShowPackagesWithNoTests, "whether to show packages that have no test files")
	descriptions.Add(&o.UseShortNames, "whether to strip module path prefixes from package names in output")
}

// AddFlags registers the appearance flags with the flag set.
func (o *Appearance) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Format"))
	flags = o.tracker

	flags.StringVarP(
		&o.Color,
		"color", "",
		"color output mode: auto, on, off (respects NO_COLOR and FORCE_COLOR env vars)",
	)
}

// PostLoad applies environment variable overrides and CI detection for color configuration.
func (o *Appearance) PostLoad() error {
	o.Color = resolveColor(o.Color)
	return nil
}

// resolveColor determines the final color setting based on environment variables and CI detection.
// Priority order:
// 1. NO_COLOR env var (highest priority, per no-color.org standard)
// 2. FORCE_COLOR env var
// 3. Explicit "on" or "off" setting
// 4. CI detection (enables color in CI environments)
// 5. Default to "auto" (let terminal detection decide)
func resolveColor(current string) string {
	// NO_COLOR takes highest priority (per no-color.org standard)
	if env.Truthy(os.Getenv("NO_COLOR")) {
		log.Trace("NO_COLOR set, disabling colorized output")
		return offColor
	}

	// FORCE_COLOR enables color explicitly
	if env.Truthy(os.Getenv("FORCE_COLOR")) {
		log.Trace("FORCE_COLOR set, enabling colorized output")
		lipgloss.SetColorProfile(termenv.TrueColor)
		return onColor
	}

	// if explicitly set to on/off, honor it
	if current == onColor {
		lipgloss.SetColorProfile(termenv.TrueColor)
		return onColor
	}
	if current == offColor {
		return offColor
	}

	// auto mode: detect CI and enable color
	if ci.Detect() != ci.ProviderUnknown {
		log.Trace("CI detected, enabling colorized output by default")
		lipgloss.SetColorProfile(termenv.TrueColor)
		return onColor
	}

	return autoColor
}
