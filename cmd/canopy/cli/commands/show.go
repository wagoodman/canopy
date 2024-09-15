package commands

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"
	"github.com/wagoodman/canopy/cmd/canopy/internal/test"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var (
	_ fangs.FlagAdder  = (*showCoreConfig)(nil)
	_ fangs.PostLoader = (*showCoreConfig)(nil)
)

// canopy show FILE                          Read go test json output from a file and reformat as go-std to stdout
// go test -json | canopy show -o jest       Get test output from stdin and reformat as jest to stdout
// canopy show --store FILE                  Import into DB, otherwise reformat as go-std to stdout
//
// config:
// - test.format
// - test.appearance
// - test.open-on-failure
// ^ ... by doing all of these you can use --profile to select test formats and behavior without test run and build flags
// ... alternative name: view
// show vs view vs open... we're getting a little too close to each other

type showCoreConfig struct {
	options.Config `yaml:",inline" mapstructure:",squash"`
	options.Store  `yaml:"store" json:"store" mapstructure:"store"`

	Test showTestConfig `yaml:"test" json:"test" mapstructure:"test"`
	Show showConfig     `yaml:"show" json:"show" mapstructure:"show"`
}

type showConfig struct {
	File string `yaml:"file" json:"file" mapstructure:"file"`

	reader io.ReadCloser
}

func (o *showConfig) PostLoad() error {
	switch o.File {
	case "-":
		log.Debug("reading test json from stdin")
		o.reader = io.NopCloser(os.Stdin) // we don't want to prevent input from the user for TUI accidentally
	case "":
		break
	default:
		log.WithFields("file", o.File).Debug("reading test json from file")
		f, err := os.Open(o.File)
		if err != nil {
			return fmt.Errorf("unable to open test json file=%q: %w", o.File, err)
		}
		o.reader = f
	}

	return nil
}

type showTestConfig struct {
	options.Format     `yaml:",inline" json:"" mapstructure:",squash"`
	options.Appearance `yaml:",inline" json:"" mapstructure:",squash"`
	options.Open       `yaml:",inline" json:"" mapstructure:",squash"`
	options.Coverage   `yaml:",inline" json:"" mapstructure:",squash"`
}

func defaultShowOptions() *showCoreConfig {
	return &showCoreConfig{
		Store: options.DefaultStore(),
		Test: showTestConfig{
			Format: options.DefaultTestFormat(),
		},
	}
}

func Show(app clio.Application) *cobra.Command {
	opts := defaultShowOptions()

	var logTestFailuresAsErrors bool
	cmd := &cobra.Command{
		Use:   "show FILE",
		Short: "Show formatted test output from a given `go test -json` input",
		Args: func(_ *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				if piped, err := isPipedInput(); !piped || err != nil {
					return fmt.Errorf("requires a single argument for a go test json file (or piped from stdin)")
				}
				args = append(args, "-")
			case 1:
				break
			default:
				return fmt.Errorf("too many arguments; accepts only a single argument for a go test json file")
			}

			opts.Show.File = args[0]
			return nil
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if opts.Show.File == "-" {
				log.Debug("reading test json from stdin")
			}
			var err error
			logTestFailuresAsErrors, err = setUI(app, opts.Test.Format.Output, opts.Test.Appearance, nil)
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFormat(cmd.Context(), app, *opts, logTestFailuresAsErrors)
		},
	}

	return app.SetupCommand(cmd, opts)
}

// isPipedInput returns true if there is no input device, which means the user **may** be providing input via a pipe.
func isPipedInput() (bool, error) {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false, fmt.Errorf("unable to determine if there is piped input: %w", err)
	}

	// note: we should NOT use the absence of a character device here as the hint that there may be input expected
	// on stdin, as running syft as a subprocess you would expect no character device to be present but input can
	// be from either stdin or indicated by the CLI. Checking if stdin is a pipe is the most direct way to determine
	// if there *may* be bytes that will show up on stdin that should be used for the analysis source.
	return fi.Mode()&os.ModeNamedPipe != 0, nil
}

func runFormat(ctx context.Context, app clio.Application, coreCfg showCoreConfig, logTestFailuresAsErrors bool) error {
	cfg := coreCfg.Test

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
			Runner:                  gotest.RunnerConfig{},
			Result: gotest.ResultConfig{
				TrackOtherOutput:   false,
				TrackFailingOutput: false,
			},
			Reader: coreCfg.Show.reader,
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
