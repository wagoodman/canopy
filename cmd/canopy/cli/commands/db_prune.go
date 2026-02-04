package commands

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*dbPruneConfig)(nil)

type dbPruneConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`

	// All deletes all runs and sessions.
	All bool `yaml:"-" mapstructure:"-"`
	// Yes skips the confirmation prompt.
	Yes bool `yaml:"-" mapstructure:"-"`
	// OlderThan overrides store.max-age for this invocation (e.g., "30d", "720h").
	OlderThan string `yaml:"-" mapstructure:"-"`
	// KeepLast overrides store.max-runs for this invocation.
	KeepLast int `yaml:"-" mapstructure:"-"`
	// NoVacuum skips VACUUM after cleanup.
	NoVacuum bool `yaml:"-" mapstructure:"-"`
}

func (o *dbPruneConfig) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(&o.All, "all", "", "delete all test runs and sessions")
	flags.BoolVarP(&o.Yes, "yes", "y", "skip confirmation prompt")
	flags.StringVarP(&o.OlderThan, "older-than", "", "delete runs older than this duration (e.g., 30d, 720h)")
	flags.IntVarP(&o.KeepLast, "keep-last", "", "keep only the N most recent runs")
	flags.BoolVarP(&o.NoVacuum, "no-vacuum", "", "skip VACUUM after cleanup")
}

// DBPrune creates a command to remove old test runs and reclaim database space.
func DBPrune(app clio.Application) *cobra.Command {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true

	opts := &dbPruneConfig{
		Store: store,
	}

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "remove old test runs and reclaim database space",
		Long: `Remove old test runs, their events, coverage data, and associated records.

By default, applies the retention policy from your configuration (store.max-runs
and store.max-age). Use flags to override or delete everything.

After deletion, VACUUM is run to reclaim disk space (use --no-vacuum to skip).

Examples:
  canopy db prune                        # apply configured retention policy
  canopy db prune --all --yes            # delete everything without prompting
  canopy db prune --older-than 30d       # delete runs older than 30 days
  canopy db prune --keep-last 10         # keep only the 10 most recent runs
  canopy db prune --keep-last 5 --yes    # keep 5 most recent, no prompt`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDBPrune(*opts)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runDBPrune(cfg dbPruneConfig) error {
	log.Info("pruning test data")

	m, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create test manager: %w", err)
	}
	defer func() {
		if err := m.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test manager")
		}
	}()

	store := m.DBStore()
	if store == nil {
		return fmt.Errorf("no database store available")
	}

	if cfg.All {
		return pruneAll(store, cfg)
	}

	return pruneByPolicy(store, cfg)
}

func pruneAll(store *db.Store, cfg dbPruneConfig) error {
	count, err := store.CountRuns()
	if err != nil {
		return fmt.Errorf("unable to count runs: %w", err)
	}

	if count == 0 {
		fmt.Println("no test runs to delete")
		return nil
	}

	if !cfg.Yes {
		if !confirmAction(fmt.Sprintf("Delete all %d test run(s) and associated data?", count)) {
			fmt.Println("cancelled")
			return nil
		}
	}

	deleted, err := store.DeleteAllRuns()
	if err != nil {
		return fmt.Errorf("unable to delete runs: %w", err)
	}

	sessions, err := store.DeleteOrphanedSessions()
	if err != nil {
		log.WithFields("error", err).Warn("failed to clean up orphaned sessions")
	}

	fmt.Printf("deleted %d run(s) and %d session(s)\n", deleted, sessions)

	return maybeVacuum(store, cfg.NoVacuum)
}

func pruneByPolicy(store *db.Store, cfg dbPruneConfig) error {
	// determine which policy to apply: flags override config
	olderThan := cfg.OlderThan
	keepLast := cfg.KeepLast

	if olderThan == "" && keepLast == 0 {
		// fall back to config values
		olderThan = cfg.MaxAge
		keepLast = cfg.MaxRuns
	}

	if olderThan == "" && keepLast == 0 {
		fmt.Println("no retention policy configured; use --all, --older-than, or --keep-last")
		return nil
	}

	var totalDeleted int

	if olderThan != "" {
		deleted, cancelled, err := pruneByAge(store, olderThan, cfg.Yes)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		totalDeleted += deleted
	}

	if keepLast > 0 {
		deleted, cancelled, err := pruneByCount(store, keepLast, cfg.Yes)
		if err != nil {
			return err
		}
		if cancelled {
			return nil
		}
		totalDeleted += deleted
	}

	if totalDeleted == 0 {
		fmt.Println("nothing to prune")
		return nil
	}

	sessions, err := store.DeleteOrphanedSessions()
	if err != nil {
		log.WithFields("error", err).Warn("failed to clean up orphaned sessions")
	}

	fmt.Printf("deleted %d run(s) and %d session(s)\n", totalDeleted, sessions)

	return maybeVacuum(store, cfg.NoVacuum)
}

// pruneByAge deletes runs older than the given duration string. Returns (deleted, cancelled, error).
func pruneByAge(store *db.Store, olderThan string, skipPrompt bool) (int, bool, error) {
	maxAge, err := options.ParseDuration(olderThan)
	if err != nil {
		return 0, false, fmt.Errorf("invalid duration %q: %w", olderThan, err)
	}

	count, err := store.CountRunsByAge(maxAge)
	if err != nil {
		return 0, false, fmt.Errorf("unable to count runs by age: %w", err)
	}

	if count == 0 {
		return 0, false, nil
	}

	if !skipPrompt && !confirmAction(fmt.Sprintf("Delete %d run(s) older than %s?", count, olderThan)) {
		fmt.Println("cancelled")
		return 0, true, nil
	}

	deleted, err := store.DeleteRunsByAge(maxAge)
	if err != nil {
		return 0, false, fmt.Errorf("unable to delete runs by age: %w", err)
	}

	return deleted, false, nil
}

// pruneByCount deletes runs beyond the keep limit. Returns (deleted, cancelled, error).
func pruneByCount(store *db.Store, keepLast int, skipPrompt bool) (int, bool, error) {
	count, err := store.CountRunsBeyondKeep(keepLast)
	if err != nil {
		return 0, false, fmt.Errorf("unable to count excess runs: %w", err)
	}

	if count == 0 {
		return 0, false, nil
	}

	if !skipPrompt && !confirmAction(fmt.Sprintf("Delete %d run(s) to keep only the last %d?", count, keepLast)) {
		fmt.Println("cancelled")
		return 0, true, nil
	}

	deleted, err := store.DeleteRunsKeepingLast(keepLast)
	if err != nil {
		return 0, false, fmt.Errorf("unable to prune excess runs: %w", err)
	}

	return deleted, false, nil
}

func maybeVacuum(store *db.Store, noVacuum bool) error {
	if noVacuum {
		return nil
	}

	fmt.Print("running VACUUM to reclaim disk space...")
	if err := store.Vacuum(); err != nil {
		fmt.Println(" failed")
		return fmt.Errorf("unable to vacuum database: %w", err)
	}
	fmt.Println(" done")
	return nil
}

func confirmAction(prompt string) bool {
	fmt.Printf("%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}
