package options

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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

	// MaxRuns is the maximum number of test runs to retain (0 = unlimited).
	MaxRuns int `yaml:"max-runs" mapstructure:"max-runs"`
	// MaxAge is the maximum age of test runs to keep (e.g., "30d", "720h"; "" = unlimited).
	MaxAge string `yaml:"max-age" mapstructure:"max-age"`

	// parsedMaxAge is MaxAge parsed into a time.Duration (set in PostLoad).
	parsedMaxAge time.Duration

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// ParsedMaxAge returns the parsed duration from MaxAge.
func (o *Store) ParsedMaxAge() time.Duration {
	return o.parsedMaxAge
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

// PostLoad configures ephemeral storage, expands the root path, and parses retention settings.
func (o *Store) PostLoad() error {
	// when the enabled flag is hidden, force it to be enabled (prevents env var overrides)
	if o.HideEnabledFlag {
		o.Enabled = true
	}

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

	if o.MaxAge != "" {
		d, err := ParseDuration(o.MaxAge)
		if err != nil {
			return fmt.Errorf("invalid store.max-age %q: %w", o.MaxAge, err)
		}
		o.parsedMaxAge = d
	}

	return nil
}

// ParseDuration parses a duration string supporting both Go-style ("720h", "1h30m") and
// day-style ("30d") formats.
func ParseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}

	// try standard Go duration parsing first
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// handle day suffix: "30d" → 30 * 24h
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		if days < 0 {
			return 0, fmt.Errorf("invalid duration %q: days must be non-negative", s)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}

	return 0, fmt.Errorf("invalid duration %q", s)
}
