package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gookit/color"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui"
	"github.com/wagoodman/canopy/cmd/canopy/cli/ui/studio"
	"github.com/wagoodman/canopy/cmd/canopy/internal/db"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/source"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
	"github.com/anchore/go-logger"
	"github.com/anchore/go-logger/adapter/discard"
)

var _ fangs.FlagAdder = (*TestCoreConfig)(nil)

var _ SilentError = (*ErrTestSuiteFailed)(nil)

type ErrTestSuiteFailed struct {
	Reasons []string
	noisy   bool
}

func (e ErrTestSuiteFailed) IsSilent() bool {
	return !e.noisy
}

func (e ErrTestSuiteFailed) Error() string {
	if len(e.Reasons) == 0 {
		return ""
	}
	var render string
	for _, reason := range e.Reasons {
		render += fmt.Sprintf("\n  • %s", reason)
	}
	return fmt.Sprintf("test suite failed: %s", render)
}

type TestCoreConfig struct {
	options.Config     `yaml:",inline" mapstructure:",squash"`
	options.Experiment `yaml:"exp" json:"exp" mapstructure:"exp"`
	options.Store      `yaml:"store" json:"store" mapstructure:"store"`

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
	options.Appearance `yaml:"appearance" json:"appearance" mapstructure:"appearance"`
	ExtraFlags         []string `yaml:"extra-flags" json:"extra-flags" mapstructure:"extra-flags"`

	// post parse
	Runtime testRuntimeConfig `yaml:"-" json:"-" mapstructure:"-"`
}

type testRuntimeConfig struct {
	Packages *golist.PackageCollection
}

func (t *TestCoreConfig) AddFlags(flags fangs.FlagSet) {
	t.NamedFlagSet = xflagset.NewNamed()
	t.tracker = xflagset.NewDecorator(flags, t.NamedFlagSet.FlagSet("State"))
	flags = t.tracker
	flags.BoolVarP(&t.Test.NoCache, "no-cache", "", "do not use cached test results")
}

func withoutCoverageOpts() func(*TestCoreConfig) {
	return func(cfg *TestCoreConfig) {
		cfg.Test.Coverage.Disabled = true
	}
}

func withoutOpenOpts() func(*TestCoreConfig) {
	return func(cfg *TestCoreConfig) {
		cfg.Test.Open.Disabled = true
	}
}

func withoutRunOptsRendered() func(*TestCoreConfig) {
	return func(cfg *TestCoreConfig) {
		cfg.Test.IgnoreRenderingFlags = append(cfg.Test.IgnoreRenderingFlags, "run")
	}
}

func withCombineMultipleRuns() func(*TestCoreConfig) {
	return func(cfg *TestCoreConfig) {
		cfg.Test.CombineMultipleRuns = true
	}
}

func defaultTestOptions(opts ...func(*TestCoreConfig)) *TestCoreConfig {
	t := &TestCoreConfig{
		Experiment: options.DefaultExperiment(),
		Store:      options.DefaultStore(),
		Test: testConfig{
			Packages:   options.DefaultPackages(),
			GoTest:     options.DefaultGoTest(),
			Coverage:   options.DefaultCoverage(),
			GoBuild:    options.DefaultGoBuild(),
			Format:     options.DefaultTestFormat(),
			Open:       options.DefaultOpen(),
			Appearance: options.DefaultAppearance(),
		},
	}

	for _, fn := range opts {
		fn(t)
	}

	return t
}

func Test(app clio.Application) *cobra.Command {
	opts := defaultTestOptions()

	var logTestFailuresAsErrors bool
	var runErr error
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
				return fmt.Errorf("unable to get test paths: %w", err)
			}
			if testPkgs.Size() == 0 {
				return fmt.Errorf("no packages selected to test (given %q)", opts.Test.Specifiers)
			}
			opts.Test.Runtime.Packages = testPkgs

			// set the UI dynamically
			var module string
			if opts.Test.UseShortNames {
				module = testPkgs.Module()
				log.WithFields("module", module).Debug("using module name for package prefix stripping")
			}

			maxPkgName := maxPkgNameLength(testPkgs.ImportPaths(), module)
			logTestFailuresAsErrors, err = setupTestUIs(app, opts.Test.Writers, opts.Test.Appearance, maxPkgName, module)
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			defer func() {
				if err := opts.Test.Writers.Close(); err != nil {
					runErr = multierror.Append(runErr, err)
					log.WithFields("error", err).Error("unable to close format writers")
				}
			}()

			runErr = runTest(cmd.Context(), app, *opts, logTestFailuresAsErrors)

			return nil
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

	return app.SetupCommand(cmd, opts)
}

