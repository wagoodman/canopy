package options

import (
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*Open)(nil)

type Open struct {
	Disabled             bool `yaml:"-" json:"-" mapstructure:"-"`
	OpenSessionOnFailure bool `yaml:"open-on-failure" json:"open-on-failure" mapstructure:"open-on-failure"`
	tracker              *xflagset.Decorator
	NamedFlagSet         *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultOpen() Open {
	return Open{
		OpenSessionOnFailure: false,
	}
}

func (o *Open) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("State"))
	flags = o.tracker

	if !o.Disabled {
		flags.BoolVarP(&o.OpenSessionOnFailure, "open-on-failure", "f", "open an interactive session on test failure")
	}
}
