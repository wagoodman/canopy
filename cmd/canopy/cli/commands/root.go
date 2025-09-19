package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
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
	return app.SetupRootCommand(&cobra.Command{
		Use:   fmt.Sprintf("%s GO-PKG-SPECIFIER...", app.ID().Name),
		Short: "select and run go tests",
		Long:  "This is a wrapper around the 'go test' command that provides additional value. See 'go help test' and 'go help build' for detailed flag information.",
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
				return fmt.Errorf("unble to get test paths: %w", err)
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
	}, opts)
}

func runRoot(ctx context.Context, app clio.Application, rootCfg rootConfig) error {
	// we can't narrow down the definitions based on the run statements, instead we want to capture from the definitions,
	// which references would be selected from those definitions. If we fitler the definitions based on the run statements,
	// then we're at risk of claiming that the minimal test selection can select a full package, when in fact it cannot
	// (since we may have removed some tests from the package definitions). Thus we never filter the test definitions.
	testDefs, err := gotest.FindDefinitions(rootCfg.Test.Runtime.Packages)
	if err != nil {
		return err
	}

	if len(testDefs) == 0 {
		return fmt.Errorf("no tests found in packages %q (with run options %q)", rootCfg.Test.Specifiers, rootCfg.Test.Run)
	}

	id := app.ID()

	var selected gotest.References

	if len(rootCfg.Test.Run) > 0 {
		runPatterns, err := internal.MakeRegexes(rootCfg.Test.Run)
		if err != nil {
			return fmt.Errorf("failed to compile '-run' patterns: %w", err)
		}

		patternRemoveFilter := func(ref gotest.Reference) bool {
			if !internal.MatchesAny(ref.FuncName, runPatterns) {
				log.WithFields("fn", ref.FuncName, "package", ref.Package).Trace("skipping test function that does not match run patterns")
				return true
			}
			return false
		}

		pkgRemoveFilter := func(ref gotest.Reference) bool {
			if ref.IsPackage() {
				return true
			}
			return false
		}

		selected = testDefs.References(patternRemoveFilter, pkgRemoveFilter)
	}

	ux := ui.NewSelectorUI(selector.Config{
		ID:    fmt.Sprintf("%s@%s", id.Name, id.Version),
		Debug: false,
	}, testDefs, selected)

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	if err = state.UI.Replace(ui.NewCollection(ux)); err != nil {
		return fmt.Errorf("unable to replace UI: %w", err)
	}

	refs := ux.Prompt()

	// set the UI dynamically
	maxPkgName := maxPkgNameLength(refs.Packages())
	logTestFailuresAsErrors, err := setupTestUIs(app, rootCfg.Test.Writers, rootCfg.Test.Appearance, maxPkgName)
	if err != nil {
		return fmt.Errorf("unable to setup test UIs: %w", err)
	}

	runGroups := gotest.GroupIntoRuns(gotest.MinimalSelection(testDefs, refs))

	if len(runGroups) == 0 {
		return nil
	}

	log.WithFields("references", refs.TestFunctionsCount()).Info("running selected tests")
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

	coreCfg := rootCfg.TestCoreConfig
	cfg := coreCfg.Test

	s, err := test.NewManager(
		test.Config{
			DBRoot:    coreCfg.Root,
			Ephemeral: coreCfg.Ephemeral,
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

	var commonArgs []string
	commonArgs = append(commonArgs, cfg.GoBuild.RenderedFlags...)
	commonArgs = append(commonArgs, cfg.GoTest.RenderedFlags...)
	commonArgs = append(commonArgs, cfg.ExtraFlags...)

	var runCfgs []test.RunConfig
	for _, group := range runGroups {
		var args []string
		args = append(args, commonArgs...)
		args = append(args, group.Packages()...)

		rArgs := runArgs(group)
		if rArgs != "" {
			args = append(args, rArgs)
		}

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

	start := time.Now()
	var runs []*gotest.Run
	err = sync.CollectSlice(&ctx, internal.ExecutorTestRunner,
		sync.ToSeq(runCfgs),
		func(runConfig test.RunConfig) (*gotest.Run, error) {
			return s.RunTests(ctx, runConfig)
		},
		&runs,
	)

	if err != nil {
		return fmt.Errorf("unable to run tests: %w", err)
	}

	_, resultErr := evaluateResults(runs, time.Since(start), logTestFailuresAsErrors)

	return resultErr
}

func runArgs(refs gotest.References) string {
	var funcs []string
	for _, ref := range refs {
		if ref.FuncName != "" {
			funcs = append(funcs, ref.FuncName)
		}
	}
	if len(funcs) == 0 {
		return ""
	}
	return "-run='" + strings.Join(funcs, "|") + "'"
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
