package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*Packages)(nil)

type Packages struct {
	Specifiers      []string `yaml:"packages" json:"packages" mapstructure:"packages"`
	ExcludePatterns []string `yaml:"exclude" json:"exclude" mapstructure:"exclude"`

	// internal

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func (o *Packages) PostLoad() error {
	return nil
}

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
