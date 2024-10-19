package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
)

const defaultPackageSelection = "./..."

var (
	_ fangs.FlagAdder  = (*testCoreConfig)(nil)
	_ fangs.PostLoader = (*testCoreConfig)(nil)
)

type ErrTestSuiteFailed struct {
	Reasons []string
}

func (e ErrTestSuiteFailed) Error() string {
	var render string
	for _, reason := range e.Reasons {
		render += fmt.Sprintf("\n  - %s", reason)
	}
	return fmt.Sprintf("test suite failed: %s", render)
}

type testCoreConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`

	Test testConfig `yaml:"test" json:"test" mapstructure:"test"`

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

type testConfig struct {
	options.Packages   `yaml:",inline" json:"" mapstructure:",squash"`
	options.GoTest     `yaml:",inline" json:"" mapstructure:",squash"`
	options.Coverage   `yaml:",inline" json:"" mapstructure:",squash"`
	options.GoBuild    `yaml:",inline" json:"" mapstructure:",squash"`
	options.Format     `yaml:",inline" json:"" mapstructure:",squash"`
	options.Open       `yaml:",inline" json:"" mapstructure:",squash"`
	options.Appearance `yaml:",inline" json:"" mapstructure:",squash"`
	ExtraFlags         []string `yaml:"extra-flags" json:"extra-flags" mapstructure:"extra-flags"`

	// post parse
	Runtime testRuntimeConfig `yaml:"-" json:"-" mapstructure:"-"`
}

type testRuntimeConfig struct {
	Packages *golist.PackageCollection
}

func (t *testCoreConfig) AddFlags(flags fangs.FlagSet) {
	t.NamedFlagSet = xflagset.NewNamed()
	t.tracker = xflagset.NewDecorator(flags, t.NamedFlagSet.FlagSet("General"))
	flags = t.tracker
	flags.BoolVarP(&t.Test.NoCache, "no-cache", "", "do not use cached test results")
}

func defaultTestOptions() *testCoreConfig {
	return &testCoreConfig{
		Store: options.DefaultStore(),
		Test: testConfig{
			Packages: options.Packages{
				Specifiers: []string{defaultPackageSelection},
			},
			Format: options.DefaultTestFormat(),
		},
	}
}

func Test(app clio.Application) *cobra.Command {
	opts := defaultTestOptions()

	var logTestFailuresAsErrors bool
	cmd := &cobra.Command{
		Use:     "test GO-PKG-SPECIFIER...",
		Short:   "run the tests for the given package(s)",
		Long:    "This is a wrapper around the 'go test' command that provides additional value. See 'go help test' and 'go help build' for detailed flag information.",
		Example: fmt.Sprintf("%s test ./... --no-cache --covermin 80 --exclude 'github.com/me/my/other/pkg/**'", app.ID().Name),
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Test.Specifiers = args
			}

			return nil
		},
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

			// set the UI dynamically
			logTestFailuresAsErrors, err = setUI(app, opts.Test.Format.Output, opts.Test.Appearance, testPkgs)
			return err
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
			return runTest(cmd.Context(), app, *opts, logTestFailuresAsErrors)
		},
	}

	ogHelp := cmd.Help
	cmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		nfs := xflagset.NewNamed()
		// TODO: enumerating these manually is annoying, we should do some reflect properties to get all the named flag sets (or a better interface)
		nfs.Merge(opts.Test.GoTest.NamedFlagSet)
		nfs.Merge(opts.Test.Coverage.NamedFlagSet)
		nfs.Merge(opts.Test.GoBuild.NamedFlagSet)
		nfs.Merge(opts.Test.Packages.NamedFlagSet)
		nfs.Merge(opts.Test.Format.NamedFlagSet)
		// nfs.Merge(opts.Test.Appearance.NamedFlagSet)
		nfs.Merge(opts.Test.Open.NamedFlagSet)
		nfs.Merge(opts.NamedFlagSet)
		nfs.BindUsageAndHelpFunc(cmd, -1)
		_ = ogHelp()
	})

	return app.SetupCommand(cmd, opts)
}

func runTest(ctx context.Context, app clio.Application, coreCfg testCoreConfig, logTestFailuresAsErrors bool) error {
	cfg := coreCfg.Test

	log.WithFields("pkgs", cfg.Specifiers).Info("running test suite")

	var args []string
	args = append(args, cfg.Runtime.Packages.ImportPaths()...)
	args = append(args, cfg.GoBuild.RenderedFlags...)
	args = append(args, cfg.GoTest.RenderedFlags...)
	args = append(args, cfg.ExtraFlags...)

	runConfig := gotest.RunnerConfig{
		Packages: coreCfg.Test.Runtime.Packages,
		Coverage: cfg.Cover,
		NoCache:  cfg.NoCache,
		UserArgs: args,
	}

	s, err := test.NewManager(
		test.Config{
			DBRoot:    coreCfg.Store.Root,
			Ephemeral: coreCfg.Store.Ephemeral,
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
			Runner:                  runConfig,
			Result: gotest.ResultConfig{
				TrackOtherOutput:   false,
				TrackFailingOutput: false,
			},
		},
	)

	if err != nil {
		return fmt.Errorf("unable to run tests: %w", err)
	}

	passed, resultErr := evaluateResult(run, logTestFailuresAsErrors, cfg.CoverMin)

	if cfg.OpenSessionOnFailure && !passed {
		return openUIWithExisting(app, s, resultErr)
	}

	return resultErr
}

func evaluateResult(run *gotest.Run, logTestFailuresAsErrors bool, coverMin float64) (bool, error) {
	result := run.Result
	passed, _ := result.Passed()

	var resultStr = "passed"
	var resultErr error

	if !passed {
		resultStr = "failed"

		if len(result.References()) == 0 {
			resultErr = ErrTestSuiteFailed{Reasons: []string{"no test events observed"}}
		} else {
			resultErr = ErrTestSuiteFailed{Reasons: []string{"not all tests passed"}}
		}
	}

	nested := log.WithFields("result", resultStr, "elapsed", fmt.Sprintf("%2.2fs", run.Elapsed().Seconds()))

	if !passed && logTestFailuresAsErrors {
		nested.Error("completed test suite")
	} else {
		nested.Info("completed test suite")
	}

	if percent, ok := run.Result.Coverage(); ok {
		log.WithFields("percent", percent).Info("test coverage")

		if coverMin > 0 && percent < coverMin {
			// TODO: should we make a lit of errors? not just replace the error?
			resultErr = ErrTestSuiteFailed{Reasons: []string{fmt.Sprintf("coverage below threshold: %2.2f%% < %2.2f%%", percent, coverMin)}}
		}
	}

	return passed, resultErr
}

func setUI(app clio.Application, formatStr string, appearance options.Appearance, testPkgs *golist.PackageCollection) (bool, error) {
	var ux clio.UI

	formatStr = strings.ToLower(formatStr)

	if formatStr != "log" {
		log.Set(discard.New())
	}

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	uiConfig := getUIConfig(appearance, state.Config)

	var logTestFailuresAsErrors bool
	switch formatStr {
	case "go-std", "go", "std":
		ux = ui.NewGoStdUI(testPkgs, false, uiConfig)
	case "go-std-json", "go-json":
		// TODO: we're not passing testPkgs intentionally?
		ux = ui.NewGoStdUI(nil, true, uiConfig)
	case "jest":
		ux = ui.NewJestUI(uiConfig)
	case "dot":
		ux = ui.NewDotUI(uiConfig)
	case "log":
		if state.Config.Log.Verbosity == 0 || !logger.IsVerbose(state.Config.Log.Level) {
			if state.Config.Log.Verbosity == 0 {
				state.Config.Log.Verbosity = 1
				state.Config.Log.Level = logger.InfoLevel
			}

			var err error
			state.Logger, err = clio.DefaultLogger(state.Config, state.RedactStore)
			if err != nil {
				return false, fmt.Errorf("unable to setup logger: %w", err)
			}
			log.Set(state.Logger)
		}

		ux = ui.None()
		logTestFailuresAsErrors = true
	}

	if ux != nil {
		if err := state.UI.Replace(ux); err != nil {
			return false, err
		}
	}

	return logTestFailuresAsErrors, nil
}

func getUIConfig(appearance options.Appearance, clioCfg clio.Config) ui.Config {
	return ui.Config{
		Color:                    !appearance.NoColor,
		Verbose:                  clioCfg.Log.Verbosity,
		ShowPackagesMissingTests: appearance.ShowPackagesWithNoTests,
	}
}

func openUIWithExisting(app clio.Application, s *test.Manager, resultErr error) error {
	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	sessionInfo, err := s.CurrentSession()
	if err != nil {
		return fmt.Errorf("unable to get current test session: %w", err)
	}
	if sessionInfo == nil {
		return fmt.Errorf("no test session found")
	}

	id := app.ID()

	ux := ui.NewStudioUI(studio.Config{
		ID:              fmt.Sprintf("%s@%s", id.Name, id.Version),
		RunController:   s,
		RunStore:        s,
		SessionInfo:     *sessionInfo,
		Debug:           false,
		FailedTestsOnly: true,
	})

	if err := state.UI.Replace(ux); err != nil {
		return err
	}

	ux.Wait()

	// remember -- we opened this to begin with because there were failing tests... so we need to exit 1
	return resultErr
}
