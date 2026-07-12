// Package commands implements the CLI commands for canopy.
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/selector"
	"github.com/wagoodman/canopy/cmd/canopy/internal"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/go-sync"
)

// const prettyTitle = `
// ▛▘▀▌▛▌▛▌▛▌▌▌
// ▙▖█▌▌▌▙▌▙▌▙▌
//        ▌ ▄▌
//`

// const prettyTitle = `
// ░█▀▀░█▀█░█▀█░█▀█░█▀█░█░█
// ░█░░░█▀█░█░█░█░█░█▀▀░░█░
// ░▀▀▀░▀░▀░▀░▀░▀▀▀░▀░░░░▀░
//`

const prettyTitle = `
█▀▀ ▄▀▀▄ █▀▀▄ █▀▀█ █▀▀█ █  █
█░░ █▄▄█ █░░█ █░░█ █░░█ █░░█
▀▀▀ ▀  ▀ ▀  ▀ ▀▀▀▀ █▀▀▀ ▀▀▀█
                   ▀     ▀▀`

type rootConfig struct {
	*TestCoreConfig `yaml:",inline" json:",inline" mapstructure:",squash"`
}

func defaultRootOptions() *rootConfig {
	c := rootConfig{
		TestCoreConfig: defaultTestOptions(
			withoutCoverageOpts(),     // we cannot determine coverage for multiple sessions, so we disable it here
			withoutOpenOpts(),         // we cannot open failed results as this may be for multiple sessions
			withoutRunOptsRendered(),  // -run is being rendered based on the user selection, thus does not need to be rendered to be passed to 'go test'
			withCombineMultipleRuns(), // we want a single summary for multiple running sessions
		),
	}
	c.Test.Specifiers = []string{"./..."} // default to all project packages
	return &c
}

func Root(app clio.Application) *cobra.Command {
	opts := defaultRootOptions()

	var runErr error
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s GO-PKG-SPECIFIER...", app.ID().Name),
		Short: "select and run go tests",
		Long:  "This is a wrapper around the 'go test' command that provides additional value. See 'go help test' and 'go help build' for detailed flag information." + "\n" + prettyTitle,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Test.Specifiers = args
			}

			return nil
		},
		//Example: // TODO
		PreRunE: func(_ *cobra.Command, _ []string) error {
			// get the final set of packages to use
			testPkgs, err := golist.SelectPackages(opts.Test.Specifiers, opts.Test.ExcludePatterns)
			if err != nil {
				return fmt.Errorf("unable to get test paths: %w", err)
			}
			if testPkgs.Size() == 0 {
				return fmt.Errorf("no packages selected to test (given %q)", opts.Test.Specifiers)
			}
			opts.Test.Runtime.Packages = testPkgs

			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer func() {
				if err := opts.Test.Writers.Close(); err != nil {
					runErr = multierror.Append(runErr, err)
					log.WithFields("error", err).Error("unable to close format writers")
				}
			}()

			return runRoot(cmd.Context(), app, *opts)
		},
		PostRunE: func(_ *cobra.Command, _ []string) error {
			// this runs after the UI, so we can safely print to stdout/stderr now if we need to
			if runErr != nil {
				showTestFailure(runErr)
			}
			return runErr
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupRootCommand(cmd, opts)
}

func runRoot(ctx context.Context, app clio.Application, rootCfg rootConfig) error {
	testDefs, selected, err := discoverAndSelectTests(rootCfg)
	if err != nil {
		return err
	}

	refs, err := promptUserSelection(app, testDefs, selected)
	if err != nil {
		return err
	}

	plan, err := prepareTestExecution(app, rootCfg, testDefs, refs)
	if err != nil {
		return err
	}

	if len(plan.runGroups) == 0 {
		return nil
	}

	return executeTests(ctx, rootCfg.TestCoreConfig, plan)
}

