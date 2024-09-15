package options

import (
	"fmt"
	"strings"

	"github.com/scylladb/go-set/strset"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"

	"github.com/anchore/fangs"
)

var _ interface {
	fangs.FlagAdder
	fangs.PostLoader
} = (*Format)(nil)

type Format struct {
	Output           string   `yaml:"output" json:"output" mapstructure:"output"`
	AllowableFormats []string `yaml:"-" json:"-" mapstructure:"-"`
	Aliases          []string `yaml:"-" json:"-" mapstructure:"-"`

	// internal

	tracker      *xflagset.Decorator
	NamedFlagSet *xflagset.Named `yaml:"-" json:"-" mapstructure:"-"`
}

func DefaultTestFormat() Format {
	return Format{
		Output:           "go-std",
		AllowableFormats: []string{"go-std", "go-std-json", "log", "jest", "jest-log", "dot"},
		Aliases:          []string{"go", "std", "go-json"},
	}
}

func (o *Format) PostLoad() error {
	o.Output = strings.ToLower(strings.TrimSpace(o.Output))
	var all []string
	all = append(all, o.AllowableFormats...)
	all = append(all, o.Aliases...)
	if !strset.New(all...).Has(o.Output) {
		return fmt.Errorf("invalid output format: %q", o.Output)
	}
	return nil
}

func (o *Format) AddFlags(flags fangs.FlagSet) {
	o.NamedFlagSet = xflagset.NewNamed()
	o.tracker = xflagset.NewDecorator(flags, o.NamedFlagSet.FlagSet("Format"))
	flags = o.tracker

	flags.StringVarP(
		&o.Output,
		"output", "o",
		fmt.Sprintf("output format to report results in (allowable values: %s)", o.AllowableFormats),
	)
}
