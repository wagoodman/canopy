package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*Open)(nil)

// Open configures options for opening an interactive session on test failure.
type Open struct {
	// Disabled prevents the open-on-failure flag from being added to commands.
	Disabled bool `yaml:"-" json:"-" mapstructure:"-"`
	// OpenSessionOnFailure controls whether to open an interactive UI session when tests fail.
	OpenSessionOnFailure bool `yaml:"open-on-failure" json:"open-on-failure" mapstructure:"open-on-failure"`
	tracker              *xflagset.Decorator
	NamedFlagSet         *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultOpen returns open options with interactive session disabled by default.
func DefaultOpen() Open {
	return Open{
		OpenSessionOnFailure: false,
	}
}

// AddFlags registers the open-on-failure flag with the flag set.
func (o *Open) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("State"))
	flags = o.tracker

	if !o.Disabled {
		flags.BoolVarP(&o.OpenSessionOnFailure, "open-on-failure", "f", "open an interactive session on test failure")
	}
}
