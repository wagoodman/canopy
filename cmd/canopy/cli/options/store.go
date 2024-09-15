package options

import (
	"fmt"
	"os"

	"github.com/mitchellh/go-homedir"
	"github.com/wagoodman/canopy/cmd/canopy/internal"

	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*Store)(nil)
	_ fangs.PostLoader = (*Store)(nil)
)

type Store struct {
	Enabled   bool   `yaml:"enabled" mapstructure:"enabled"`
	Root      string `yaml:"root" mapstructure:"root"`
	Ephemeral bool   `yaml:"yaml" mapstructure:"-"`
}

func DefaultStore() Store {
	return Store{
		Enabled: false,
		Root:    fmt.Sprintf(".%s", internal.ApplicationName),
	}
}

func (c *Store) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(&c.Enabled, "store", "", "store test output to a sqlite DB")
}

func (c *Store) PostLoad() error {
	if !c.Enabled {
		c.Ephemeral = true
		c.Root = ""
	}

	if c.Root == "" {
		var err error
		c.Root, err = os.MkdirTemp("", "canopy-db")
		if err != nil {
			return fmt.Errorf("unable to create temporary directory for db: %v", err)
		}
	}

	cleanRoot, err := homedir.Expand(c.Root)
	if err != nil {
		return fmt.Errorf("unable to expand store path %q: %v", c.Root, err)
	}

	c.Root = cleanRoot

	return nil
}
