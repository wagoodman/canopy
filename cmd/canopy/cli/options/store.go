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

// Store configures persistent storage of test results in a SQLite database.
type Store struct {
	// HideEnabledFlag prevents the --store flag from being added to the command.
	// Use this for commands where the store is always enabled.
	HideEnabledFlag bool `yaml:"-" json:"-" mapstructure:"-"`

	// Enabled controls whether test results should be stored in the database.
	Enabled bool `yaml:"enabled" mapstructure:"enabled"`
	// Root is the directory path where the SQLite database will be stored.
	Root string `yaml:"root" mapstructure:"root"`
	// Ephemeral indicates whether the database should be temporary and discarded after use.
	Ephemeral bool `yaml:"yaml" mapstructure:"-"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultStore returns store options with persistence disabled by default and using the .canopy directory.
func DefaultStore() Store {
	return Store{
		Enabled: false,
		Root:    fmt.Sprintf(".%s", internal.ApplicationName),
	}
}

// AddFlags registers store-related flags with the flag set.
func (o *Store) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("State"))
	flags = o.tracker

	if !o.HideEnabledFlag {
		flags.BoolVarP(&o.Enabled, "store", "", "store test output to a sqlite DB")
	}
	flags.StringVarP(&o.Root, "store-dir", "", "directory to store test output to a sqlite DB (enabled by --store)")
}

// PostLoad configures ephemeral storage and expands the root path, creating temp directories if needed.
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
