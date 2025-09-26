package options

import (
	"fmt"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*Coverage)(nil)
	_ fangs.PostLoader = (*Coverage)(nil)
)

type Coverage struct {
	Disabled bool `yaml:"-" json:"-" mapstructure:"-"`

	Cover    bool    `yaml:"cover" json:"cover" mapstructure:"cover"`          // custom flag
	CoverMin float64 `yaml:"covermin" json:"covermin" mapstructure:"covermin"` // custom flag

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultCoverage() Coverage {
	return Coverage{
		Cover:    false,
		CoverMin: 0.0, // default to no minimum coverage
	}
}

func (o *Coverage) PostLoad() error {
	if o.Disabled {
		return nil
	}

	if o.CoverMin > 0 {
		o.Cover = true
	}

	if o.CoverMin < 0 || o.CoverMin > 100 {
		return fmt.Errorf("invalid coverage minimum value '%0.2f' (must be between 0 and 100)", o.CoverMin)
	}

	return nil
}

func (o *Coverage) AddFlags(fangFlags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(fangFlags, o.NamedFlagSet.FlagSet("Test"))
	flags := o.tracker

	if !o.Disabled {
		flags.BoolVarP(&o.Cover, "cover", "", "enable coverage analysis")

		// custom flags
		flags.WithNoTrack().Float64VarP(&o.CoverMin, "covermin", "", "minimum coverage to enforce (percentage)")
	}
}
