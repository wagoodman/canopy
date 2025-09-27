package options

import (
	"os"
	"strings"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/fangs"
)

var (
	_ fangs.PostLoader = (*Appearance)(nil)
)

type Appearance struct {
	CombineMultipleRuns     bool `yaml:"-" json:"-" mapstructure:"-"`
	NoColor                 bool `yaml:"no-color" json:"no-color" mapstructure:"no-color"`
	ShowPackagesWithNoTests bool `yaml:"show-packages-with-no-tests" json:"show-packages-with-no-tests" mapstructure:"show-packages-with-no-tests"`
	UseShortNames           bool `yaml:"use-short-names" json:"use-short-names" mapstructure:"use-short-names"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultAppearance() Appearance {
	return Appearance{
		NoColor:                 false,
		ShowPackagesWithNoTests: false,
		UseShortNames:           true,
	}
}

func (o *Appearance) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Format"))
	flags = o.tracker

	flags.BoolVarP(
		&o.NoColor,
		"no-color", "",
		"disable all colorized output (can be overridden by the NO_COLOR environment variable as well)",
	)
}

func (o *Appearance) PostLoad() error {
	overrideNoColorFromEnv(&o.NoColor)
	return nil
}

func overrideNoColorFromEnv(opt *bool) {
	// override no-color with NO_COLOR env var
	noColorEnvVar := strings.TrimSpace(os.Getenv("NO_COLOR"))
	switch strings.ToLower(noColorEnvVar) {
	case "true", "1", "t":
		log.WithFields("NO_COLOR", noColorEnvVar).Trace("disabling colorized output")
		*opt = true
	}
}