func runTest(ctx context.Context, app clio.Application, coreCfg TestCoreConfig, logTestFailuresAsErrors bool) error {
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

	// capture source state only when store is enabled (persistent)
	var sourceState *db.SourceStateInput
	if coreCfg.Enabled {
		if ss := source.CaptureState("."); ss != nil {
			sourceState = toSourceStateInput(ss)
			log.WithFields("commit", ss.Commit, "branch", ss.Branch, "dirty", ss.Dirty).Debug("captured source state")
		}
	}

	run, err := s.RunTests(
		ctx,
		test.RunConfig{
			LogTestFailuresAsErrors: logTestFailuresAsErrors,
			Runner:                  runConfig,
			Result: gotest.ResultConfig{
				TrackOtherOutput:   false,
				TrackFailingOutput: false,
			},
			SourceState: sourceState,
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
			// if tests simply failed, let the UI show this as a failure, no need to show an additional message
			resultErr = ErrTestSuiteFailed{}
		}
	}

	nested := log.WithFields("result", resultStr, "elapsed", fmt.Sprintf("%2.2fs", run.Result.Elapsed(false).Seconds()))

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

func setupTestUIs(app clio.Application, writers []options.FormatWriter, appearance options.Appearance, maxPkgName int, module string) (bool, error) {
	var logTestFailuresAsErrors bool

	var uxs []clio.UI
	for _, writer := range writers {
		ux, ltaf, err := setupTestUI(app, writer, appearance, maxPkgName, module)
		if err != nil {
			return false, fmt.Errorf("unable to setup UI %q: %w", writer.Name, err)
		}
		uxs = append(uxs, ux)
		logTestFailuresAsErrors = logTestFailuresAsErrors || ltaf
	}

	if len(uxs) > 0 {
		type Stater interface {
			State() *clio.State
		}

		state := app.(Stater).State()

		if err := state.UI.Replace(ui.NewCollection(uxs...)); err != nil {
			return logTestFailuresAsErrors, err
		}
	}

	return logTestFailuresAsErrors, nil
}

func setupTestUI(app clio.Application, format options.FormatWriter, appearance options.Appearance, maxPkgName int, module string) (clio.UI, bool, error) {
	var ux clio.UI

	fields := logger.Fields{
		"format": format.Name,
	}
	if format.Path != "" {
		fields["path"] = format.Path
	}
	log.WithFields(fields).Debug("setting up UI")

	type Stater interface {
		State() *clio.State
	}

	state := app.(Stater).State()

	uiConfig := getUIConfig(appearance, state.Config, format, module)

	var logTestFailuresAsErrors bool
	switch format.Name {
	// case "go++":
	//	ux = ui.NewGoxxUI(uiConfig, maxPkgName)
	case "go":
		ux = ui.NewTestGoUI(uiConfig, maxPkgName)
	case formatJSON:
		// TODO: we're not passing testPkgs intentionally?
		ux = ui.NewTestJSONUI(uiConfig)
	case "jest":
		ux = ui.NewTestJestUI(uiConfig)
	case "dot":
		ux = ui.NewTestDotUI(uiConfig)
	case "log":
		if state.Config.Log.Verbosity == 0 || !logger.IsVerbose(state.Config.Log.Level) {
			if state.Config.Log.Verbosity == 0 {
				state.Config.Log.Verbosity = 1
				state.Config.Log.Level = logger.InfoLevel
			}

			var err error
			state.Logger, err = clio.DefaultLogger(state.Config, state.RedactStore)
			if err != nil {
				return nil, false, fmt.Errorf("unable to setup logger: %w", err)
			}
			log.Set(state.Logger)
		}

		ux = ui.TestNoUI()
		if format.PrimaryUI {
			logTestFailuresAsErrors = true
		}
	}

	// if the format is not log, then we should discard the logger since it may be noisy
	// and write over the default UI in the terminal
	if format.PrimaryUI && format.Name != "log" {
		state.Logger = discard.New()
		log.Set(state.Logger)
	}

	return ux, logTestFailuresAsErrors, nil
}

func getUIConfig(appearance options.Appearance, clioCfg clio.Config, format options.FormatWriter, module string) ui.TestUIConfig {
	var removePrefix string
	if appearance.UseShortNames {
		removePrefix = module
	}
	return ui.TestUIConfig{
		Color:                   appearance.Color != "off",
		Verbose:                 clioCfg.Log.Verbosity,
		ShowPackagesWithNoTests: appearance.ShowPackagesWithNoTests,
		StripPackagePrefix:      removePrefix,
		Writer:                  format.Writer,
		IsTTY:                   format.IsTTY,
		CombineMultipleRuns:     appearance.CombineMultipleRuns,
		ExecutionMarkers:        appearance.ExecutionMarkers,
		Grouping:                appearance.Grouping.ToAPIConfig(),
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

func showTestFailure(err error) {
	var resErr ErrTestSuiteFailed
	if errors.As(err, &resErr) {
		msg := renderTestSuiteFailure(resErr)
		if msg != "" {
			color.Red.Println(msg)
		}
	} else {
		msg := color.Red.Render(strings.TrimSpace(err.Error()))
		fmt.Fprintln(os.Stderr, msg)
	}
}

func renderTestSuiteFailure(err ErrTestSuiteFailed) string {
	if len(err.Reasons) == 0 {
		return ""
	}
	var render string
	for _, reason := range err.Reasons {
		render += fmt.Sprintf("\n  • %s", reason)
	}
	return fmt.Sprintf("Test suite failed: %s", render)
}

func toSourceStateInput(s *source.State) *db.SourceStateInput {
	var files []db.DirtyFileInput
	for _, f := range s.DirtyFiles {
		files = append(files, db.DirtyFileInput{
			Path:        f.Path,
			ContentHash: f.ContentHash,
			ModTime:     f.ModTime,
		})
	}
	return &db.SourceStateInput{
		Commit:     s.Commit,
		Branch:     s.Branch,
		Dirty:      s.Dirty,
		DirtyFiles: files,
	}
}

func maxPkgNameLength(testPkgs []string, removePrefix string) int {
	maxPkgName := 30
	for _, pkg := range testPkgs {
		if removePrefix != "" && strings.HasPrefix(pkg, removePrefix) {
			pkg = strings.TrimPrefix(pkg, removePrefix)
			pkg = strings.TrimPrefix(pkg, "/")
		}
		if len(pkg) > maxPkgName {
			maxPkgName = len(pkg)
		}
	}
	return maxPkgName
}
