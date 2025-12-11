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

// Coverage configures test coverage analysis options including whether to collect coverage and minimum coverage thresholds.
type Coverage struct {
	// Disabled prevents coverage flags from being added to the command.
	Disabled bool `yaml:"-" json:"-" mapstructure:"-"`

	// Cover enables coverage analysis during test execution.
	Cover bool `yaml:"cover" json:"cover" mapstructure:"cover"` // custom flag
	// CoverMin specifies the minimum coverage percentage required (0-100). Setting this also enables coverage.
	CoverMin float64 `yaml:"covermin" json:"covermin" mapstructure:"covermin"` // custom flag

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultCoverage returns coverage options with coverage disabled by default.
func DefaultCoverage() Coverage {
	return Coverage{
		Cover:    false,
		CoverMin: 0.0, // default to no minimum coverage
	}
}

// PostLoad validates coverage configuration and auto-enables coverage if a minimum is specified.
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

// AddFlags registers coverage-related flags with the flag set.
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