// discoverAndSelectTests finds test definitions and applies run pattern filtering
func discoverAndSelectTests(rootCfg rootConfig) (gotest.Definitions, gotest.References, error) {
	// we can't narrow down the definitions based on the run statements, instead we want to capture from the definitions,
	// which references would be selected from those definitions. If we fitler the definitions based on the run statements,
	// then we're at risk of claiming that the minimal test selection can select a full package, when in fact it cannot
	// (since we may have removed some tests from the package definitions). Thus we never filter the test definitions.
	testDefs, err := gotest.FindDefinitions(rootCfg.Test.Runtime.Packages)
	if err != nil {
		return nil, nil, err
	}

	if len(testDefs) == 0 {
		return nil, nil, fmt.Errorf("no tests found in packages %q (with run options %q)", rootCfg.Test.Specifiers, rootCfg.Test.Run)
	}

	var selected gotest.References

	if len(rootCfg.Test.Run) > 0 {
		runPatterns, err := internal.MakeRegexes(rootCfg.Test.Run)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to compile '-run' patterns: %w", err)
		}

		patternRemoveFilter := func(ref gotest.Reference) bool {
			// match against the full Func/Case name so `--run 'TestFoo/case'` can target a
			// single table case, mirroring `go test -run` semantics
			if !internal.MatchesAny(ref.TestName(true), runPatterns) {
				log.WithFields("test", ref.TestName(true), "package", ref.Package).Trace("skipping test that does not match run patterns")
				return true
			}
			return false
		}

		pkgRemoveFilter := func(ref gotest.Reference) bool {
			return ref.IsPackage()
		}

		selected = testDefs.References(patternRemoveFilter, pkgRemoveFilter)
	}

	return testDefs, selected, nil
}

// promptUserSelection creates the UI and prompts the user to select tests
func promptUserSelection(app clio.Application, testDefs gotest.Definitions, preSelected gotest.References) (gotest.References, error) {
	id := app.ID()

	ux := ui.NewSelectorUI(selector.Config{
		ID:    fmt.Sprintf("%s@%s", id.Name, id.Version),
		Debug: false,
	}, testDefs, preSelected)

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	if err := state.UI.Replace(ui.NewCollection(ux)); err != nil {
		return nil, fmt.Errorf("unable to replace UI: %w", err)
	}

	refs := ux.Prompt()
	return refs, nil
}

// executionPlan contains all the data needed to execute tests
type executionPlan struct {
	runGroups               []gotest.References
	runConfigs              []test.RunConfig
	logTestFailuresAsErrors bool
}

// prepareTestExecution sets up UIs, groups tests, and builds run configurations
func prepareTestExecution(app clio.Application, rootCfg rootConfig, testDefs gotest.Definitions, refs gotest.References) (*executionPlan, error) {
	// set the UI dynamically
	module := testDefs.Module()
	maxPkgName := maxPkgNameLength(refs.Packages(), module)
	logTestFailuresAsErrors, err := setupTestUIs(app, rootCfg.Test.Writers, rootCfg.Test.Appearance, maxPkgName, module)
	if err != nil {
		return nil, fmt.Errorf("unable to setup test UIs: %w", err)
	}

	// MinimizeReferences (unlike the old MinimalSelection) preserves t.Run case leaves, so a
	// single selected table case survives to `-run=^Func/Case$`.
	minimized := gotest.MinimizeReferences(testDefs.References(), refs)
	if len(minimized) == 0 && len(refs) > 0 {
		// a fully-selected set collapses to nothing (the tree prunes to its root); represent
		// that as "run every selected package with no -run filter"
		for _, pkg := range refs.Packages() {
			minimized = append(minimized, gotest.Reference{Package: pkg})
		}
	}

	runGroups := gotest.GroupIntoRuns(minimized)

	if len(runGroups) == 0 {
		return &executionPlan{
			runGroups:               runGroups,
			runConfigs:              nil,
			logTestFailuresAsErrors: logTestFailuresAsErrors,
		}, nil
	}

	log.WithFields("references", refs.TestFunctionsCount()).Info("running selected tests")
	logRunGroups(runGroups)

	cfg := rootCfg.Test

	var commonArgs []string
	commonArgs = append(commonArgs, cfg.GoBuild.RenderedFlags...)
	commonArgs = append(commonArgs, cfg.GoTest.RenderedFlags...)
	commonArgs = append(commonArgs, cfg.ExtraFlags...)

	var runCfgs []test.RunConfig
	for _, group := range runGroups {
		// UserArgs carries only the package args; test selection (-run) is applied by
		// the runner via OnlyRefs, so we must not inject -run here.
		var args []string
		args = append(args, commonArgs...)
		args = append(args, group.Packages()...)

		runCfgs = append(runCfgs,
			test.RunConfig{
				LogTestFailuresAsErrors: logTestFailuresAsErrors,
				Runner: gotest.RunnerConfig{
					OnlyRefs: group,
					Coverage: false, // coverage is not supported in this command
					NoCache:  cfg.NoCache,
					UserArgs: args,
				},
				Result: gotest.ResultConfig{
					TrackOtherOutput:   false,
					TrackFailingOutput: false,
				},
			},
		)
	}

	return &executionPlan{
		runGroups:               runGroups,
		runConfigs:              runCfgs,
		logTestFailuresAsErrors: logTestFailuresAsErrors,
	}, nil
}

