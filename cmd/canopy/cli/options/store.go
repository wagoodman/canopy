package options

import (
	"fmt"
	"os"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal"

	"github.com/anchore/fangs"
	"github.com/anchore/go-homedir"
)

var (
	_ fangs.FlagAdder  = (*Store)(nil)
	_ fangs.PostLoader = (*Store)(nil)
)

type Store struct {
	Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
	Root      string `yaml:"root" mapstructure:"root"`
	Ephemeral bool   `yaml:"yaml" mapstructure:"-"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultStore() Store {
	return Store{
		Enabled: false,
		Root:    fmt.Sprintf(".%s", internal.ApplicationName),
	}
}

func (o *Store) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("State"))
	flags = o.tracker

	flags.BoolVarP(&o.Enabled, "store", "", "store test output to a sqlite DB")
	flags.StringVarP(&o.Root, "store-dir", "", "directory to store test output to a sqlite DB (enabled by --store)")
}

func (o *Store) PostLoad() error {
	if !o.Enabled {
		o.Ephemeral = true
		o.Root = ""
	}

	if o.Root == "" {
		var err error
		o.Root, err = os.MkdirTemp("", "canopy-db")
		if err != nil {
			return fmt.Errorf("unable to create temporary directory for db: %v", err)
		}
	}

	cleanRoot, err := homedir.Expand(o.Root)
	if err != nil {
		return fmt.Errorf("unable to expand store path %q: %v", o.Root, err)
	}

	o.Root = cleanRoot

	return nil
}
