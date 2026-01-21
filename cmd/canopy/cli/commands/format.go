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

var _ fangs.FlagAdder = (*formatCoreConfig)(nil)

// canopy format test.json                     Read go test json output from a file and reformat as go-std to stdout
// go test -json | canopy format -o jest       Get test output from stdin and reformat as jest to stdout
// canopy format --store FILE                  Import into DB, otherwise reformat as go-std to stdout

// formatCoreConfig holds all configuration options for the format command, including where to read test output
// and how to display it.
type formatCoreConfig struct {
	options.Config     `yaml:",inline" mapstructure:",squash"`
	options.Experiment `yaml:"exp" json:"exp" mapstructure:"exp"`
	options.Store      `yaml:"store" json:"store" mapstructure:"store"`

	Test   formatTestConfig `yaml:"test" json:"test" mapstructure:"test"`
	Format formatConfig     `yaml:"format" json:"format" mapstructure:"format"`
}

// formatConfig specifies the source file for test output (file path or "-" for stdin).
type formatConfig struct {
	// File specifies the path to read test JSON output from, or "-" for stdin (this is the format command argument).
	File string `yaml:"file" json:"file" mapstructure:"file"`

	reader io.ReadCloser
}

func (o *formatConfig) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&o.File, "the file path to read go test -json output from; use '-' to read from stdin (this is the format command argument)")
}

// PostLoad opens the file or stdin for reading test JSON output based on the File field.
func (o *formatConfig) PostLoad() error {
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

// TODO: make this a shared struct between test and show commands... this is brittle when using with the config command

// formatTestConfig contains formatting and display options for reformatting test output.
type formatTestConfig struct {
	options.Format     `yaml:",inline" json:"" mapstructure:",squash"`
	options.Open       `yaml:",inline" json:"" mapstructure:",squash"`
	options.Coverage   `yaml:",inline" json:"" mapstructure:",squash"`
	options.Appearance `yaml:"appearance" json:"appearance" mapstructure:"appearance"`
}

func defaultFormatOptions() *formatCoreConfig {
	return &formatCoreConfig{
		Experiment: options.DefaultExperiment(),
		Store:      options.DefaultStore(),
		Test: formatTestConfig{
			Format:     options.DefaultTestFormat(),
			Appearance: options.DefaultAppearance(),
		},
	}
}

// Format creates a command to reformat previously generated `go test -json` output using different output formats.
// It accepts input from a file or stdin and can optionally store results in a database session.
func Format(app clio.Application) *cobra.Command {
	opts := defaultFormatOptions()

	var logTestFailuresAsErrors bool
	cmd := &cobra.Command{
		Use:   "format [FILE]",
		Short: "Format formatted test output from a given `go test -json` input (from a file or stdin)",
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

			opts.Format.File = args[0]
			return nil
		},
		PreRunE: func(_ *cobra.Command, _ []string) error {
			if opts.Format.File == "-" {
				log.Debug("reading test json from stdin")
			}
			var err error
			logTestFailuresAsErrors, err = setupTestUIs(app, opts.Test.Writers, opts.Test.Appearance, 30, "") // TODO: we do not support module prefix stripping here yet for format-only operations
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

func runFormat(ctx context.Context, app clio.Application, coreCfg formatCoreConfig, logTestFailuresAsErrors bool) error {
	cfg := coreCfg.Test

	// only use a DB store when persistence is needed
	needsStorage := coreCfg.Enabled || cfg.OpenSessionOnFailure

	s, err := test.NewManager(
		test.Config{
			DBRoot:    coreCfg.Root,
			Ephemeral: coreCfg.Ephemeral,
			NoStore:   !needsStorage,
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
			Reader: coreCfg.Format.reader,
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
