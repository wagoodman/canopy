package options

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/ci"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"

	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*Shard)(nil)
	_ fangs.PostLoader = (*Shard)(nil)
	_ fangs.FlagAdder  = (*ShardIndex)(nil)
	_ fangs.PostLoader = (*ShardIndex)(nil)
)

// ShardAuto is the --shard value that reads the shard from the CI provider's environment.
const ShardAuto = "auto"

// Shard configures where sharded runs keep receipts and metrics. It lives under test.shard and is
// shared by `canopy test`, `canopy shard join` and `canopy shard plan`, so the keys, env vars and
// flags are the same everywhere.
type Shard struct {
	// Disabled prevents the shard flags from being added to the command.
	Disabled bool `yaml:"-" json:"-" mapstructure:"-"`

	// Dir holds the metrics/ dir and the out/ dir with receipts (independent of the store dir).
	Dir string `yaml:"dir" json:"dir" mapstructure:"dir"`
	// Overhead is the fixed per-shard cost (runner startup, checkout, build) the shard count suggestion assumes.
	Overhead string `yaml:"overhead" json:"overhead" mapstructure:"overhead"`

	parsedOverhead time.Duration

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultShard returns the shard options with the conventional .canopy/shard dir.
func DefaultShard() Shard {
	return Shard{
		Dir:      ".canopy/shard",
		Overhead: "60s",
	}
}

// ParsedOverhead returns Overhead as a duration.
func (o *Shard) ParsedOverhead() time.Duration {
	return o.parsedOverhead
}

// AddFlags registers the shard dir and overhead flags.
func (o *Shard) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Sharding"))
	flags = o.tracker

	if !o.Disabled {
		flags.StringVarP(&o.Dir, "shard-dir", "", "directory for shard receipts (out/) and timing metrics")
		flags.StringVarP(&o.Overhead, "overhead", "", "fixed per-shard cost assumed by the shard count suggestion (e.g. 60s)")
	}
}

// PostLoad validates the overhead duration.
func (o *Shard) PostLoad() error {
	d, err := time.ParseDuration(o.Overhead)
	if err != nil || d < 0 {
		return fmt.Errorf("invalid shard overhead %q (want a duration like 60s)", o.Overhead)
	}
	o.parsedOverhead = d
	return nil
}

// ShardEnv is the env var read when --shard isn't given. fangs doesn't bind it (the field is
// mapstructure:"-"), so PostLoad looks it up directly.
const ShardEnv = "CANOPY_TEST_SHARD"

// ShardIndex is the --shard flag on `canopy test`. It deliberately never comes from a config file
// (the owning field is tagged mapstructure:"-"): the index changes per job, and a checked-in value
// would quietly shard every local run. CANOPY_TEST_SHARD is the only fallback.
type ShardIndex struct {
	// Disabled prevents the --shard flag from being added to the command.
	Disabled bool `yaml:"-" json:"-" mapstructure:"-"`

	// Value is the raw flag value: "i/n", "auto", or empty when not sharding.
	Value string `yaml:"-" json:"-" mapstructure:"-"`
	// FromEnv is set when Value came from CANOPY_TEST_SHARD instead of the flag.
	FromEnv bool `yaml:"-" json:"-" mapstructure:"-"`

	// Index and Total are the parsed i/n (1-based), zero for "auto" until Resolve.
	Index int `yaml:"-" json:"-" mapstructure:"-"`
	Total int `yaml:"-" json:"-" mapstructure:"-"`

	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// AddFlags registers --shard. It is a plain string flag, not an optional-value one: pflag would
// otherwise read `--shard 1/4` as a bare --shard followed by a package argument.
func (o *ShardIndex) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	tracker := xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Sharding"))

	if !o.Disabled {
		tracker.WithNoTrack().StringVarP(&o.Value, "shard", "", "run only shard i of n of the packages (e.g. 1/4) and write a receipt for 'canopy shard join'; 'auto' reads i/n from the CI provider")
	}
}

// PostLoad parses and validates the flag value.
func (o *ShardIndex) PostLoad() error {
	if o.Value == "" && !o.Disabled {
		if v := os.Getenv(ShardEnv); v != "" {
			o.Value, o.FromEnv = v, true
		}
	}
	if o.Value == "" || o.Value == ShardAuto {
		return nil
	}
	i, n, err := ParseShard(o.Value)
	if err != nil {
		return err
	}
	o.Index, o.Total = i, n
	return nil
}

// Enabled reports whether --shard was given.
func (o ShardIndex) Enabled() bool {
	return o.Value != ""
}

// Resolve returns the 1-based shard index and total, reading the CI provider's variables for
// "auto". source is "flag", "CANOPY_TEST_SHARD (env)", the CI variable pair used, or empty when auto found no parallelism
// (and resolved to 1/1).
func (o ShardIndex) Resolve(e env.EnvironmentGetter) (index, total int, source string, err error) {
	if o.Value != ShardAuto {
		if o.FromEnv {
			return o.Index, o.Total, ShardEnv + " (env)", nil
		}
		return o.Index, o.Total, "flag", nil
	}
	i, n, src, err := ci.ShardFromEnv(e)
	if err != nil {
		return 0, 0, "", fmt.Errorf("--shard auto: %w", err)
	}
	return i, n, src, nil
}

// ParseShard parses a 1-based "i/n" shard value.
func ParseShard(s string) (index, total int, err error) {
	is, ns, ok := strings.Cut(s, "/")
	i, iErr := strconv.Atoi(is)
	n, nErr := strconv.Atoi(ns)
	if !ok || iErr != nil || nErr != nil {
		return 0, 0, fmt.Errorf("invalid --shard %q (want i/n like 1/4, or auto)", s)
	}
	if n < 1 || i < 1 || i > n {
		return 0, 0, fmt.Errorf("invalid --shard %q (want 1 <= i <= n)", s)
	}
	return i, n, nil
}
