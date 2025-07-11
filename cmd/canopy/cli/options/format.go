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
	"golang.org/x/term"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*Format)(nil)

type Format struct {
	Output           string        `yaml:"-" json:"-" mapstructure:"-"`
	Outputs          []string      `yaml:"output" json:"output" mapstructure:"output"`
	AllowableFormats []string      `yaml:"-" json:"-" mapstructure:"-"`
	Aliases          []string      `yaml:"-" json:"-" mapstructure:"-"`
	Writers          FormatWriters `yaml:"-" json:"-" mapstructure:"-"`
	FileDisallowed   []string      `yaml:"-" json:"-" mapstructure:"-"`
	AllowMultiple    bool          `yaml:"-" json:"-" mapstructure:"-"`

	// internal

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultTestFormat() Format {
	return Format{
		Outputs:          []string{"go++"},
		AllowMultiple:    true,
		AllowableFormats: []string{"go", "go++", "json", "log", "jest", "dot"},
		FileDisallowed:   []string{"jest", "dot", "log"}, // TODO: log should not be on this list
	}
}

func (o *Format) PostLoad() error {
	if len(o.Output) > 0 {
		o.Outputs = append(o.Outputs, o.Output)
		o.Output = ""
	}

	if len(o.Outputs) > 0 && !o.AllowMultiple {
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

type FormatWriters []FormatWriter

type FormatWriter struct {
	Path      string
	Writer    io.WriteCloser
	IsTTY     bool
	PrimaryUI bool
	Name      string
}

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

func (f FormatWriters) Close() error {
	var errs error
	for _, w := range f {
		if err := w.Close(); err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	return errs
}

func isATTY(fd int) bool {
	if val := os.Getenv("NO_TTY"); val != "" {
		return !isPositive(val)
	}
	return term.IsTerminal(fd)
}

func isPositive(val string) bool {
	switch strings.ToLower(strings.TrimSpace(val)) {
	case "true", "yes", "y", "1", "t":
		return true
	}
	return false
}
