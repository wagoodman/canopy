package commands

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ fangs.FlagAdder = (*showConfig)(nil)

// showConfig holds all configuration options for the show command.
type showConfig struct {
	options.Config     `yaml:",inline" mapstructure:",squash"`
	options.Experiment `yaml:"exp" json:"exp" mapstructure:"exp"`
	options.Store      `yaml:"store" json:"store" mapstructure:"store"`

	Test   showTestConfig `yaml:"test" json:"test" mapstructure:"test"`
	Filter showFilter     `yaml:"filter" json:"filter" mapstructure:"filter"`
	RunID  string         `yaml:"-" json:"-" mapstructure:"-"` // from argument
}

// showTestConfig contains formatting options for test output display.
type showTestConfig struct {
	options.Format     `yaml:",inline" json:"" mapstructure:",squash"`
	options.Appearance `yaml:"appearance" json:"appearance" mapstructure:"appearance"`
}

// showFilter contains filter options for narrowing down which tests to display.
type showFilter struct {
	Failed  bool   `yaml:"failed" json:"failed" mapstructure:"failed"`
	Passed  bool   `yaml:"passed" json:"passed" mapstructure:"passed"`
	Skipped bool   `yaml:"skipped" json:"skipped" mapstructure:"skipped"`
	Test    string `yaml:"test" json:"test" mapstructure:"test"`
	Package string `yaml:"package" json:"package" mapstructure:"package"`
}

// AddFlags registers the filter flags with the flag set.
func (o *showFilter) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(&o.Failed, "failed", "", "show only failed tests")
	flags.BoolVarP(&o.Passed, "passed", "", "show only passed tests")
	flags.BoolVarP(&o.Skipped, "skipped", "", "show only skipped tests")
	flags.StringVarP(&o.Test, "test", "", "filter to matching test names (glob pattern)")
	flags.StringVarP(&o.Package, "package", "", "filter to matching packages (glob pattern)")
}

func defaultShowOptions() *showConfig {
	store := options.DefaultStore()
	store.Enabled = true
	store.HideEnabledFlag = true

	return &showConfig{
		Experiment: options.DefaultExperiment(),
		Store:      store,
		Test: showTestConfig{
			Format:     options.DefaultTestFormat(),
			Appearance: options.DefaultAppearance(),
		},
	}
}

// Show creates a command to display formatted output from a previous test run.
// It retrieves test events from the database and formats them using the same
// infrastructure as the format command, making output appear as if tests were just run.
func Show(app clio.Application) *cobra.Command {
	opts := defaultShowOptions()

	var logTestFailuresAsErrors bool
	cmd := &cobra.Command{
		Use:   "show [RUN-ID]",
		Short: "Show formatted test output from the last run (or a specific run)",
		Long: `View formatted output from a previous test run stored in the database.

By default, shows results from the last test run. Optionally specify a run ID
to view a specific run. Output is formatted using the same infrastructure as
'canopy format', making it appear as if the tests were just executed.

Examples:
  canopy show                    # show last run, all results
  canopy show <run-id>           # show specific run
  canopy show --failed           # show only failed tests
  canopy show --test 'TestFoo*'  # filter by test name pattern
  canopy show --package ./cmd/.. # filter by package pattern`,
		Args: cobra.MaximumNArgs(1),
		PreRunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.RunID = args[0]
			}
			var err error
			logTestFailuresAsErrors, err = setupTestUIs(app, opts.Test.Writers, opts.Test.Appearance, 30, "")
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runShow(cmd.Context(), app, *opts, logTestFailuresAsErrors)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runShow(ctx context.Context, _ clio.Application, cfg showConfig, logTestFailuresAsErrors bool) error { //nolint: funlen
	// create manager to access DB for reading historical data
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

	// determine which run to show
	runID, err := resolveRunID(m, cfg.RunID)
	if err != nil {
		return err
	}

	log.WithFields("run-id", runID.String()).Debug("showing test run")

	// get run info for configuration
	runInfo, err := m.GetRunInfo(runID)
	if err != nil {
		return fmt.Errorf("unable to get run info: %w", err)
	}

	// get event count for logging
	eventCount, err := m.GetTestEventCount(runID)
	if err != nil {
		return fmt.Errorf("unable to get event count: %w", err)
	}

	log.WithFields("event-count", eventCount).Debug("streaming events from database")

	// convert filter config to gotest.EventFilter
	filter := gotest.EventFilter{
		Failed:         cfg.Filter.Failed,
		Passed:         cfg.Filter.Passed,
		Skipped:        cfg.Filter.Skipped,
		TestPattern:    cfg.Filter.Test,
		PackagePattern: cfg.Filter.Package,
	}

	// create streaming event reader that fetches from DB in batches
	reader := gotest.NewDBEventReader(m, runID, filter, 1000)
	defer reader.Close()

	// create a separate manager for replaying events (NoStore to avoid re-persisting)
	s, err := test.NewManager(
		test.Config{
			DBRoot:    cfg.Root,
			Ephemeral: cfg.Ephemeral,
			NoStore:   true, // we don't want to store replayed events
		},
	)
	if err != nil {
		return fmt.Errorf("unable to create test session: %w", err)
	}
	defer func() {
		if err := s.Close(); err != nil {
			log.WithFields("error", err).Error("unable to close test session")
		}
	}()

	run, err := s.RunTests(
		ctx,
		test.RunConfig{
			LogTestFailuresAsErrors: logTestFailuresAsErrors,
			Runner:                  runInfo.Config,
			Result: gotest.ResultConfig{
				TrackOtherOutput:   false,
				TrackFailingOutput: false,
			},
			Reader: reader,
		},
	)

	if err != nil {
		return fmt.Errorf("unable to replay test events: %w", err)
	}

	passed, resultErr := evaluateResult(run, logTestFailuresAsErrors, 0) // no coverage minimum for show
	_ = passed

	return resultErr
}

// resolveRunID determines the run ID to display, either from explicit argument or from the last run.
func resolveRunID(m *test.Manager, runIDArg string) (uuid.UUID, error) {
	if runIDArg != "" {
		runID, err := uuid.Parse(runIDArg)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid run ID %q: %w", runIDArg, err)
		}
		return runID, nil
	}

	// get last run from most recent session
	sessions, err := m.ListSessions()
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		return uuid.Nil, fmt.Errorf("no test sessions found in database")
	}

	// sort sessions by start time (most recent first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Started.After(sessions[j].Started)
	})

	// find the most recent run across all sessions
	var latestRunID uuid.UUID
	var latestRunTime = sessions[0].Started // initialize with oldest possible time for comparison

	for _, session := range sessions {
		for _, run := range session.Runs {
			if run.Started.After(latestRunTime) || latestRunID == uuid.Nil {
				latestRunTime = run.Started
				latestRunID = run.UUID
			}
		}
	}

	if latestRunID == uuid.Nil {
		return uuid.Nil, fmt.Errorf("no test runs found in database")
	}

	return latestRunID, nil
}
