package options

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/env"
	"golang.org/x/term"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*Format)(nil)

// Format configures output formatting options for test results, supporting multiple concurrent output formats.
type Format struct {
	// Output is a single output format (used when AllowMultiple is false).
	Output string `yaml:"-" json:"-" mapstructure:"-"`
	// Outputs is a list of output formats to use (e.g., "go", "json", "jest=results.json").
	Outputs []string `yaml:"output" json:"output" mapstructure:"output"`
	// AllowableFormats lists the valid format names that can be specified.
	AllowableFormats []string `yaml:"-" json:"-" mapstructure:"-"`
	// Aliases contains alternative names for formats (e.g., "fn" for "function").
	Aliases []string `yaml:"-" json:"-" mapstructure:"-"`
	// Writers holds the configured output writers after post-load processing.
	Writers FormatWriters `yaml:"-" json:"-" mapstructure:"-"`
	// FileDisallowed lists formats that cannot be written to files (only stdout).
	FileDisallowed []string `yaml:"-" json:"-" mapstructure:"-"`
	// AllowMultiple controls whether multiple output formats can be specified simultaneously.
	AllowMultiple bool `yaml:"-" json:"-" mapstructure:"-"`

	// internal

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

// DefaultTestFormat returns format options configured for test output with the "go" format as default.
// Jest and dot formats are experimental and enabled via environment variables.
func DefaultTestFormat() Format {
	allowable := []string{"go", "json", "log"}
	fileDisallowed := []string{"log"}

	// jest and dot are experimental, enabled via environment variables
	if isEnvEnabled("CANOPY_EXP_JEST_UI") {
		allowable = append(allowable, "jest")
		fileDisallowed = append(fileDisallowed, "jest")
	}
	if isEnvEnabled("CANOPY_EXP_DOT_UI") {
		allowable = append(allowable, "dot")
		fileDisallowed = append(fileDisallowed, "dot")
	}

	return Format{
		Outputs:          []string{"go"},
		AllowMultiple:    true,
		AllowableFormats: allowable,
		FileDisallowed:   fileDisallowed,
	}
}

// PostLoad validates format specifications and creates output writers (files or stdout) for each format.
func (o *Format) PostLoad() error {
	if len(o.Output) > 0 {
		o.Outputs = append(o.Outputs, o.Output)
		o.Output = ""
	}

	if len(o.Outputs) > 1 && !o.AllowMultiple {
		return fmt.Errorf("only one output format may be specified (multiple were specified: %v)", strings.Join(o.Outputs, ", "))
	}

	if len(o.Outputs) == 0 {
		return fmt.Errorf("no output format specified")
	}

	var all []string

	all = append(all, o.AllowableFormats...)
	all = append(all, o.Aliases...)
	var bad []string
	for _, output := range o.Outputs {
		fields := strings.Split(output, "=")
		if !strset.New(all...).Has(fields[0]) {
			bad = append(bad, fields[0])
		}
	}

	if len(bad) > 0 {
		return fmt.Errorf("invalid output format(s) specified: %v", strings.Join(bad, ", "))
	}

	filesDisallowedSet := strset.New(o.FileDisallowed...)

	var nonFileOutputs []string
	for _, output := range o.Outputs {
		fields := strings.Split(output, "=")
		switch len(fields) {
		case 1:
			// write to stdout
			o.Writers = append(o.Writers, FormatWriter{
				IsTTY:     isATTY(int(os.Stdout.Fd())),
				Name:      strings.ToLower(output),
				PrimaryUI: true,
			})
			nonFileOutputs = append(nonFileOutputs, output)
		case 2:
			if filesDisallowedSet.Has(output) {
				return fmt.Errorf("output format %q cannot be written to a file", output)
			}
			// write to file
			f, err := os.Create(fields[1])
			if err != nil {
				return fmt.Errorf("unable to create output file: %w", err)
			}
			o.Writers = append(o.Writers, FormatWriter{
				Path:   fields[1],
				Writer: f,
				IsTTY:  false,
				Name:   strings.ToLower(fields[0]),
			})
		default:
			return fmt.Errorf("invalid output format specified: %s", output)
		}
	}

	if len(nonFileOutputs) > 1 {
		return fmt.Errorf("only one output format may be written to stdout (multiple were specified: %v)", strings.Join(nonFileOutputs, ", "))
	}

	return nil
}

// AddFlags registers the output format flag with the flag set.
func (o *Format) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Format"))
	flags = o.tracker

	if o.AllowMultiple {
		flags.StringArrayVarP(
			&o.Outputs,
			"output", "o",
			fmt.Sprintf("output format to report results in (allowable values: %s)", o.AllowableFormats),
		)
	} else {
		flags.StringVarP(
			&o.Output,
			"output", "o",
			fmt.Sprintf("output format to report results in (allowable values: %s)", o.AllowableFormats),
		)
	}
}

// FormatWriters is a collection of FormatWriter instances representing all configured output destinations.
type FormatWriters []FormatWriter

// FormatWriter represents a single output destination for formatted test results.
type FormatWriter struct {
	// Path is the file path if writing to a file (empty for stdout).
	Path string
	// Writer is the underlying writer for output (file or stdout).
	Writer io.WriteCloser
	// IsTTY indicates whether the output destination is a terminal.
	IsTTY bool
	// PrimaryUI indicates if this is the main UI output (affects logging behavior).
	PrimaryUI bool
	// Name is the format name (e.g., "go", "json", "jest").
	Name string
}

// Close closes the underlying writer, ignoring "already closed" errors.
func (f FormatWriter) Close() error {
	if f.Writer != nil {
		// return error unless it's an "already closed" error
		err := f.Writer.Close()
		if err != nil {
			if !errors.Is(err, fs.ErrClosed) {
				return err
			}
		}
	}
	return nil
}

// Close closes all writers in the collection, accumulating any errors.
func (f FormatWriters) Close() error {
	var errs error
	for _, w := range f {
		if err := w.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

// isATTY checks if the given file descriptor is a terminal, respecting the NO_TTY environment variable.
func isATTY(fd int) bool {
	if val := os.Getenv("NO_TTY"); val != "" {
		return !env.Truthy(val)
	}
	return term.IsTerminal(fd)
}

// isEnvEnabled checks if the given environment variable is set to a truthy value.
func isEnvEnabled(key string) bool {
	return env.Truthy(os.Getenv(key))
}
