package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/scylladb/go-set/strset"
	"github.com/spf13/cobra"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options"
	"github.com/wagoodman/canopy/cmd/canopy/cli/options/xflagset"
	"github.com/wagoodman/canopy/cmd/canopy/internal/bus"
	"github.com/wagoodman/canopy/cmd/canopy/internal/golist"
	"github.com/wagoodman/canopy/cmd/canopy/internal/gotest"
	"github.com/wagoodman/canopy/cmd/canopy/internal/log"

	"github.com/anchore/clio"
	"github.com/anchore/fangs"
)

var _ clio.FlagAdder = (*listConfig)(nil)

type listConfig struct {
	options.Packages `yaml:",inline" json:"" mapstructure:",squash"`
	options.Format   `yaml:",inline" json:"" mapstructure:",squash"`
	// Cases controls whether to show test cases within test functions (e.g. t.Run() calls).
	// This may be inaccurate as it relies on static analysis.
	Cases bool `yaml:"cases" json:"cases" mapstructure:"cases"`
}

func (o *listConfig) AddFlags(flags fangs.FlagSet) {
	flags.BoolVarP(
		&o.Cases,
		"cases", "",
		"show test cases within test functions (e.g. t.Run() calls). This may be inaccurate.",
	)
}

// ListDefs creates a command to discover and list all test functions in the specified packages.
// It supports multiple output formats (function names, package names, or JSON) and can optionally
// show test cases (subtests created with t.Run()).
func ListDefs(app clio.Application) *cobra.Command {
	type listTestEnvelope struct {
		options.Config `yaml:",inline" mapstructure:",squash"`
		List           listConfig `yaml:"list" json:"list" mapstructure:"list"`
	}

	opts := &listTestEnvelope{
		List: listConfig{
			Packages: options.DefaultPackages(),
			Format: options.Format{
				Outputs:          []string{"function"},
				AllowableFormats: []string{"function", "json", "package"},
				Aliases:          []string{"fn", "fns", "f", "p", "pkg", "pkgs", "functions", "packages"},
				AllowMultiple:    false,
			},
		},
	}

	cmd := &cobra.Command{
		Use:   "defs GO-PKG-SPECIFIER...",
		Short: "list all test definitions found in the package path (use ... for recursive search)",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.List.Specifiers = args
			}
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			defer bus.Exit()

			return runList(opts.List)
		},
	}

	// facilitates grouping of flags into sections in help text
	xflagset.BindCobraHelpFromOpts(cmd, opts)

	return app.SetupCommand(cmd, opts)
}

func runList(cfg listConfig) error {
	testPkgs, err := golist.SelectPackages(cfg.Specifiers, cfg.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("unable to get test paths: %w", err)
	}
	if testPkgs.Size() == 0 {
		return fmt.Errorf("no packages selected (given %q)", cfg.Specifiers)
	}

	log.WithFields("pkgs", cfg.Specifiers).Info("listing tests in test suite")

	tests, err := gotest.FindDefinitions(testPkgs)
	if err != nil {
		return err
	}

	var report string
	switch strings.ToLower(cfg.Outputs[0]) {
	case "package", "packages", "pkg", "pkgs", "p":
		report = listTestPkgs(tests)
	case "function", "functions", "fn", "fns", "f":
		report = listTestFunctions(tests, cfg.Cases)
	case "json":
		report, err = listTestJSON(tests)
	default:
		err = fmt.Errorf("unknown format: %s", cfg.Outputs)
	}

	if err != nil {
		return err
	}

	bus.Report(report)

	return nil
}

// listTestPkgs returns a newline-separated list of unique package import paths.
func listTestPkgs(tests []gotest.Definition) string {
	pkgs := strset.New()
	for _, t := range tests {
		pkgs.Add(t.ImportPath)
	}

	pkgsSlice := pkgs.List()
	sort.Strings(pkgsSlice)

	sb := strings.Builder{}

	for _, p := range pkgsSlice {
		sb.WriteString(fmt.Sprintf("%s\n", p))
	}

	return sb.String()
}

// listTestFunctions returns a newline-separated list of test function names in "package.TestFunc" format.
// If showCases is true, also includes subtests in "package.TestFunc/SubTest" format.
func listTestFunctions(tests []gotest.Definition, showCases bool) string {
	sb := strings.Builder{}
	for _, t := range tests {
		sb.WriteString(fmt.Sprintf("%s.%s\n", t.ImportPath, t.FnName))
		if !showCases {
			continue
		}

		for _, c := range t.Cases {
			sb.WriteString(fmt.Sprintf("%s.%s/%s\n", t.ImportPath, t.FnName, c))
		}
	}
	return sb.String()
}

// listTestJSON returns a JSON representation of all test definitions.
func listTestJSON(tests []gotest.Definition) (string, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)

	err := enc.Encode(tests)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