// logRunGroups emits the selected test run groups as a trace-level tree.
func logRunGroups(runGroups []gotest.References) {
	log.WithFields("runGroups", len(runGroups)).Debug("selected test run groups")
	for i, group := range runGroups {
		log.WithFields("group", i+1, "refs", len(group)).Trace("test run group")
		for j, ref := range group {
			branch := "├── "
			if j == len(group)-1 {
				branch = "└── "
			}
			log.Trace(branch + ref.String(true))
		}
	}
}

// executeTests creates a session, runs all test configurations, and evaluates results
func executeTests(ctx context.Context, coreCfg *TestCoreConfig, plan *executionPlan) error {
	s, err := test.NewManager(
		test.Config{
			DBRoot:    coreCfg.Root,
			Ephemeral: coreCfg.Ephemeral,
			Retention: test.RetentionConfig{
				MaxRuns: coreCfg.MaxRuns,
				MaxAge:  coreCfg.ParsedMaxAge(),
			},
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

	start := time.Now()
	var runs []*gotest.Run
	err = sync.CollectSlice(&ctx, internal.ExecutorTestRunner,
		sync.ToSeq(plan.runConfigs),
		func(runConfig test.RunConfig) (*gotest.Run, error) {
			return s.RunTests(ctx, runConfig)
		},
		&runs,
	)

	if err != nil {
		return fmt.Errorf("unable to run tests: %w", err)
	}

	_, resultErr := evaluateResults(runs, time.Since(start), plan.logTestFailuresAsErrors)

	return resultErr
}

func evaluateResults(runs []*gotest.Run, elapsed time.Duration, logTestFailuresAsErrors bool) (bool, error) {
	var resultStr = "passed"
	var passed = true
	var resultErr error
	for _, run := range runs {
		result := run.Result
		runPassed, _ := result.Passed()
		passed = passed && runPassed

		if !runPassed {
			resultStr = "failed"

			if len(result.References()) == 0 {
				resultErr = multierror.Append(resultErr, ErrTestSuiteFailed{Reasons: []string{"no test events observed"}})
			} else {
				// if tests simply failed, let the UI show this as a failure, no need to show an additional message
				resultErr = multierror.Append(resultErr, ErrTestSuiteFailed{})
			}
		}
	}

	nested := log.WithFields("result", resultStr, "elapsed", fmt.Sprintf("%2.2fs", elapsed.Seconds()))

	if !passed && logTestFailuresAsErrors {
		nested.Error("completed test suite")
	} else {
		nested.Info("completed test suite")
	}

	return passed, resultErr
}
