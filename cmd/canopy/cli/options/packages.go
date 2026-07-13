package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*Packages)(nil)

// DefaultPackageSpecifier is the package path pattern used when the user gives none:
// all packages under the current module, recursively.
const DefaultPackageSpecifier = "./..."

// Packages configures package selection for test discovery and execution.
type Packages struct {
	// Specifiers are Go package path patterns (e.g., "./...", "github.com/user/pkg/...").
	Specifiers []string `yaml:"packages" json:"packages" mapstructure:"packages"`
	// ExcludePatterns are glob patterns of package paths to ignore during selection.
	ExcludePatterns []string `yaml:"exclude" json:"exclude" mapstructure:"exclude"`

	// internal

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultPackages returns package options defaulting to all packages under the current module.
func DefaultPackages() Packages {
	return Packages{
		Specifiers: []string{DefaultPackageSpecifier},
	}
}

// PostLoad performs any necessary post-configuration validation (currently a no-op).
func (o *Packages) PostLoad() error {
	return nil
}

// AddFlags registers package selection flags with the flag set.
func (o *Packages) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Package Selection"))
	flags = o.tracker

	flags.StringArrayVarP(
		&o.ExcludePatterns,
		"exclude", "e",
		"glob patterns of package paths to ignore",
	)
}
